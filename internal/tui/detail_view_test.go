package tui

import (
	"testing"
	"time"

	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
)

func TestNewDetailViewModel(t *testing.T) {
	node := &kube.ResourceNode{
		Kind: "Bucket", Name: "my-bucket",
		Resource: &model.Resource{
			Kind: "Bucket", Name: "my-bucket", Status: model.StatusHealthy,
		},
	}

	m := newDetailViewModel(node, 80, 24)
	if !m.ready {
		t.Error("expected ready = true")
	}
	if m.node != node {
		t.Error("node not set")
	}
}

func TestFilterCrossplaneAnnotations(t *testing.T) {
	annotations := map[string]string{
		"crossplane.io/external-name":       "ext",
		"crossplane.io/composition-name":    "comp",
		"upbound.io/provider":               "aws",
		"kubectl.kubernetes.io/restartedAt": "2025-01-01",
		"app.kubernetes.io/name":            "test",
	}

	filtered := filterCrossplaneAnnotations(annotations)

	if len(filtered) != 3 {
		t.Errorf("filtered = %d annotations, want 3", len(filtered))
	}
	if filtered["crossplane.io/external-name"] != "ext" {
		t.Error("missing crossplane.io/external-name")
	}
	if filtered["upbound.io/provider"] != "aws" {
		t.Error("missing upbound.io/provider")
	}
	if _, ok := filtered["kubectl.kubernetes.io/restartedAt"]; ok {
		t.Error("should not include kubectl annotations")
	}
}

func TestFilterCrossplaneAnnotations_Nil(t *testing.T) {
	filtered := filterCrossplaneAnnotations(nil)
	if len(filtered) != 0 {
		t.Errorf("nil input should return empty map, got %d", len(filtered))
	}
}

func TestFormatAgeLong(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"seconds", 30 * time.Second, "30 seconds"},
		{"minutes_seconds", 5*time.Minute + 30*time.Second, "5m30s"},
		{"hours_minutes", 2*time.Hour + 15*time.Minute, "2h15m"},
		{"days_hours", 3*24*time.Hour + 6*time.Hour, "3d6h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAgeLong(tt.d)
			if got != tt.want {
				t.Errorf("formatAgeLong(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
