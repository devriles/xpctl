package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/devriles/xpctl/internal/kube"
)

type logViewModel struct {
	node       *kube.ResourceNode
	viewport   viewport.Model
	lines      []string
	autoScroll bool
	podName    string
	ready      bool
	err        error
}

func newLogViewModel(node *kube.ResourceNode, width, height int) logViewModel {
	vp := viewport.New(width, height-4)
	return logViewModel{
		node:       node,
		viewport:   vp,
		autoScroll: true,
		ready:      true,
	}
}

func (m *logViewModel) appendLine(line string) {
	m.lines = append(m.lines, line)
	content := strings.Join(m.lines, "\n")
	m.viewport.SetContent(content)
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

func (m logViewModel) render(width, height int) string {
	var b strings.Builder

	title := "Provider Logs"
	if m.node != nil {
		title = fmt.Sprintf("Logs: %s/%s", m.node.Kind, m.node.Name)
	}
	if m.podName != "" {
		title += StyleMuted.Render(fmt.Sprintf(" (pod: %s)", m.podName))
	}
	b.WriteString(StyleTitle.Render(title) + "\n")
	b.WriteString(StyleMuted.Render(strings.Repeat("─", width)) + "\n")

	if m.err != nil {
		b.WriteString(StyleError.Render(fmt.Sprintf("Error: %v", m.err)) + "\n")
	} else if len(m.lines) == 0 {
		b.WriteString(StyleMuted.Render("Waiting for logs...") + "\n")
	} else {
		b.WriteString(m.viewport.View())
	}

	b.WriteString("\n")
	b.WriteString(StyleMuted.Render(strings.Repeat("─", width)) + "\n")
	scrollStatus := ""
	if !m.autoScroll {
		scrollStatus = StyleProgressing.Render(" [PAUSED] ") + " "
	}
	help := scrollStatus + StyleMuted.Render("esc:back  G:bottom  q:quit")
	b.WriteString(help)

	return b.String()
}
