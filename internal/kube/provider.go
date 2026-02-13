package kube

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FindProviderPod resolves the provider pod for a given API group, returning (podName, containerName, error).
func FindProviderPod(ctx context.Context, c *Client, apiVersion string) (string, string, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return "", "", fmt.Errorf("parsing apiVersion %q: %w", apiVersion, err)
	}
	apiGroup := gv.Group

	podName, containerName, err := findViaProviderRevision(ctx, c, apiGroup)
	if err == nil {
		return podName, containerName, nil
	}

	podName, containerName, err = findViaHeuristic(ctx, c, apiGroup)
	if err == nil {
		return podName, containerName, nil
	}

	return "", "", fmt.Errorf("could not find provider pod for API group %q", apiGroup)
}

// findViaProviderRevision traces API group → ProviderRevision → Deployment → Pod.
func findViaProviderRevision(ctx context.Context, c *Client, apiGroup string) (string, string, error) {
	provRevGVR := schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "providerrevisions",
	}

	revisions, err := c.Dynamic.Resource(provRevGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", fmt.Errorf("listing provider revisions: %w", err)
	}

	for _, rev := range revisions.Items {
		objRefs, found, _ := unstructured.NestedSlice(rev.Object, "status", "objectRefs")
		if !found {
			continue
		}

		ownsGroup := false
		for _, ref := range objRefs {
			refMap, ok := ref.(map[string]interface{})
			if !ok {
				continue
			}
			refKind := stringField(refMap, "kind")
			refName := stringField(refMap, "name")
			if refKind == "CustomResourceDefinition" && strings.Contains(refName, apiGroup) {
				ownsGroup = true
				break
			}
		}

		if !ownsGroup {
			continue
		}

		revName := rev.GetName()
		return findPodForRevision(ctx, c, revName, objRefs)
	}

	return "", "", fmt.Errorf("no provider revision found for API group %q", apiGroup)
}

func findPodForRevision(ctx context.Context, c *Client, revisionName string, objRefs []interface{}) (string, string, error) {
	for _, ref := range objRefs {
		refMap, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		if stringField(refMap, "kind") != "Deployment" {
			continue
		}
		depName := stringField(refMap, "name")
		depNS := stringField(refMap, "namespace")
		if depName == "" {
			continue
		}
		if depNS == "" {
			depNS = "crossplane-system"
		}

		return findPodForDeployment(ctx, c, depName, depNS)
	}

	return findPodByPrefix(ctx, c, "crossplane-system", revisionName)
}

func findPodForDeployment(ctx context.Context, c *Client, depName, depNS string) (string, string, error) {
	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	pods, err := c.Dynamic.Resource(podGVR).Namespace(depNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}

	for _, pod := range pods.Items {
		name := pod.GetName()
		if strings.HasPrefix(name, depName) {
			phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
			if phase == "Running" {
				container := findMainContainer(&pod)
				return name, container, nil
			}
		}
	}

	return "", "", fmt.Errorf("no running pod found for deployment %s/%s", depNS, depName)
}

func findPodByPrefix(ctx context.Context, c *Client, namespace, prefix string) (string, string, error) {
	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	pods, err := c.Dynamic.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", err
	}

	for _, pod := range pods.Items {
		name := pod.GetName()
		if strings.HasPrefix(name, prefix) {
			phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
			if phase == "Running" {
				container := findMainContainer(&pod)
				return name, container, nil
			}
		}
	}

	return "", "", fmt.Errorf("no running pod with prefix %q in namespace %q", prefix, namespace)
}

func findViaHeuristic(ctx context.Context, c *Client, apiGroup string) (string, string, error) {
	// e.g., "s3.aws.upbound.io" → try "provider-aws"
	// e.g., "compute.gcp.upbound.io" → try "provider-gcp"
	// e.g., "network.azure.upbound.io" → try "provider-azure"
	parts := strings.Split(apiGroup, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("cannot derive provider name from %q", apiGroup)
	}

	// Try common patterns
	prefixes := []string{}
	if len(parts) >= 3 {
		// e.g., s3.aws.upbound.io → "provider-aws"
		prefixes = append(prefixes, "provider-"+parts[1])
	}
	// Also try the first part: "provider-s3"
	prefixes = append(prefixes, "provider-"+parts[0])

	for _, prefix := range prefixes {
		pod, container, err := findPodByPrefix(ctx, c, "crossplane-system", prefix)
		if err == nil {
			return pod, container, nil
		}
	}

	return "", "", fmt.Errorf("no provider pod found matching API group %q", apiGroup)
}

func findMainContainer(pod *unstructured.Unstructured) string {
	containers, found, _ := unstructured.NestedSlice(pod.Object, "spec", "containers")
	if !found || len(containers) == 0 {
		return "provider"
	}
	first, ok := containers[0].(map[string]interface{})
	if !ok {
		return "provider"
	}
	name := stringField(first, "name")
	if name == "" {
		return "provider"
	}
	return name
}
