package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
)

func TestSetVersion(t *testing.T) {
	old := version
	defer func() { version = old }()

	SetVersion("1.2.3")
	if version != "1.2.3" {
		t.Errorf("version = %q, want %q", version, "1.2.3")
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"just under a minute", 59 * time.Second, "59s"},
		{"one minute", time.Minute, "1m"},
		{"minutes", 45 * time.Minute, "45m"},
		{"one hour", time.Hour, "1h"},
		{"hours", 6 * time.Hour, "6h"},
		{"one day", 24 * time.Hour, "1d"},
		{"days", 72 * time.Hour, "3d"},
		{"zero", 0, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.FormatAge(tt.d)
			if got != tt.want {
				t.Errorf("FormatAge(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestNodeToJSON(t *testing.T) {
	node := &kube.ResourceNode{
		Kind:       "XMyApp",
		APIVersion: "example.org/v1",
		Name:       "my-app",
		Namespace:  "",
		Status:     "Healthy",
		Resource: &model.Resource{
			Kind:         "XMyApp",
			APIVersion:   "example.org/v1",
			Name:         "my-app",
			ExternalName: "ext-app",
			Age:          2 * time.Hour,
			Synced:       &model.Condition{Type: "Synced", Status: "True"},
			Ready:        &model.Condition{Type: "Ready", Status: "True"},
			Conditions: []model.Condition{
				{Type: "Synced", Status: "True", Reason: "ReconcileSuccess"},
				{Type: "Ready", Status: "True", Reason: "Available"},
			},
		},
		Children: []*kube.ResourceNode{
			{
				Kind:       "Bucket",
				APIVersion: "s3.aws.upbound.io/v1beta1",
				Name:       "my-bucket",
				Namespace:  "default",
				Status:     "Error",
				Resource: &model.Resource{
					Kind:       "Bucket",
					APIVersion: "s3.aws.upbound.io/v1beta1",
					Name:       "my-bucket",
					Namespace:  "default",
					Age:        30 * time.Minute,
					Synced:     &model.Condition{Type: "Synced", Status: "False"},
					Conditions: []model.Condition{
						{Type: "Synced", Status: "False", Reason: "ReconcileError", Message: "access denied"},
					},
				},
			},
		},
	}

	jr := nodeToJSON(node)

	if jr.Kind != "XMyApp" {
		t.Errorf("Kind = %q, want %q", jr.Kind, "XMyApp")
	}
	if jr.Status != "Healthy" {
		t.Errorf("Status = %q, want %q", jr.Status, "Healthy")
	}
	if jr.ExternalName != "ext-app" {
		t.Errorf("ExternalName = %q, want %q", jr.ExternalName, "ext-app")
	}
	if jr.Synced != "True" {
		t.Errorf("Synced = %q, want %q", jr.Synced, "True")
	}
	if jr.Ready != "True" {
		t.Errorf("Ready = %q, want %q", jr.Ready, "True")
	}
	if jr.Age != "2h" {
		t.Errorf("Age = %q, want %q", jr.Age, "2h")
	}
	if len(jr.Conditions) != 2 {
		t.Errorf("len(Conditions) = %d, want 2", len(jr.Conditions))
	}
	if len(jr.Children) != 1 {
		t.Fatalf("len(Children) = %d, want 1", len(jr.Children))
	}
	child := jr.Children[0]
	if child.Kind != "Bucket" {
		t.Errorf("child Kind = %q", child.Kind)
	}

	data, err := json.Marshal(jr)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if !json.Valid(data) {
		t.Error("produced invalid JSON")
	}
}

func TestNodeToJSON_NilResource(t *testing.T) {
	node := &kube.ResourceNode{Kind: "Unknown", Name: "orphan", Status: "Unknown"}
	jr := nodeToJSON(node)
	if jr.Age != "" {
		t.Errorf("Age = %q, want empty", jr.Age)
	}
	if jr.Synced != "" {
		t.Errorf("Synced = %q, want empty", jr.Synced)
	}
}

func TestNodeToJSON_NoConditions(t *testing.T) {
	node := &kube.ResourceNode{
		Kind: "ConfigMap", Name: "my-cm", Status: "Unknown",
		Resource: &model.Resource{Kind: "ConfigMap", Name: "my-cm"},
	}
	jr := nodeToJSON(node)
	if jr.Conditions != nil {
		t.Errorf("Conditions = %v, want nil", jr.Conditions)
	}
}

// captureStdout redirects os.Stdout to a buffer, runs f, and returns the captured output.
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrintTree_Output(t *testing.T) {
	root := &kube.ResourceNode{
		Kind: "XMyApp", Name: "my-app", Status: "Healthy",
		Children: []*kube.ResourceNode{
			{Kind: "Bucket", Name: "my-bucket", Status: "Error"},
			{Kind: "Instance", Name: "my-instance", Status: "Progressing"},
		},
	}

	out := captureStdout(func() {
		printTree(root, "", true)
	})

	if !strings.Contains(out, "XMyApp/my-app") {
		t.Errorf("missing root in output: %q", out)
	}
	if !strings.Contains(out, "Bucket/my-bucket") {
		t.Errorf("missing bucket in output: %q", out)
	}
	if !strings.Contains(out, "Instance/my-instance") {
		t.Errorf("missing instance in output: %q", out)
	}
	if !strings.Contains(out, "✔") {
		t.Error("missing healthy icon")
	}
	if !strings.Contains(out, "✘") {
		t.Error("missing error icon")
	}
	if !strings.Contains(out, "⟳") {
		t.Error("missing progressing icon")
	}
	if !strings.Contains(out, "├── ") {
		t.Error("missing tree connector ├── ")
	}
	if !strings.Contains(out, "└── ") {
		t.Error("missing tree connector └── ")
	}
}

func TestPrintTree_NilRoot(t *testing.T) {
	out := captureStdout(func() {
		printTree(nil, "", true)
	})
	if out != "" {
		t.Errorf("nil root should produce no output, got %q", out)
	}
}

func TestPrintTree_UnknownStatus(t *testing.T) {
	out := captureStdout(func() {
		printTree(&kube.ResourceNode{Kind: "X", Name: "y", Status: "Unknown"}, "", true)
	})
	if !strings.Contains(out, "?") {
		t.Error("unknown status should show ? icon")
	}
}

func TestPrintJSON(t *testing.T) {
	tree := &kube.CompositionResult{
		Root: &kube.ResourceNode{
			Kind: "XMyApp", Name: "my-app", APIVersion: "example.org/v1", Status: "Healthy",
			Resource: &model.Resource{
				Kind:       "XMyApp",
				Name:       "my-app",
				APIVersion: "example.org/v1",
				Status:     model.StatusHealthy,
				Synced:     &model.Condition{Status: "True"},
				Ready:      &model.Condition{Status: "True"},
			},
			Children: []*kube.ResourceNode{
				{
					Kind: "Bucket", Name: "b1", Status: "Error",
					Resource: &model.Resource{Kind: "Bucket", Name: "b1"},
				},
			},
		},
	}

	out := captureStdout(func() {
		err := printJSON(tree)
		if err != nil {
			t.Errorf("printJSON error: %v", err)
		}
	})

	if !json.Valid([]byte(out)) {
		t.Errorf("printJSON produced invalid JSON: %q", out)
	}

	var parsed jsonResource
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if parsed.Kind != "XMyApp" {
		t.Errorf("kind = %q", parsed.Kind)
	}
	if len(parsed.Children) != 1 {
		t.Errorf("children = %d, want 1", len(parsed.Children))
	}
}

func TestPrintWide(t *testing.T) {
	tree := &kube.CompositionResult{
		Root: &kube.ResourceNode{
			Kind: "XMyApp", Name: "my-app", Status: "Healthy",
			Resource: &model.Resource{
				Kind:         "XMyApp",
				Name:         "my-app",
				Status:       model.StatusHealthy,
				ExternalName: "ext-app",
				Age:          2 * time.Hour,
				Synced:       &model.Condition{Status: "True"},
				Ready:        &model.Condition{Status: "True"},
			},
		},
		Flat: []*kube.ResourceNode{
			{
				Kind: "XMyApp", Name: "my-app", Status: "Healthy",
				Resource: &model.Resource{
					Kind: "XMyApp", Name: "my-app", Status: model.StatusHealthy,
					ExternalName: "ext-app", Age: 2 * time.Hour,
					Synced: &model.Condition{Status: "True"},
					Ready:  &model.Condition{Status: "True"},
				},
			},
			{
				Kind: "Bucket", Name: "my-bucket", Status: "Error",
				Resource: &model.Resource{
					Kind: "Bucket", Name: "my-bucket", Status: model.StatusError,
					Synced: &model.Condition{Status: "False"},
				},
			},
		},
		Stats: model.TreeStats{Total: 2, Healthy: 1, Error: 1},
	}

	out := captureStdout(func() {
		err := printWide(tree)
		if err != nil {
			t.Errorf("printWide error: %v", err)
		}
	})

	// Check header
	if !strings.Contains(out, "KIND") {
		t.Error("missing KIND header")
	}
	if !strings.Contains(out, "STATUS") {
		t.Error("missing STATUS header")
	}
	if !strings.Contains(out, "EXTERNAL-NAME") {
		t.Error("missing EXTERNAL-NAME header")
	}

	// Check data
	if !strings.Contains(out, "XMyApp") {
		t.Error("missing XMyApp in output")
	}
	if !strings.Contains(out, "my-bucket") {
		t.Error("missing my-bucket in output")
	}
	if !strings.Contains(out, "ext-app") {
		t.Error("missing external name in output")
	}

	// Check summary
	if !strings.Contains(out, "1/2 Ready") {
		t.Error("missing summary")
	}
	if !strings.Contains(out, "1 Error") {
		t.Error("missing error count in summary")
	}
}

func TestPrintOutput_AllModes(t *testing.T) {
	tree := &kube.CompositionResult{
		Root: &kube.ResourceNode{
			Kind: "XMyApp", Name: "test", Status: "Healthy",
			Resource: &model.Resource{Kind: "XMyApp", Name: "test"},
		},
		Flat: []*kube.ResourceNode{
			{
				Kind: "XMyApp", Name: "test", Status: "Healthy",
				Resource: &model.Resource{Kind: "XMyApp", Name: "test"},
			},
		},
		Stats: model.TreeStats{Total: 1, Healthy: 1},
	}

	for _, mode := range []string{"tree", "json", "wide"} {
		t.Run(mode, func(t *testing.T) {
			captureStdout(func() {
				err := printOutput(tree, mode)
				if err != nil {
					t.Errorf("printOutput(%q) error: %v", mode, err)
				}
			})
		})
	}
}

func TestPrintOutput_UnknownMode(t *testing.T) {
	tree := &kube.CompositionResult{
		Root: &kube.ResourceNode{Kind: "Test", Name: "test", Status: "Healthy"},
	}

	err := printOutput(tree, "invalid")
	if err == nil {
		t.Error("expected error for unknown mode")
	}
	if !strings.Contains(err.Error(), "unknown output mode") {
		t.Errorf("error = %q, want 'unknown output mode'", err.Error())
	}
}

func TestPrintWide_WithProgressing(t *testing.T) {
	tree := &kube.CompositionResult{
		Flat: []*kube.ResourceNode{
			{
				Kind: "XMyApp", Name: "my-app", Status: "Healthy",
				Resource: &model.Resource{Kind: "XMyApp", Name: "my-app", Status: model.StatusHealthy},
			},
			{
				Kind: "Bucket", Name: "b1", Status: "Progressing",
				Resource: &model.Resource{Kind: "Bucket", Name: "b1", Status: model.StatusProgressing},
			},
		},
		Stats: model.TreeStats{Total: 2, Healthy: 1, Progressing: 1},
	}

	out := captureStdout(func() {
		err := printWide(tree)
		if err != nil {
			t.Errorf("printWide error: %v", err)
		}
	})

	if !strings.Contains(out, "1/2 Ready") {
		t.Error("missing ready count in summary")
	}
	if !strings.Contains(out, "1 Progressing") {
		t.Error("missing progressing count in summary")
	}
	// Should NOT contain error count
	if strings.Contains(out, "Error") {
		t.Error("should not contain Error in summary")
	}
}

func TestPrintWide_NilResource(t *testing.T) {
	tree := &kube.CompositionResult{
		Flat: []*kube.ResourceNode{
			{Kind: "Test", Name: "test", Status: "Unknown"},
		},
		Stats: model.TreeStats{Total: 1, Unknown: 1},
	}

	out := captureStdout(func() {
		err := printWide(tree)
		if err != nil {
			t.Errorf("printWide error: %v", err)
		}
	})

	// Should show dashes for missing fields
	if !strings.Contains(out, "-") {
		t.Error("nil resource fields should show dashes")
	}
}
