package tui

import (
	"testing"

	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
)

func TestNewLogViewModel(t *testing.T) {
	node := &kube.ResourceNode{
		Kind: "Bucket", Name: "my-bucket",
		Resource: &model.Resource{Kind: "Bucket", Name: "my-bucket"},
	}
	m := newLogViewModel(node, 80, 24)

	if !m.ready {
		t.Error("expected ready = true")
	}
	if !m.autoScroll {
		t.Error("expected autoScroll = true")
	}
	if m.node != node {
		t.Error("node not set")
	}
	if len(m.lines) != 0 {
		t.Errorf("lines = %d, want 0", len(m.lines))
	}
}

func TestAppendLine(t *testing.T) {
	m := newLogViewModel(nil, 80, 24)

	m.appendLine("line 1")
	if len(m.lines) != 1 {
		t.Errorf("after 1 append: lines = %d", len(m.lines))
	}

	m.appendLine("line 2")
	m.appendLine("line 3")
	if len(m.lines) != 3 {
		t.Errorf("after 3 appends: lines = %d", len(m.lines))
	}

	if m.lines[0] != "line 1" {
		t.Errorf("lines[0] = %q", m.lines[0])
	}
	if m.lines[2] != "line 3" {
		t.Errorf("lines[2] = %q", m.lines[2])
	}
}
