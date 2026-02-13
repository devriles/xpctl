package kube

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/devriles/xpctl/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceNode struct {
	Kind       string
	Name       string
	Namespace  string
	APIVersion string
	Status     string
	Children   []*ResourceNode
	Resource   *model.Resource
}

type CompositionResult struct {
	Root  *ResourceNode
	Flat  []*ResourceNode
	Stats model.TreeStats
}

// Discover traverses a Crossplane resource tree: top-down from an XR or Claim,
// or bottom-up from a managed resource to its parent XR.
func Discover(ctx context.Context, c *Client, kind, name, namespace string) (*CompositionResult, error) {
	resolved, err := resolveGVR(c, kind)
	if err != nil {
		return nil, fmt.Errorf("resolving resource %q: %w", kind, err)
	}
	gvr := resolved.GroupVersionResource

	var obj *unstructured.Unstructured
	if resolved.Namespaced && namespace != "" {
		obj, err = c.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = c.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("fetching %s/%s: %w", kind, name, err)
	}

	root := buildResourceNode(obj)
	result := &CompositionResult{Root: root}

	// If this is a Claim, follow .spec.resourceRef to the XR
	if ref, ok := getResourceRef(obj); ok {
		xrResolved, err := resolveGVR(c, ref.Kind)
		if err == nil {
			xrObj, err := c.Dynamic.Resource(xrResolved.GroupVersionResource).Get(ctx, ref.Name, metav1.GetOptions{})
			if err == nil {
				xrNode := buildResourceNode(xrObj)
				root.Children = append(root.Children, xrNode)
				// Traverse composed resources from the XR
				if err := discoverComposed(ctx, c, xrObj, xrNode); err != nil {
					return nil, err
				}
			}
		}
	} else if _, found, _ := unstructured.NestedSlice(obj.Object, "spec", "resourceRefs"); found {
		// This is an XR — traverse its composed resources
		if err := discoverComposed(ctx, c, obj, root); err != nil {
			return nil, err
		}
	} else {
		// Bottom-up: this might be a managed resource — try to walk up to the parent XR
		if parent, err := discoverUpward(ctx, c, obj); err == nil && parent != nil {
			// Re-root: parent becomes root, the original resource is in the tree
			result.Root = parent
			root = parent
		}
	}

	result.Flat = flatten(root)
	result.Stats = computeStats(result.Flat)

	return result, nil
}

// discoverUpward walks from a managed resource to its parent XR via label or owner references.
func discoverUpward(ctx context.Context, c *Client, obj *unstructured.Unstructured) (*ResourceNode, error) {
	// Strategy 1: Check crossplane.io/composite label
	labels := obj.GetLabels()
	compositeName := labels["crossplane.io/composite"]
	if compositeName == "" {
		// Strategy 2: Check owner references
		owners := obj.GetOwnerReferences()
		for _, ref := range owners {
			compositeName = ref.Name
			// Try to discover from the owner
			parentResolved, err := resolveGVR(c, ref.Kind)
			if err != nil {
				continue
			}
			parentObj, err := c.Dynamic.Resource(parentResolved.GroupVersionResource).Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			parentNode := buildResourceNode(parentObj)
			if err := discoverComposed(ctx, c, parentObj, parentNode); err == nil {
				return parentNode, nil
			}
		}
		if compositeName == "" {
			return nil, fmt.Errorf("no parent composite found")
		}
	}

	// We have a composite name from the label — find it
	// The composite kind isn't in the label, so we search XR-like resources
	return findCompositeByName(ctx, c, compositeName)
}

// findCompositeByName searches all composite-category API resources for one matching name.
func findCompositeByName(ctx context.Context, c *Client, name string) (*ResourceNode, error) {
	_, resources, err := c.Discovery.ServerGroupsAndResources()
	if err != nil && len(resources) == 0 {
		return nil, fmt.Errorf("discovering API resources: %w", err)
	}

	for _, list := range resources {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range list.APIResources {
			// Composite resources typically have the "composites" category
			if !hasCategory(r.Categories, "composite") {
				continue
			}
			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}
			obj, err := c.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			node := buildResourceNode(obj)
			_ = discoverComposed(ctx, c, obj, node)
			return node, nil
		}
	}

	return nil, fmt.Errorf("composite %q not found", name)
}

func hasCategory(categories []string, target string) bool {
	for _, c := range categories {
		if c == target {
			return true
		}
	}
	return false
}

type CompositeItem struct {
	Kind       string
	Name       string
	Namespace  string
	APIVersion string
	Status     string
}

// ListComposites returns all Crossplane XR and Claim instances in the cluster.
func ListComposites(ctx context.Context, c *Client) ([]CompositeItem, error) {
	_, resources, err := c.Discovery.ServerGroupsAndResources()
	if err != nil && len(resources) == 0 {
		return nil, fmt.Errorf("discovering API resources: %w", err)
	}

	var (
		items []CompositeItem
		mu    sync.Mutex
		wg    sync.WaitGroup
		sem   = make(chan struct{}, 10)
	)

	for _, list := range resources {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range list.APIResources {
			if !hasCategory(r.Categories, "composite") && !hasCategory(r.Categories, "claim") {
				continue
			}
			if strings.Contains(r.Name, "/") { // skip subresources
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}
			namespaced := r.Namespaced

			wg.Add(1)
			go func(gvr schema.GroupVersionResource, kind string, namespaced bool) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				var objs *unstructured.UnstructuredList
				var err error
				objs, err = c.Dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
				if err != nil {
					return
				}

				for _, obj := range objs.Items {
					conditions := ParseConditions(&obj)
					status := DeriveStatus(conditions)
					item := CompositeItem{
						Kind:       obj.GetKind(),
						Name:       obj.GetName(),
						Namespace:  obj.GetNamespace(),
						APIVersion: obj.GetAPIVersion(),
						Status:     string(status),
					}
					mu.Lock()
					items = append(items, item)
					mu.Unlock()
				}
			}(gvr, r.Kind, namespaced)
		}
	}

	wg.Wait()
	return items, nil
}

func discoverComposed(ctx context.Context, c *Client, xr *unstructured.Unstructured, parent *ResourceNode) error {
	refs, found, err := unstructured.NestedSlice(xr.Object, "spec", "resourceRefs")
	if err != nil || !found {
		return nil
	}

	type fetchResult struct {
		node *ResourceNode
		obj  *unstructured.Unstructured
		idx  int
	}

	var (
		mu      sync.Mutex
		results = make([]fetchResult, len(refs))
		wg      sync.WaitGroup
		sem     = make(chan struct{}, 10) // limit concurrency
	)

	for i, ref := range refs {
		refMap, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		refKind := stringField(refMap, "kind")
		refName := stringField(refMap, "name")
		refNS := stringField(refMap, "namespace")
		refAPIVersion := stringField(refMap, "apiVersion")
		if refKind == "" || refName == "" {
			continue
		}

		wg.Add(1)
		go func(idx int, kind, name, ns, apiVersion string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			gvr, err := resolveGVRFromAPIVersion(c, apiVersion, kind)
			if err != nil {
				// Fallback: try resolving by kind
				fallback, err2 := resolveGVR(c, kind)
				if err2 != nil {
					err = err2
				} else {
					gvr = fallback.GroupVersionResource
					err = nil
				}
				if err != nil {
					node := &ResourceNode{
						Kind:   kind,
						Name:   name,
						Status: string(model.StatusUnknown),
						Resource: &model.Resource{
							Kind:   kind,
							Name:   name,
							Status: model.StatusUnknown,
						},
					}
					mu.Lock()
					results[idx] = fetchResult{node: node, idx: idx}
					mu.Unlock()
					return
				}
			}

			var obj *unstructured.Unstructured
			if ns != "" {
				obj, err = c.Dynamic.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
			} else {
				obj, err = c.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
			}

			if err != nil {
				node := &ResourceNode{
					Kind:   kind,
					Name:   name,
					Status: string(model.StatusError),
					Resource: &model.Resource{
						Kind:   kind,
						Name:   name,
						Status: model.StatusError,
					},
				}
				mu.Lock()
				results[idx] = fetchResult{node: node, idx: idx}
				mu.Unlock()
				return
			}

			node := buildResourceNode(obj)
			mu.Lock()
			results[idx] = fetchResult{node: node, obj: obj, idx: idx}
			mu.Unlock()
		}(i, refKind, refName, refNS, refAPIVersion)
	}

	wg.Wait()

	// Attach children in order and recurse for nested XRs
	for _, r := range results {
		if r.node == nil {
			continue
		}
		parent.Children = append(parent.Children, r.node)

		// Recurse if this composed resource is itself an XR (has resourceRefs)
		if r.obj != nil {
			if _, found, _ := unstructured.NestedSlice(r.obj.Object, "spec", "resourceRefs"); found {
				if err := discoverComposed(ctx, c, r.obj, r.node); err != nil {
					continue // don't fail the whole tree for one subtree
				}
			}
		}
	}

	return nil
}

func buildResourceNode(obj *unstructured.Unstructured) *ResourceNode {
	conditions := ParseConditions(obj)
	status := DeriveStatus(conditions)
	synced := FindCondition(conditions, "Synced")
	ready := FindCondition(conditions, "Ready")

	extName := ""
	if ann := obj.GetAnnotations(); ann != nil {
		extName = ann["crossplane.io/external-name"]
	}

	var age time.Duration
	if ts := obj.GetCreationTimestamp(); !ts.IsZero() {
		age = time.Since(ts.Time)
	}

	res := &model.Resource{
		APIVersion:   obj.GetAPIVersion(),
		Kind:         obj.GetKind(),
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		ExternalName: extName,
		Synced:       synced,
		Ready:        ready,
		Conditions:   conditions,
		Status:       status,
		Age:          age,
		Raw:          obj,
	}

	return &ResourceNode{
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		APIVersion: obj.GetAPIVersion(),
		Status:     string(status),
		Resource:   res,
	}
}

func getResourceRef(obj *unstructured.Unstructured) (struct{ Kind, Name string }, bool) {
	ref, found, err := unstructured.NestedMap(obj.Object, "spec", "resourceRef")
	if err != nil || !found {
		return struct{ Kind, Name string }{}, false
	}
	kind := stringField(ref, "kind")
	name := stringField(ref, "name")
	if kind == "" || name == "" {
		return struct{ Kind, Name string }{}, false
	}
	return struct{ Kind, Name string }{Kind: kind, Name: name}, true
}

type resolvedGVR struct {
	schema.GroupVersionResource
	Namespaced bool
}

func resolveGVR(c *Client, kind string) (resolvedGVR, error) {
	_, resources, err := c.Discovery.ServerGroupsAndResources()
	if err != nil {
		// ServerGroupsAndResources can return partial results with an error.
		// Only fail if we got nothing.
		if len(resources) == 0 {
			return resolvedGVR{}, fmt.Errorf("discovering API resources: %w", err)
		}
	}

	for _, list := range resources {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range list.APIResources {
			if strings.EqualFold(r.Kind, kind) || strings.EqualFold(r.Name, kind) {
				return resolvedGVR{
					GroupVersionResource: schema.GroupVersionResource{
						Group:    gv.Group,
						Version:  gv.Version,
						Resource: r.Name,
					},
					Namespaced: r.Namespaced,
				}, nil
			}
		}
	}

	return resolvedGVR{}, fmt.Errorf("kind %q not found in cluster", kind)
}

func resolveGVRFromAPIVersion(c *Client, apiVersion, kind string) (schema.GroupVersionResource, error) {
	if apiVersion == "" {
		return schema.GroupVersionResource{}, fmt.Errorf("empty apiVersion")
	}

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	resources, err := c.Discovery.ServerResourcesForGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	for _, r := range resources.APIResources {
		if strings.EqualFold(r.Kind, kind) {
			return schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}, nil
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("kind %q not found in group %q", kind, apiVersion)
}

func flatten(node *ResourceNode) []*ResourceNode {
	if node == nil {
		return nil
	}
	result := []*ResourceNode{node}
	for _, child := range node.Children {
		result = append(result, flatten(child)...)
	}
	return result
}

func computeStats(nodes []*ResourceNode) model.TreeStats {
	stats := model.TreeStats{Total: len(nodes)}
	for _, n := range nodes {
		switch model.StatusSummary(n.Status) {
		case model.StatusHealthy:
			stats.Healthy++
		case model.StatusProgressing:
			stats.Progressing++
		case model.StatusError:
			stats.Error++
		default:
			stats.Unknown++
		}
	}
	return stats
}

