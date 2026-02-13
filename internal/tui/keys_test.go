package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	bindings := []struct {
		name    string
		binding key.Binding
	}{
		{"Up", km.Up},
		{"Down", km.Down},
		{"Enter", km.Enter},
		{"Back", km.Back},
		{"Logs", km.Logs},
		{"Events", km.Events},
		{"Refresh", km.Refresh},
		{"Quit", km.Quit},
		{"Help", km.Help},
		{"Bottom", km.Bottom},
		{"Top", km.Top},
	}

	for _, b := range bindings {
		t.Run(b.name, func(t *testing.T) {
			if !b.binding.Enabled() {
				t.Errorf("%s binding is not enabled", b.name)
			}
			help := b.binding.Help()
			if help.Key == "" {
				t.Errorf("%s has empty help key", b.name)
			}
			if help.Desc == "" {
				t.Errorf("%s has empty help desc", b.name)
			}
		})
	}
}
