package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
)

func makeTestResult() *kube.CompositionResult {
	return &kube.CompositionResult{
		Root: &kube.ResourceNode{
			Kind: "XMyApp", Name: "my-app", APIVersion: "example.org/v1", Status: "Healthy",
			Resource: &model.Resource{
				Kind: "XMyApp", Name: "my-app", APIVersion: "example.org/v1",
				Status: model.StatusHealthy,
			},
			Children: []*kube.ResourceNode{
				{
					Kind: "Bucket", Name: "my-bucket", APIVersion: "s3.aws.upbound.io/v1beta1", Status: "Healthy",
					Resource: &model.Resource{
						Kind: "Bucket", Name: "my-bucket", Status: model.StatusHealthy,
					},
				},
				{
					Kind: "Instance", Name: "my-instance", APIVersion: "ec2.aws/v1", Status: "Error",
					Resource: &model.Resource{
						Kind: "Instance", Name: "my-instance", Status: model.StatusError,
					},
				},
			},
		},
		Flat: []*kube.ResourceNode{
			{Kind: "XMyApp", Name: "my-app", Status: "Healthy"},
			{Kind: "Bucket", Name: "my-bucket", Status: "Healthy"},
			{Kind: "Instance", Name: "my-instance", Status: "Error"},
		},
		Stats: model.TreeStats{Total: 3, Healthy: 2, Error: 1},
	}
}

// --- Construction ---

func TestNewApp(t *testing.T) {
	result := makeTestResult()
	m := NewApp(result, nil)

	if m.state != viewList {
		t.Errorf("state = %d, want viewList", m.state)
	}
	if m.tree != result {
		t.Error("tree not set")
	}
	if len(m.listView.rows) != 3 {
		t.Errorf("listView.rows = %d, want 3", len(m.listView.rows))
	}
}

func TestNewAppLoading(t *testing.T) {
	m := NewAppLoading(nil, "XMyApp", "my-app", "")
	if m.state != viewLoading {
		t.Errorf("state = %d, want viewLoading", m.state)
	}
	if m.loadingMsg == "" {
		t.Error("loadingMsg is empty")
	}
}

func TestNewAppPicker(t *testing.T) {
	m := NewAppPicker(nil)
	if m.state != viewLoading {
		t.Errorf("state = %d, want viewLoading", m.state)
	}
}

func TestInit(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init() returned nil cmd, expected spinner tick")
	}
}

// --- Update: message handling ---

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	am := updated.(AppModel)
	if am.width != 120 {
		t.Errorf("width = %d, want 120", am.width)
	}
	if am.height != 40 {
		t.Errorf("height = %d, want 40", am.height)
	}
}

func TestUpdate_SpinnerTick(t *testing.T) {
	m := NewAppLoading(nil, "XMyApp", "my-app", "")
	updated, _ := m.Update(spinner.TickMsg{})
	if updated == nil {
		t.Error("Update returned nil")
	}
}

func TestUpdate_DiscoverDoneMsg(t *testing.T) {
	m := NewAppLoading(nil, "XMyApp", "my-app", "")
	result := makeTestResult()
	updated, _ := m.Update(discoverDoneMsg{result: result})
	am := updated.(AppModel)

	if am.state != viewList {
		t.Errorf("state = %d, want viewList", am.state)
	}
	if am.tree != result {
		t.Error("tree not set after discover")
	}
	if len(am.listView.rows) != 3 {
		t.Errorf("listView.rows = %d, want 3", len(am.listView.rows))
	}
}

func TestUpdate_DiscoverDoneMsg_Error(t *testing.T) {
	m := NewAppLoading(nil, "XMyApp", "my-app", "")
	updated, _ := m.Update(discoverDoneMsg{err: errTest})
	am := updated.(AppModel)

	if am.err == nil {
		t.Error("err should be set")
	}
	if am.state != viewList {
		t.Errorf("state = %d, want viewList on error", am.state)
	}
}

func TestUpdate_CompositeListMsg(t *testing.T) {
	m := NewAppPicker(nil)
	m.width = 80
	m.height = 24

	items := []kube.CompositeItem{
		{Kind: "XMyApp", Name: "app-1", Status: "Healthy"},
		{Kind: "XMyApp", Name: "app-2", Status: "Error"},
	}

	updated, _ := m.Update(compositeListMsg{items: items})
	am := updated.(AppModel)

	if am.state != viewPicker {
		t.Errorf("state = %d, want viewPicker", am.state)
	}
}

func TestUpdate_CompositeListMsg_Empty(t *testing.T) {
	m := NewAppPicker(nil)
	updated, _ := m.Update(compositeListMsg{items: nil})
	am := updated.(AppModel)

	if am.err == nil {
		t.Error("err should be set for empty list")
	}
}

func TestUpdate_CompositeListMsg_Error(t *testing.T) {
	m := NewAppPicker(nil)
	updated, _ := m.Update(compositeListMsg{err: errTest})
	am := updated.(AppModel)
	if am.err == nil {
		t.Error("err should be set for composite list error")
	}
}

func TestUpdate_RefreshDoneMsg(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.listView.cursor = 1

	newResult := makeTestResult()
	updated, _ := m.Update(refreshDoneMsg{result: newResult})
	am := updated.(AppModel)

	if am.tree != newResult {
		t.Error("tree not updated")
	}
	if am.listView.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (preserved)", am.listView.cursor)
	}
}

func TestUpdate_RefreshDoneMsg_Error(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	updated, _ := m.Update(refreshDoneMsg{err: errTest})
	am := updated.(AppModel)
	if am.err == nil {
		t.Error("err should be set on refresh error")
	}
}

func TestUpdate_RefreshDone_CursorOutOfBounds(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.listView.cursor = 10

	newResult := &kube.CompositionResult{
		Root:  &kube.ResourceNode{Kind: "XMyApp", Name: "my-app", Status: "Healthy"},
		Stats: model.TreeStats{Total: 1, Healthy: 1},
	}
	updated, _ := m.Update(refreshDoneMsg{result: newResult})
	am := updated.(AppModel)

	// cursor should NOT be preserved since 10 >= len(rows)=1
	if am.listView.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (not preserved when out of bounds)", am.listView.cursor)
	}
}

func TestUpdate_PickedMsg(t *testing.T) {
	m := NewAppPicker(nil)
	m.width = 80
	m.height = 24
	m.state = viewPicker

	updated, cmd := m.Update(pickedMsg{kind: "XMyApp", name: "my-app"})
	am := updated.(AppModel)

	if am.state != viewLoading {
		t.Errorf("state = %d, want viewLoading", am.state)
	}
	if cmd == nil {
		t.Error("should return a cmd for discovering")
	}
}

func TestUpdate_LogMessages(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.state = viewLogs
	m.logView = newLogViewModel(m.tree.Root, 80, 24)

	// logStartedMsg
	updated, _ := m.Update(logStartedMsg{podName: "provider-aws-123"})
	am := updated.(AppModel)
	if am.logView.podName != "provider-aws-123" {
		t.Errorf("podName = %q", am.logView.podName)
	}

	// logLineMsg
	updated, _ = am.Update(logLineMsg("test log line"))
	am = updated.(AppModel)
	if len(am.logView.lines) != 1 {
		t.Errorf("lines = %d, want 1", len(am.logView.lines))
	}

	// logErrMsg
	updated, _ = am.Update(logErrMsg{err: errTest})
	am = updated.(AppModel)
	if am.logView.err == nil {
		t.Error("log err not set")
	}
}

// --- Key handlers: list view ---

func TestHandleListKey_Navigation(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24

	// Down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	am := updated.(AppModel)
	if am.listView.cursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", am.listView.cursor)
	}

	// Down again
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyDown})
	am = updated.(AppModel)
	if am.listView.cursor != 2 {
		t.Errorf("after down×2: cursor = %d, want 2", am.listView.cursor)
	}

	// Down at boundary
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyDown})
	am = updated.(AppModel)
	if am.listView.cursor != 2 {
		t.Errorf("at boundary: cursor = %d, want 2", am.listView.cursor)
	}

	// Up
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyUp})
	am = updated.(AppModel)
	if am.listView.cursor != 1 {
		t.Errorf("after up: cursor = %d, want 1", am.listView.cursor)
	}

	// g = top
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	am = updated.(AppModel)
	if am.listView.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", am.listView.cursor)
	}

	// G = bottom
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	am = updated.(AppModel)
	if am.listView.cursor != 2 {
		t.Errorf("after G: cursor = %d, want 2", am.listView.cursor)
	}
}

func TestHandleListKey_Enter(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am := updated.(AppModel)

	if am.state != viewDetail {
		t.Errorf("state = %d, want viewDetail", am.state)
	}
	if am.detailView.node == nil {
		t.Error("detailView.node is nil")
	}
	if am.detailView.node.Kind != "XMyApp" {
		t.Errorf("detailView.node.Kind = %q, want %q", am.detailView.node.Kind, "XMyApp")
	}
}

func TestHandleListKey_Quit(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("quit should return a cmd")
	}
}

func TestHandleListKey_Logs(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	am := updated.(AppModel)
	if am.state != viewLogs {
		t.Errorf("state = %d, want viewLogs", am.state)
	}
}

func TestHandleListKey_Refresh(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Error("refresh should return a cmd")
	}
}

// --- Key handlers: detail view ---

func TestHandleDetailKey_Back(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24
	m.state = viewDetail
	m.detailView = newDetailViewModel(m.tree.Root, 80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	am := updated.(AppModel)
	if am.state != viewList {
		t.Errorf("state = %d, want viewList after Esc", am.state)
	}
}

func TestHandleDetailKey_Quit(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24
	m.state = viewDetail
	m.detailView = newDetailViewModel(m.tree.Root, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("quit from detail should return a cmd")
	}
}

func TestHandleDetailKey_Logs(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24
	m.state = viewDetail
	m.detailView = newDetailViewModel(m.tree.Root, 80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	am := updated.(AppModel)
	if am.state != viewLogs {
		t.Errorf("state = %d, want viewLogs", am.state)
	}
}

// --- Key handlers: log view ---

func TestHandleLogKey_Quit(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24
	m.state = viewLogs
	m.logView = newLogViewModel(m.tree.Root, 80, 24)
	cancelled := false
	m.logCancel = func() { cancelled = true }

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("quit from logs should return a cmd")
	}
	if !cancelled {
		t.Error("logCancel not called on quit")
	}
}

func TestHandleLogKey_Back(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24
	m.state = viewLogs
	m.logView = newLogViewModel(m.tree.Root, 80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	am := updated.(AppModel)
	if am.state != viewList {
		t.Errorf("state = %d, want viewList", am.state)
	}
}

func TestHandleLogKey_Bottom(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24
	m.state = viewLogs
	m.logView = newLogViewModel(m.tree.Root, 80, 24)
	m.logView.autoScroll = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	am := updated.(AppModel)
	if !am.logView.autoScroll {
		t.Error("G should enable autoScroll")
	}
}

// --- Key handlers: picker view ---

func TestHandlePickerKey_Quit(t *testing.T) {
	items := []kube.CompositeItem{
		{Kind: "XMyApp", Name: "app-1", Status: "Healthy"},
	}
	m := NewAppPicker(nil)
	m.width = 80
	m.height = 24
	m.state = viewPicker
	m.pickerView = newPickerViewModel(items, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("quit from picker should return a cmd")
	}
}

func TestHandlePickerKey_Enter(t *testing.T) {
	items := []kube.CompositeItem{
		{Kind: "XMyApp", Name: "app-1", Status: "Healthy"},
	}
	m := NewAppPicker(nil)
	m.width = 80
	m.height = 24
	m.state = viewPicker
	m.pickerView = newPickerViewModel(items, 80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter in picker should return a cmd")
	}
}

// --- Key handlers: loading ---

func TestHandleKey_Loading_Quit(t *testing.T) {
	m := NewAppLoading(nil, "XMyApp", "my-app", "")
	m.width = 80

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("quit from loading should return a cmd")
	}
}

// --- Error dismissal ---

func TestUpdate_ErrorDismissal(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.width = 80
	m.height = 24
	m.err = errTest

	// Any keypress should clear the error in non-loading state
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	am := updated.(AppModel)
	if am.err != nil {
		t.Error("err should be cleared on keypress")
	}
}

// --- CancelLogs ---

func TestCancelLogs(t *testing.T) {
	m := NewApp(makeTestResult(), nil)

	cancelled := false
	m.logCancel = func() { cancelled = true }

	m.cancelLogs()
	if !cancelled {
		t.Error("cancel function was not called")
	}
	if m.logCancel != nil {
		t.Error("logCancel should be nil after cancel")
	}

	// Calling again should not panic
	m.cancelLogs()
}

// --- refreshTree / startLogStream cmd execution ---

func TestRefreshTree_NilTree(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.tree = nil

	cmd := m.refreshTree()
	if cmd == nil {
		t.Fatal("refreshTree should return a cmd")
	}

	msg := cmd()
	rdm, ok := msg.(refreshDoneMsg)
	if !ok {
		t.Fatalf("expected refreshDoneMsg, got %T", msg)
	}
	if rdm.err == nil {
		t.Error("refreshTree with nil tree should produce error")
	}
}

func TestRefreshTree_NilRoot(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	m.tree = &kube.CompositionResult{}

	cmd := m.refreshTree()
	msg := cmd()
	rdm, ok := msg.(refreshDoneMsg)
	if !ok {
		t.Fatalf("expected refreshDoneMsg, got %T", msg)
	}
	if rdm.err == nil {
		t.Error("refreshTree with nil root should produce error")
	}
}

func TestStartLogStream_NilNode(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	cmd := m.startLogStream(nil)

	msg := cmd()
	lem, ok := msg.(logErrMsg)
	if !ok {
		t.Fatalf("expected logErrMsg, got %T", msg)
	}
	if lem.err == nil {
		t.Error("startLogStream with nil node should produce error")
	}
}

func TestStartLogStream_NilResource(t *testing.T) {
	m := NewApp(makeTestResult(), nil)
	node := &kube.ResourceNode{Kind: "Bucket", Name: "test"}
	cmd := m.startLogStream(node)

	msg := cmd()
	lem, ok := msg.(logErrMsg)
	if !ok {
		t.Fatalf("expected logErrMsg, got %T", msg)
	}
	if lem.err == nil {
		t.Error("startLogStream with nil resource should produce error")
	}
}

// --- Helpers ---

var errTest = testError("test error")

type testError string

func (e testError) Error() string { return string(e) }
