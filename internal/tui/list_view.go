package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
)

type listRow struct {
	node   *kube.ResourceNode
	prefix string
	depth  int
}

type listViewModel struct {
	rows     []listRow
	cursor   int
	offset   int
	stats    model.TreeStats
	rootKind string
	rootName string
}

func newListViewModel(result *kube.CompositionResult) listViewModel {
	var rows []listRow
	if result.Root != nil {
		buildRows(result.Root, "", true, 0, &rows)
	}
	rootKind, rootName := "", ""
	if result.Root != nil {
		rootKind = result.Root.Kind
		rootName = result.Root.Name
	}
	return listViewModel{
		rows:     rows,
		stats:    result.Stats,
		rootKind: rootKind,
		rootName: rootName,
	}
}

func buildRows(node *kube.ResourceNode, prefix string, isLast bool, depth int, rows *[]listRow) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	displayPrefix := ""
	if depth > 0 {
		displayPrefix = prefix + connector
	}

	*rows = append(*rows, listRow{
		node:   node,
		prefix: displayPrefix,
		depth:  depth,
	})

	childPrefix := prefix
	if depth > 0 {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		buildRows(child, childPrefix, i == len(node.Children)-1, depth+1, rows)
	}
}

func (m listViewModel) selectedNode() *kube.ResourceNode {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return m.rows[m.cursor].node
	}
	return nil
}

func (m listViewModel) render(width, height int) string {
	if len(m.rows) == 0 {
		return StyleMuted.Render("No resources found.")
	}

	headerHeight := 2
	statusBarHeight := 2
	visibleHeight := height - headerHeight - statusBarHeight
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}

	var b strings.Builder

	header := StyleTitle.Render(fmt.Sprintf(" xpctl: %s/%s", m.rootKind, m.rootName))
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(StyleMuted.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	end := m.offset + visibleHeight
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.offset; i < end; i++ {
		row := m.rows[i]
		line := renderRow(row, i == m.cursor, width)
		b.WriteString(line)
		b.WriteString("\n")
	}

	rendered := end - m.offset
	for i := rendered; i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	b.WriteString(StyleMuted.Render(strings.Repeat("─", width)))
	b.WriteString("\n")
	b.WriteString(renderStatusBar(m.stats, width))

	return b.String()
}

func renderRow(row listRow, selected bool, width int) string {
	node := row.node
	icon, style := statusIconAndStyle(model.StatusSummary(node.Status))

	age := ""
	if node.Resource != nil && node.Resource.Age > 0 {
		age = model.FormatAge(node.Resource.Age)
	}

	kindName := fmt.Sprintf("%s/%s", node.Kind, node.Name)
	ageSuffix := ""
	if age != "" {
		ageSuffix = StyleMuted.Render(fmt.Sprintf("  %s", age))
	}

	if selected {
		line := fmt.Sprintf("%s%s %s%s", row.prefix, icon, kindName, ageSuffix)
		padded := line + strings.Repeat(" ", max(0, width-lipgloss.Width(line)))
		return StyleSelected.Render(padded)
	}

	return style.Render(fmt.Sprintf("%s%s", row.prefix, icon)) +
		" " + kindName + ageSuffix
}

func statusIconAndStyle(status model.StatusSummary) (string, lipgloss.Style) {
	switch status {
	case model.StatusHealthy:
		return IconHealthy, StyleHealthy
	case model.StatusError:
		return IconError, StyleError
	case model.StatusProgressing:
		return IconProgressing, StyleProgressing
	default:
		return IconUnknown, StyleUnknown
	}
}

func renderStatusBar(stats model.TreeStats, width int) string {
	parts := []string{
		StyleHealthy.Render(fmt.Sprintf("%d/%d Ready", stats.Healthy, stats.Total)),
	}
	if stats.Error > 0 {
		parts = append(parts, StyleError.Render(fmt.Sprintf("%d Error", stats.Error)))
	}
	if stats.Progressing > 0 {
		parts = append(parts, StyleProgressing.Render(fmt.Sprintf("%d Progressing", stats.Progressing)))
	}
	if stats.Unknown > 0 {
		parts = append(parts, StyleUnknown.Render(fmt.Sprintf("%d Unknown", stats.Unknown)))
	}

	status := strings.Join(parts, StyleMuted.Render(" │ "))
	help := StyleMuted.Render("↑↓:navigate  enter:detail  l:logs  r:refresh  q:quit")

	return status + strings.Repeat(" ", max(0, width-lipgloss.Width(status)-lipgloss.Width(help))) + help
}
