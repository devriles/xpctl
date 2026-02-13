package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
)

type pickerItem struct {
	item kube.CompositeItem
}

func (i pickerItem) Title() string {
	return fmt.Sprintf("%s/%s", i.item.Kind, i.item.Name)
}

func (i pickerItem) Description() string {
	var parts []string
	if i.item.Namespace != "" {
		parts = append(parts, fmt.Sprintf("ns: %s", i.item.Namespace))
	}
	parts = append(parts, fmt.Sprintf("status: %s", i.item.Status))
	return strings.Join(parts, "  ")
}

func (i pickerItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s", i.item.Kind, i.item.Name, i.item.Namespace)
}

type pickerItemDelegate struct{}

func (d pickerItemDelegate) Height() int                             { return 2 }
func (d pickerItemDelegate) Spacing() int                            { return 0 }
func (d pickerItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d pickerItemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	pi, ok := item.(pickerItem)
	if !ok {
		return
	}

	icon, style := statusIconAndStyle(model.StatusSummary(pi.item.Status))
	title := fmt.Sprintf("%s %s", icon, pi.Title())
	desc := pi.Description()

	if index == m.Index() {
		title = StyleSelected.Render(style.Render(title))
		desc = StyleSelected.Render(StyleMuted.Render("  " + desc))
	} else {
		title = style.Render(title)
		desc = StyleMuted.Render("  " + desc)
	}

	fmt.Fprintf(w, "  %s\n%s", title, desc)
}

type pickerViewModel struct {
	list  list.Model
	items []kube.CompositeItem
}

func newPickerViewModel(items []kube.CompositeItem, width, height int) pickerViewModel {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = pickerItem{item: item}
	}

	delegate := pickerItemDelegate{}
	l := list.New(listItems, delegate, width, height-2)
	l.Title = "Select a Crossplane Resource"
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorAccent).
		MarginLeft(2)
	l.SetFilteringEnabled(true)
	l.SetShowStatusBar(true)
	l.SetShowHelp(true)

	return pickerViewModel{
		list:  l,
		items: items,
	}
}

func (m pickerViewModel) selectedItem() *kube.CompositeItem {
	item, ok := m.list.SelectedItem().(pickerItem)
	if !ok {
		return nil
	}
	return &item.item
}

func (m pickerViewModel) render(width, height int) string {
	return m.list.View()
}
