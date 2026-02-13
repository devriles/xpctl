package tui

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/devriles/xpctl/internal/kube"
)

type viewState int

const (
	viewLoading viewState = iota
	viewPicker
	viewList
	viewDetail
	viewLogs
)

type logLineMsg string
type logErrMsg struct{ err error }
type logDoneMsg struct{}

type logStartedMsg struct {
	podName string
	cancel  context.CancelFunc
	ch      <-chan tea.Msg
}

type refreshDoneMsg struct {
	result *kube.CompositionResult
	err    error
}

type discoverDoneMsg struct {
	result *kube.CompositionResult
	err    error
}

type compositeListMsg struct {
	items []kube.CompositeItem
	err   error
}

type pickedMsg struct {
	kind      string
	name      string
	namespace string
}

type AppModel struct {
	state      viewState
	keys       KeyMap
	spinner    spinner.Model
	listView   listViewModel
	detailView detailViewModel
	logView    logViewModel
	pickerView pickerViewModel
	tree       *kube.CompositionResult
	client     *kube.Client
	width      int
	height     int
	err        error
	loadingMsg string
	logCancel  context.CancelFunc
	logCh      <-chan tea.Msg
}

func NewApp(result *kube.CompositionResult, client *kube.Client) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return AppModel{
		state:    viewList,
		keys:     DefaultKeyMap(),
		spinner:  s,
		listView: newListViewModel(result),
		tree:     result,
		client:   client,
	}
}

func NewAppLoading(client *kube.Client, kind, name, namespace string) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return AppModel{
		state:      viewLoading,
		keys:       DefaultKeyMap(),
		spinner:    s,
		client:     client,
		loadingMsg: fmt.Sprintf("Discovering %s/%s...", kind, name),
	}
}

func NewAppPicker(client *kube.Client) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return AppModel{
		state:      viewLoading,
		keys:       DefaultKeyMap(),
		spinner:    s,
		client:     client,
		loadingMsg: "Scanning cluster for Crossplane resources...",
	}
}

func Run(result *kube.CompositionResult, client *kube.Client) error {
	m := NewApp(result, client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func RunWithDiscovery(client *kube.Client, kind, name, namespace string) error {
	m := NewAppLoading(client, kind, name, namespace)
	p := tea.NewProgram(m, tea.WithAltScreen())
	go func() {
		result, err := kube.Discover(context.Background(), client, kind, name, namespace)
		p.Send(discoverDoneMsg{result: result, err: err})
	}()
	_, err := p.Run()
	return err
}

func RunPicker(client *kube.Client) error {
	m := NewAppPicker(client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	go func() {
		items, err := kube.ListComposites(context.Background(), client)
		p.Send(compositeListMsg{items: items, err: err})
	}()
	_, err := p.Run()
	return err
}

func (m AppModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.detailView.ready {
			m.detailView.viewport.Width = msg.Width
			m.detailView.viewport.Height = msg.Height - 4
		}
		if m.logView.ready {
			m.logView.viewport.Width = msg.Width
			m.logView.viewport.Height = msg.Height - 4
		}
		if m.state == viewPicker {
			m.pickerView.list.SetSize(msg.Width, msg.Height-2)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case discoverDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = viewList
			return m, nil
		}
		m.tree = msg.result
		m.listView = newListViewModel(msg.result)
		m.state = viewList
		return m, nil

	case compositeListMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if len(msg.items) == 0 {
			m.err = fmt.Errorf("no Crossplane composite resources or claims found in the cluster")
			return m, nil
		}
		m.pickerView = newPickerViewModel(msg.items, m.width, m.height)
		m.state = viewPicker
		return m, nil

	case pickedMsg:
		m.state = viewLoading
		m.loadingMsg = fmt.Sprintf("Discovering %s/%s...", msg.kind, msg.name)
		return m, func() tea.Msg {
			result, err := kube.Discover(context.Background(), m.client, msg.kind, msg.name, msg.namespace)
			return discoverDoneMsg{result: result, err: err}
		}

	case refreshDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.tree = msg.result
		cursor := m.listView.cursor
		m.listView = newListViewModel(msg.result)
		if cursor < len(m.listView.rows) {
			m.listView.cursor = cursor
		}
		m.err = nil
		return m, nil

	case logStartedMsg:
		m.logView.podName = msg.podName
		m.logCancel = msg.cancel
		m.logCh = msg.ch
		return m, readLogCh(msg.ch)

	case logLineMsg:
		m.logView.appendLine(string(msg))
		if m.logCh != nil {
			return m, readLogCh(m.logCh)
		}
		return m, nil

	case logErrMsg:
		m.logView.err = msg.err
		return m, nil

	case logDoneMsg:
		return m, nil

	case tea.KeyMsg:
		if m.err != nil && m.state != viewLoading {
			m.err = nil
		}
		return m.handleKey(msg)
	}

	switch m.state {
	case viewDetail:
		var cmd tea.Cmd
		m.detailView.viewport, cmd = m.detailView.viewport.Update(msg)
		return m, cmd
	case viewLogs:
		var cmd tea.Cmd
		m.logView.viewport, cmd = m.logView.viewport.Update(msg)
		return m, cmd
	case viewPicker:
		return m.updatePicker(msg)
	}

	return m, nil
}

func readLogCh(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return logDoneMsg{}
		}
		return msg
	}
}

func (m AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case viewLoading:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
	case viewPicker:
		return m.handlePickerKey(msg)
	case viewList:
		return m.handleListKey(msg)
	case viewDetail:
		return m.handleDetailKey(msg)
	case viewLogs:
		return m.handleLogKey(msg)
	}
	return m, nil
}

func (m AppModel) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.cancelLogs()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Up):
		if m.listView.cursor > 0 {
			m.listView.cursor--
		}

	case key.Matches(msg, m.keys.Down):
		if m.listView.cursor < len(m.listView.rows)-1 {
			m.listView.cursor++
		}

	case key.Matches(msg, m.keys.Top):
		m.listView.cursor = 0

	case key.Matches(msg, m.keys.Bottom):
		if len(m.listView.rows) > 0 {
			m.listView.cursor = len(m.listView.rows) - 1
		}

	case key.Matches(msg, m.keys.Enter):
		node := m.listView.selectedNode()
		if node != nil {
			m.detailView = newDetailViewModel(node, m.width, m.height)
			m.state = viewDetail
		}

	case key.Matches(msg, m.keys.Logs):
		node := m.listView.selectedNode()
		if node != nil {
			m.state = viewLogs
			m.logView = newLogViewModel(node, m.width, m.height)
			return m, m.startLogStream(node)
		}

	case key.Matches(msg, m.keys.Refresh):
		return m, m.refreshTree()
	}

	return m, nil
}

func (m AppModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.cancelLogs()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.state = viewList
		return m, nil

	case key.Matches(msg, m.keys.Logs):
		if m.detailView.node != nil {
			m.state = viewLogs
			m.logView = newLogViewModel(m.detailView.node, m.width, m.height)
			return m, m.startLogStream(m.detailView.node)
		}

	default:
		var cmd tea.Cmd
		m.detailView.viewport, cmd = m.detailView.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m AppModel) handleLogKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		m.cancelLogs()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		m.cancelLogs()
		m.state = viewList
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.logView.autoScroll = true
		m.logView.viewport.GotoBottom()

	default:
		prevOffset := m.logView.viewport.YOffset
		var cmd tea.Cmd
		m.logView.viewport, cmd = m.logView.viewport.Update(msg)
		if m.logView.viewport.YOffset != prevOffset {
			m.logView.autoScroll = false
		}
		return m, cmd
	}

	return m, nil
}

func (m AppModel) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, m.keys.Enter) {
		item := m.pickerView.selectedItem()
		if item != nil {
			return m, func() tea.Msg {
				return pickedMsg{
					kind:      item.Kind,
					name:      item.Name,
					namespace: item.Namespace,
				}
			}
		}
	}
	return m.updatePicker(msg)
}

func (m AppModel) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.pickerView.list, cmd = m.pickerView.list.Update(msg)
	return m, cmd
}

func (m AppModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.state == viewLoading {
		return fmt.Sprintf("\n  %s %s\n\n  %s",
			m.spinner.View(),
			StyleTitle.Render(m.loadingMsg),
			StyleMuted.Render("Press q to quit"),
		)
	}

	if m.err != nil && m.state != viewList {
		return StyleError.Render(fmt.Sprintf("\n  Error: %v\n\n", m.err)) +
			StyleMuted.Render("  Press q to quit")
	}

	switch m.state {
	case viewPicker:
		return m.pickerView.render(m.width, m.height)
	case viewList:
		view := m.listView.render(m.width, m.height)
		if m.err != nil {
			view += "\n" + StyleError.Render(fmt.Sprintf("  Error: %v (press any key to dismiss)", m.err))
		}
		return view
	case viewDetail:
		var b strings.Builder
		title := StyleTitle.Render(fmt.Sprintf(" Detail: %s/%s", m.detailView.node.Kind, m.detailView.node.Name))
		b.WriteString(title + "\n")
		b.WriteString(StyleMuted.Render(strings.Repeat("─", m.width)) + "\n")
		b.WriteString(m.detailView.viewport.View())
		b.WriteString("\n")
		b.WriteString(StyleMuted.Render(strings.Repeat("─", m.width)) + "\n")
		b.WriteString(m.detailView.renderFooter(m.width))
		return b.String()
	case viewLogs:
		return m.logView.render(m.width, m.height)
	}

	return ""
}

func (m *AppModel) cancelLogs() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	m.logCh = nil
}

func (m AppModel) refreshTree() tea.Cmd {
	return func() tea.Msg {
		if m.tree == nil || m.tree.Root == nil {
			return refreshDoneMsg{err: fmt.Errorf("no tree to refresh")}
		}
		root := m.tree.Root
		result, err := kube.Discover(context.Background(), m.client, root.Kind, root.Name, root.Namespace)
		return refreshDoneMsg{result: result, err: err}
	}
}

func (m AppModel) startLogStream(node *kube.ResourceNode) tea.Cmd {
	return func() tea.Msg {
		if node == nil || node.Resource == nil {
			return logErrMsg{err: fmt.Errorf("no resource selected")}
		}

		ctx, cancel := context.WithCancel(context.Background())
		res := node.Resource

		podName, containerName, err := kube.FindProviderPod(ctx, m.client, res.APIVersion)
		if err != nil {
			cancel()
			return logErrMsg{err: fmt.Errorf("finding provider pod: %w", err)}
		}

		reader, err := kube.StreamLogs(ctx, m.client, podName, containerName, res.Name)
		if err != nil {
			cancel()
			return logErrMsg{err: fmt.Errorf("streaming logs: %w", err)}
		}

		ch := make(chan tea.Msg, 100)
		go func() {
			defer close(ch)
			defer reader.Close()
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				select {
				case ch <- logLineMsg(scanner.Text()):
				case <-ctx.Done():
					return
				}
			}
			if err := scanner.Err(); err != nil && ctx.Err() == nil {
				ch <- logErrMsg{err: err}
			}
		}()

		return logStartedMsg{podName: podName, cancel: cancel, ch: ch}
	}
}
