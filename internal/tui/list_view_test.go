package tui

import (
	"testing"

	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
)

func TestBuildRows(t *testing.T) {
	tests := []struct {
		name       string
		root       *kube.ResourceNode
		wantLen    int
		wantDepths []int
	}{
		{
			name:       "single root",
			root:       &kube.ResourceNode{Kind: "XMyApp", Name: "my-app"},
			wantLen:    1,
			wantDepths: []int{0},
		},
		{
			name: "root with two children",
			root: &kube.ResourceNode{
				Kind: "XMyApp", Name: "my-app",
				Children: []*kube.ResourceNode{
					{Kind: "Bucket", Name: "b1"},
					{Kind: "Bucket", Name: "b2"},
				},
			},
			wantLen:    3,
			wantDepths: []int{0, 1, 1},
		},
		{
			name: "nested tree",
			root: &kube.ResourceNode{
				Kind: "XMyApp", Name: "my-app",
				Children: []*kube.ResourceNode{
					{
						Kind: "XSubApp", Name: "sub",
						Children: []*kube.ResourceNode{
							{Kind: "Instance", Name: "i1"},
						},
					},
				},
			},
			wantLen:    3,
			wantDepths: []int{0, 1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rows []listRow
			buildRows(tt.root, "", true, 0, &rows)

			if len(rows) != tt.wantLen {
				t.Errorf("buildRows() produced %d rows, want %d", len(rows), tt.wantLen)
			}

			for i, wantDepth := range tt.wantDepths {
				if i >= len(rows) {
					break
				}
				if rows[i].depth != wantDepth {
					t.Errorf("row[%d].depth = %d, want %d", i, rows[i].depth, wantDepth)
				}
			}

			// Root should have empty prefix
			if len(rows) > 0 && rows[0].prefix != "" {
				t.Errorf("root prefix = %q, want empty", rows[0].prefix)
			}

			// Children should have tree chars
			if len(rows) > 1 && rows[1].prefix == "" {
				t.Error("child prefix should not be empty")
			}
		})
	}
}

func TestBuildRows_TreeChars(t *testing.T) {
	root := &kube.ResourceNode{
		Kind: "XR", Name: "root",
		Children: []*kube.ResourceNode{
			{Kind: "A", Name: "first-child"},
			{Kind: "B", Name: "last-child"},
		},
	}

	var rows []listRow
	buildRows(root, "", true, 0, &rows)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// First child should use ├──
	if rows[1].prefix != "├── " {
		t.Errorf("first child prefix = %q, want %q", rows[1].prefix, "├── ")
	}

	// Last child should use └──
	if rows[2].prefix != "└── " {
		t.Errorf("last child prefix = %q, want %q", rows[2].prefix, "└── ")
	}
}

func TestNewListViewModel(t *testing.T) {
	result := &kube.CompositionResult{
		Root: &kube.ResourceNode{
			Kind: "XMyApp", Name: "my-app", Status: "Healthy",
			Children: []*kube.ResourceNode{
				{Kind: "Bucket", Name: "b1", Status: "Healthy"},
				{Kind: "Instance", Name: "i1", Status: "Error"},
			},
		},
		Stats: model.TreeStats{Total: 3, Healthy: 2, Error: 1},
	}

	m := newListViewModel(result)

	if len(m.rows) != 3 {
		t.Errorf("rows = %d, want 3", len(m.rows))
	}
	if m.rootKind != "XMyApp" {
		t.Errorf("rootKind = %q, want %q", m.rootKind, "XMyApp")
	}
	if m.rootName != "my-app" {
		t.Errorf("rootName = %q, want %q", m.rootName, "my-app")
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
}

func TestNewListViewModel_EmptyResult(t *testing.T) {
	result := &kube.CompositionResult{}
	m := newListViewModel(result)

	if len(m.rows) != 0 {
		t.Errorf("rows = %d, want 0", len(m.rows))
	}
}

func TestSelectedNode(t *testing.T) {
	result := &kube.CompositionResult{
		Root: &kube.ResourceNode{
			Kind: "XR", Name: "root",
			Children: []*kube.ResourceNode{
				{Kind: "A", Name: "child"},
			},
		},
	}

	m := newListViewModel(result)

	// cursor=0 → root
	node := m.selectedNode()
	if node == nil || node.Name != "root" {
		t.Errorf("selectedNode() at cursor=0 = %v, want root", node)
	}

	// cursor=1 → child
	m.cursor = 1
	node = m.selectedNode()
	if node == nil || node.Name != "child" {
		t.Errorf("selectedNode() at cursor=1 = %v, want child", node)
	}

	// cursor out of bounds
	m.cursor = 99
	node = m.selectedNode()
	if node != nil {
		t.Errorf("selectedNode() at cursor=99 = %v, want nil", node)
	}
}

func TestStatusIconAndStyle(t *testing.T) {
	tests := []struct {
		status model.StatusSummary
		icon   string
	}{
		{model.StatusHealthy, IconHealthy},
		{model.StatusError, IconError},
		{model.StatusProgressing, IconProgressing},
		{model.StatusUnknown, IconUnknown},
		{"SomethingElse", IconUnknown},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			icon, _ := statusIconAndStyle(tt.status)
			if icon != tt.icon {
				t.Errorf("icon = %q, want %q", icon, tt.icon)
			}
		})
	}
}
