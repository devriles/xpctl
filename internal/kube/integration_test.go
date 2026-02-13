//go:build integration

package kube

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// integrationClient returns a real Client connected to the kind cluster.
// It skips the test if no kubeconfig is available.
func integrationClient(t *testing.T) *Client {
	t.Helper()

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		// Fall back to default kubeconfig location
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine home directory for kubeconfig")
		}
		kubeconfig = home + "/.kube/config"
	}

	c, err := NewClient(kubeconfig, "kind-xpctl-integration")
	if err != nil {
		t.Skipf("cannot connect to integration cluster: %v", err)
	}
	return c
}

func TestIntegration_NewClient(t *testing.T) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine home directory")
		}
		kubeconfig = home + "/.kube/config"
	}

	c, err := NewClient(kubeconfig, "kind-xpctl-integration")
	if err != nil {
		t.Skipf("cannot connect to integration cluster: %v", err)
	}

	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.Dynamic == nil {
		t.Error("Dynamic client is nil")
	}
	if c.Discovery == nil {
		t.Error("Discovery client is nil")
	}
	if c.Config == nil {
		t.Error("Config is nil")
	}
}

func TestIntegration_Discover_TopDown(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Discover(ctx, c, "XNopApp", "test-app", "")
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if result.Root == nil {
		t.Fatal("Root is nil")
	}
	if result.Root.Kind != "XNopApp" {
		t.Errorf("Root.Kind = %q, want %q", result.Root.Kind, "XNopApp")
	}
	if result.Root.Name != "test-app" {
		t.Errorf("Root.Name = %q, want %q", result.Root.Name, "test-app")
	}
	if result.Root.Status != "Healthy" {
		t.Errorf("Root.Status = %q, want %q", result.Root.Status, "Healthy")
	}

	// XR should have 2 ClusterNopResource children
	if len(result.Root.Children) != 2 {
		t.Fatalf("Root.Children = %d, want 2", len(result.Root.Children))
	}
	for i, child := range result.Root.Children {
		if child.Kind != "ClusterNopResource" {
			t.Errorf("child[%d].Kind = %q, want %q", i, child.Kind, "ClusterNopResource")
		}
		if child.Status != "Healthy" {
			t.Errorf("child[%d].Status = %q, want %q", i, child.Status, "Healthy")
		}
	}

	// Stats
	if result.Stats.Total != 3 {
		t.Errorf("Stats.Total = %d, want 3", result.Stats.Total)
	}
	if result.Stats.Healthy != 3 {
		t.Errorf("Stats.Healthy = %d, want 3", result.Stats.Healthy)
	}
}

func TestIntegration_Discover_Claim(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Discover(ctx, c, "NopApp", "test-app-claim", "default")
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if result.Root == nil {
		t.Fatal("Root is nil")
	}
	if result.Root.Kind != "NopApp" {
		t.Errorf("Root.Kind = %q, want %q", result.Root.Kind, "NopApp")
	}
	if result.Root.Name != "test-app-claim" {
		t.Errorf("Root.Name = %q, want %q", result.Root.Name, "test-app-claim")
	}

	// Claim should have 1 child: the XNopApp XR
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children = %d, want 1 (the XR)", len(result.Root.Children))
	}
	xrNode := result.Root.Children[0]
	if xrNode.Kind != "XNopApp" {
		t.Errorf("XR child.Kind = %q, want %q", xrNode.Kind, "XNopApp")
	}

	// XR should have 2 ClusterNopResource grandchildren
	if len(xrNode.Children) != 2 {
		t.Fatalf("XR.Children = %d, want 2", len(xrNode.Children))
	}
	for i, child := range xrNode.Children {
		if child.Kind != "ClusterNopResource" {
			t.Errorf("grandchild[%d].Kind = %q, want %q", i, child.Kind, "ClusterNopResource")
		}
	}
}

func TestIntegration_Discover_NotFound(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := Discover(ctx, c, "XNopApp", "nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent resource, got nil")
	}
}

func TestIntegration_Discover_UnknownKind(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := Discover(ctx, c, "FakeKind", "x", "")
	if err == nil {
		t.Error("expected error for unknown kind, got nil")
	}
}

func TestIntegration_Discover_BottomUp(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First, discover top-down to find a NopResource name
	topDown, err := Discover(ctx, c, "XNopApp", "test-app", "")
	if err != nil {
		t.Fatalf("top-down Discover() error: %v", err)
	}
	if len(topDown.Root.Children) == 0 {
		t.Fatal("no children found on XR to test bottom-up traversal")
	}

	childName := topDown.Root.Children[0].Name

	// Now discover bottom-up from the ClusterNopResource
	result, err := Discover(ctx, c, "ClusterNopResource", childName, "")
	if err != nil {
		t.Fatalf("bottom-up Discover() error: %v", err)
	}

	// Should re-root to the parent XNopApp
	if result.Root == nil {
		t.Fatal("Root is nil")
	}
	if result.Root.Kind != "XNopApp" {
		t.Errorf("Root.Kind = %q, want %q (should re-root to parent XR)", result.Root.Kind, "XNopApp")
	}
	if result.Root.Name != "test-app" {
		t.Errorf("Root.Name = %q, want %q", result.Root.Name, "test-app")
	}
}

func TestIntegration_ListComposites(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, err := ListComposites(ctx, c)
	if err != nil {
		t.Fatalf("ListComposites() error: %v", err)
	}

	foundXR := false
	foundClaim := false
	for _, item := range items {
		if item.Kind == "XNopApp" {
			foundXR = true
		}
		if item.Kind == "NopApp" {
			foundClaim = true
		}
	}

	if !foundXR {
		t.Error("ListComposites() should contain XNopApp items")
	}
	if !foundClaim {
		t.Error("ListComposites() should contain NopApp claim items")
	}
}

func TestIntegration_FetchEvents(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// FetchEvents should not error even if no events exist
	_, err := FetchEvents(ctx, c, "XNopApp", "test-app", "")
	if err != nil {
		t.Fatalf("FetchEvents() error: %v", err)
	}
}

func TestIntegration_Discover_NestedXR(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Discover(ctx, c, "XNopApp", "test-app-nested", "")
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if result.Root == nil {
		t.Fatal("Root is nil")
	}
	if result.Root.Kind != "XNopApp" {
		t.Errorf("Root.Kind = %q, want %q", result.Root.Kind, "XNopApp")
	}

	// Root XR should have 1 child: the sub-XR (XNopSubnet)
	if len(result.Root.Children) != 1 {
		t.Fatalf("Root.Children = %d, want 1", len(result.Root.Children))
	}
	subXR := result.Root.Children[0]
	if subXR.Kind != "XNopSubnet" {
		t.Errorf("child.Kind = %q, want %q", subXR.Kind, "XNopSubnet")
	}

	// Sub-XR should have 1 grandchild: a ClusterNopResource
	if len(subXR.Children) != 1 {
		t.Fatalf("subXR.Children = %d, want 1", len(subXR.Children))
	}
	leaf := subXR.Children[0]
	if leaf.Kind != "ClusterNopResource" {
		t.Errorf("grandchild.Kind = %q, want %q", leaf.Kind, "ClusterNopResource")
	}

	// 3 levels deep, all healthy
	if result.Stats.Total != 3 {
		t.Errorf("Stats.Total = %d, want 3", result.Stats.Total)
	}
	if result.Stats.Healthy != 3 {
		t.Errorf("Stats.Healthy = %d, want 3", result.Stats.Healthy)
	}
}

func TestIntegration_Discover_UnhealthyResource(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Poll until the healthy child has reconciled — Crossplane's reconciliation
	// loop may not have applied conditions yet in slower CI environments.
	var healthyCount, unhealthyCount int
	deadline := time.Now().Add(60 * time.Second)
	for {
		result, err := Discover(ctx, c, "XNopApp", "test-app-unhealthy", "")
		if err != nil {
			t.Fatalf("Discover() error: %v", err)
		}
		if result.Root == nil {
			t.Fatal("Root is nil")
		}
		if len(result.Root.Children) != 2 {
			t.Fatalf("Root.Children = %d, want 2", len(result.Root.Children))
		}

		healthyCount = 0
		unhealthyCount = 0
		for _, child := range result.Root.Children {
			if child.Kind != "ClusterNopResource" {
				t.Errorf("child.Kind = %q, want %q", child.Kind, "ClusterNopResource")
			}
			switch child.Status {
			case "Healthy":
				healthyCount++
			default:
				unhealthyCount++
			}
		}

		if healthyCount == 1 && unhealthyCount == 1 {
			break
		}

		if time.Now().After(deadline) {
			t.Errorf("timed out waiting for expected state: healthy=%d (want 1), unhealthy=%d (want 1)", healthyCount, unhealthyCount)
			for _, child := range result.Root.Children {
				t.Logf("  child %q status=%q", child.Name, child.Status)
			}
			t.FailNow()
		}

		time.Sleep(2 * time.Second)
	}

	// Re-discover for stats validation
	result, err := Discover(ctx, c, "XNopApp", "test-app-unhealthy", "")
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}
	if result.Stats.Healthy == result.Stats.Total {
		t.Error("expected at least one non-healthy resource in stats")
	}
}

func TestIntegration_FindProviderPod(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Wait briefly for the provider pod to be running
	nopResourceGVR := schema.GroupVersionResource{
		Group:    "nop.crossplane.io",
		Version:  "v1alpha1",
		Resource: "nopresources",
	}

	// Verify the NopResource API is available (proves provider-nop is installed)
	_, err := c.Dynamic.Resource(nopResourceGVR).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		t.Skipf("NopResource API not available, provider-nop may not be installed: %v", err)
	}

	podName, containerName, err := FindProviderPod(ctx, c, "nop.crossplane.io/v1alpha1")
	if err != nil {
		t.Fatalf("FindProviderPod() error: %v", err)
	}

	if podName == "" {
		t.Error("podName is empty")
	}
	if !strings.Contains(podName, "provider-nop") {
		t.Errorf("podName = %q, expected to contain %q", podName, "provider-nop")
	}
	if containerName == "" {
		t.Error("containerName is empty")
	}
}
