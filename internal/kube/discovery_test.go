package kube

import (
	"testing"
	"time"

	"github.com/devriles/xpctl/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildResourceNode(t *testing.T) {
	tests := []struct {
		name           string
		obj            *unstructured.Unstructured
		wantKind       string
		wantName       string
		wantNamespace  string
		wantStatus     string
		wantExtName    string
		wantHasAge     bool
	}{
		{
			name: "healthy XR with external name",
			obj: makeObj("myapps.example.org/v1", "XMyApp", "my-app", "",
				map[string]string{"crossplane.io/external-name": "ext-my-app"},
				[]interface{}{
					condMap("Synced", "True", "ReconcileSuccess", ""),
					condMap("Ready", "True", "Available", ""),
				},
			),
			wantKind:    "XMyApp",
			wantName:    "my-app",
			wantStatus:  "Healthy",
			wantExtName: "ext-my-app",
		},
		{
			name: "errored managed resource",
			obj: makeObj("s3.aws.upbound.io/v1beta1", "Bucket", "my-bucket", "default",
				nil,
				[]interface{}{
					condMap("Synced", "False", "ReconcileError", "access denied"),
				},
			),
			wantKind:      "Bucket",
			wantName:      "my-bucket",
			wantNamespace: "default",
			wantStatus:    "Error",
		},
		{
			name: "resource with no conditions",
			obj: makeObj("v1", "ConfigMap", "my-cm", "default",
				nil, nil,
			),
			wantKind:      "ConfigMap",
			wantName:      "my-cm",
			wantNamespace: "default",
			wantStatus:    "Unknown",
		},
		{
			name: "progressing resource",
			obj: makeObj("ec2.aws.upbound.io/v1beta1", "Instance", "my-instance", "",
				nil,
				[]interface{}{
					condMap("Synced", "True", "ReconcileSuccess", ""),
					condMap("Ready", "False", "Creating", ""),
				},
			),
			wantKind:   "Instance",
			wantName:   "my-instance",
			wantStatus: "Progressing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := buildResourceNode(tt.obj)

			if node.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", node.Kind, tt.wantKind)
			}
			if node.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", node.Name, tt.wantName)
			}
			if node.Namespace != tt.wantNamespace {
				t.Errorf("Namespace = %q, want %q", node.Namespace, tt.wantNamespace)
			}
			if node.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", node.Status, tt.wantStatus)
			}
			if node.Resource == nil {
				t.Fatal("Resource is nil")
			}
			if node.Resource.ExternalName != tt.wantExtName {
				t.Errorf("ExternalName = %q, want %q", node.Resource.ExternalName, tt.wantExtName)
			}
			if node.Resource.Raw != tt.obj {
				t.Error("Raw should point to the original object")
			}
		})
	}
}

func TestBuildResourceNode_Age(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":              "test-pod",
			"creationTimestamp": metav1.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		},
	}}

	node := buildResourceNode(obj)
	if node.Resource.Age < time.Hour {
		t.Errorf("Age = %v, expected at least 1h", node.Resource.Age)
	}
}

func TestGetResourceRef(t *testing.T) {
	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		wantOk   bool
		wantKind string
		wantName string
	}{
		{
			name: "claim with resourceRef",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"resourceRef": map[string]interface{}{
						"kind": "XMyApp",
						"name": "my-app-abc123",
					},
				},
			}},
			wantOk:   true,
			wantKind: "XMyApp",
			wantName: "my-app-abc123",
		},
		{
			name: "no resourceRef",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"resourceRefs": []interface{}{},
				},
			}},
			wantOk: false,
		},
		{
			name:   "empty object",
			obj:    &unstructured.Unstructured{Object: map[string]interface{}{}},
			wantOk: false,
		},
		{
			name: "resourceRef missing kind",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"resourceRef": map[string]interface{}{
						"name": "my-app-abc123",
					},
				},
			}},
			wantOk: false,
		},
		{
			name: "resourceRef missing name",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"resourceRef": map[string]interface{}{
						"kind": "XMyApp",
					},
				},
			}},
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, ok := getResourceRef(tt.obj)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if tt.wantOk {
				if ref.Kind != tt.wantKind {
					t.Errorf("Kind = %q, want %q", ref.Kind, tt.wantKind)
				}
				if ref.Name != tt.wantName {
					t.Errorf("Name = %q, want %q", ref.Name, tt.wantName)
				}
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name     string
		root     *ResourceNode
		wantLen  int
		wantOrder []string // expected names in DFS order
	}{
		{
			name:    "nil root",
			root:    nil,
			wantLen: 0,
		},
		{
			name:      "single node",
			root:      &ResourceNode{Name: "root"},
			wantLen:   1,
			wantOrder: []string{"root"},
		},
		{
			name: "root with children",
			root: &ResourceNode{
				Name: "root",
				Children: []*ResourceNode{
					{Name: "child-1"},
					{Name: "child-2"},
				},
			},
			wantLen:   3,
			wantOrder: []string{"root", "child-1", "child-2"},
		},
		{
			name: "nested tree (3 levels)",
			root: &ResourceNode{
				Name: "xr",
				Children: []*ResourceNode{
					{
						Name: "sub-xr",
						Children: []*ResourceNode{
							{Name: "managed-1"},
							{Name: "managed-2"},
						},
					},
					{Name: "managed-3"},
				},
			},
			wantLen:   5,
			wantOrder: []string{"xr", "sub-xr", "managed-1", "managed-2", "managed-3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flatten(tt.root)
			if len(got) != tt.wantLen {
				t.Errorf("flatten() returned %d nodes, want %d", len(got), tt.wantLen)
			}
			if tt.wantOrder != nil {
				for i, name := range tt.wantOrder {
					if i >= len(got) {
						break
					}
					if got[i].Name != name {
						t.Errorf("flatten()[%d].Name = %q, want %q", i, got[i].Name, name)
					}
				}
			}
		})
	}
}

func TestComputeStats(t *testing.T) {
	tests := []struct {
		name  string
		nodes []*ResourceNode
		want  model.TreeStats
	}{
		{
			name:  "empty",
			nodes: nil,
			want:  model.TreeStats{},
		},
		{
			name: "all healthy",
			nodes: []*ResourceNode{
				{Status: "Healthy"},
				{Status: "Healthy"},
				{Status: "Healthy"},
			},
			want: model.TreeStats{Total: 3, Healthy: 3},
		},
		{
			name: "mixed statuses",
			nodes: []*ResourceNode{
				{Status: "Healthy"},
				{Status: "Error"},
				{Status: "Progressing"},
				{Status: "Unknown"},
				{Status: "Healthy"},
			},
			want: model.TreeStats{Total: 5, Healthy: 2, Error: 1, Progressing: 1, Unknown: 1},
		},
		{
			name: "all errors",
			nodes: []*ResourceNode{
				{Status: "Error"},
				{Status: "Error"},
			},
			want: model.TreeStats{Total: 2, Error: 2},
		},
		{
			name: "unrecognized status treated as unknown",
			nodes: []*ResourceNode{
				{Status: "SomethingWeird"},
			},
			want: model.TreeStats{Total: 1, Unknown: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeStats(tt.nodes)
			if got != tt.want {
				t.Errorf("computeStats() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHasCategory(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		target     string
		want       bool
	}{
		{"found", []string{"composite", "managed"}, "composite", true},
		{"not found", []string{"managed"}, "composite", false},
		{"empty list", []string{}, "composite", false},
		{"nil list", nil, "composite", false},
		{"claim category", []string{"claim"}, "claim", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasCategory(tt.categories, tt.target); got != tt.want {
				t.Errorf("hasCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringField_Discovery(t *testing.T) {
	m := map[string]interface{}{
		"kind":       "Bucket",
		"name":       "my-bucket",
		"apiVersion": "s3.aws.upbound.io/v1beta1",
		"number":     int64(42),
	}

	tests := []struct {
		key  string
		want string
	}{
		{"kind", "Bucket"},
		{"name", "my-bucket"},
		{"apiVersion", "s3.aws.upbound.io/v1beta1"},
		{"number", ""},
		{"missing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := stringField(m, tt.key); got != tt.want {
				t.Errorf("stringField(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// --- Helpers ---

func makeObj(apiVersion, kind, name, namespace string, annotations map[string]string, conditions []interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name":              name,
			"creationTimestamp": metav1.Now().Format(time.RFC3339),
		},
	}}

	if namespace != "" {
		obj.Object["metadata"].(map[string]interface{})["namespace"] = namespace
	}
	if annotations != nil {
		annMap := make(map[string]interface{}, len(annotations))
		for k, v := range annotations {
			annMap[k] = v
		}
		obj.Object["metadata"].(map[string]interface{})["annotations"] = annMap
	}
	if conditions != nil {
		obj.Object["status"] = map[string]interface{}{
			"conditions": conditions,
		}
	}
	return obj
}

func condMap(condType, status, reason, message string) map[string]interface{} {
	m := map[string]interface{}{
		"type":               condType,
		"status":             status,
		"lastTransitionTime": metav1.Now().Format(time.RFC3339),
	}
	if reason != "" {
		m["reason"] = reason
	}
	if message != "" {
		m["message"] = message
	}
	return m
}
