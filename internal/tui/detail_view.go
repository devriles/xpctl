package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/devriles/xpctl/internal/kube"
)

type detailViewModel struct {
	node     *kube.ResourceNode
	viewport viewport.Model
	ready    bool
}

func newDetailViewModel(node *kube.ResourceNode, width, height int) detailViewModel {
	vp := viewport.New(width, height-4)
	vp.SetContent(renderDetail(node, width))
	return detailViewModel{
		node:     node,
		viewport: vp,
		ready:    true,
	}
}

func renderDetail(node *kube.ResourceNode, width int) string {
	if node == nil || node.Resource == nil {
		return "No resource data available."
	}
	res := node.Resource
	var b strings.Builder

	b.WriteString(StyleLabel.Render("Kind:        ") + StyleValue.Render(res.Kind) + "\n")
	b.WriteString(StyleLabel.Render("Name:        ") + StyleValue.Render(res.Name) + "\n")
	b.WriteString(StyleLabel.Render("API Version: ") + StyleValue.Render(res.APIVersion) + "\n")
	if res.Namespace != "" {
		b.WriteString(StyleLabel.Render("Namespace:   ") + StyleValue.Render(res.Namespace) + "\n")
	}
	if res.ExternalName != "" {
		b.WriteString(StyleLabel.Render("External:    ") + StyleValue.Render(res.ExternalName) + "\n")
	}
	if res.Age > 0 {
		b.WriteString(StyleLabel.Render("Age:         ") + StyleValue.Render(formatAgeLong(res.Age)) + "\n")
	}
	b.WriteString(StyleLabel.Render("Status:      "))
	icon, style := statusIconAndStyle(res.Status)
	b.WriteString(style.Render(icon+" "+string(res.Status)) + "\n")

	b.WriteString("\n")
	b.WriteString(StyleTitle.Render("Conditions") + "\n")
	b.WriteString(StyleMuted.Render(strings.Repeat("─", min(width, 80))) + "\n")

	if len(res.Conditions) == 0 {
		b.WriteString(StyleMuted.Render("  No conditions reported.") + "\n")
	} else {
		b.WriteString(fmt.Sprintf("  %-12s %-8s %-20s %s\n",
			StyleLabel.Render("TYPE"),
			StyleLabel.Render("STATUS"),
			StyleLabel.Render("REASON"),
			StyleLabel.Render("MESSAGE"),
		))
		for _, c := range res.Conditions {
			statusStyle := StyleUnknown
			switch c.Status {
			case "True":
				statusStyle = StyleHealthy
			case "False":
				statusStyle = StyleError
			}
			msg := c.Message
			if len(msg) > 60 {
				msg = msg[:57] + "..."
			}
			b.WriteString(fmt.Sprintf("  %-12s %s %-20s %s\n",
				c.Type,
				statusStyle.Render(fmt.Sprintf("%-8s", c.Status)),
				c.Reason,
				StyleMuted.Render(msg),
			))
		}
	}

	if res.Raw != nil {
		annotations := res.Raw.GetAnnotations()
		xpAnnotations := filterCrossplaneAnnotations(annotations)
		if len(xpAnnotations) > 0 {
			b.WriteString("\n")
			b.WriteString(StyleTitle.Render("Crossplane Annotations") + "\n")
			b.WriteString(StyleMuted.Render(strings.Repeat("─", min(width, 80))) + "\n")
			for k, v := range xpAnnotations {
				b.WriteString(fmt.Sprintf("  %s: %s\n", StyleLabel.Render(k), StyleMuted.Render(v)))
			}
		}
	}

	return b.String()
}

func filterCrossplaneAnnotations(annotations map[string]string) map[string]string {
	filtered := make(map[string]string)
	for k, v := range annotations {
		if strings.HasPrefix(k, "crossplane.io/") || strings.HasPrefix(k, "upbound.io/") {
			filtered[k] = v
		}
	}
	return filtered
}

func formatAgeLong(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	case d < time.Hour:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", m, s)
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		return fmt.Sprintf("%dd%dh", days, h)
	}
}

func (m detailViewModel) renderFooter(width int) string {
	help := StyleMuted.Render("esc:back  j/k:scroll  l:logs  q:quit")
	return help + strings.Repeat(" ", max(0, width-lipgloss.Width(help)))
}
