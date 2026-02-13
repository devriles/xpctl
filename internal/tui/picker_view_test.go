package tui

import (
	"strings"
	"testing"

	"github.com/devriles/xpctl/internal/kube"
)

func TestPickerItem_Title(t *testing.T) {
	item := pickerItem{item: kube.CompositeItem{Kind: "XMyApp", Name: "my-app"}}
	if item.Title() != "XMyApp/my-app" {
		t.Errorf("Title() = %q, want %q", item.Title(), "XMyApp/my-app")
	}
}

func TestPickerItem_Description(t *testing.T) {
	tests := []struct {
		name string
		item kube.CompositeItem
		want string
	}{
		{
			name: "with namespace",
			item: kube.CompositeItem{Kind: "MyApp", Namespace: "default", Status: "Healthy"},
			want: "ns: default",
		},
		{
			name: "without namespace",
			item: kube.CompositeItem{Kind: "XMyApp", Status: "Error"},
			want: "status: Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pi := pickerItem{item: tt.item}
			desc := pi.Description()
			if !strings.Contains(desc, tt.want) {
				t.Errorf("Description() = %q, missing %q", desc, tt.want)
			}
		})
	}
}

func TestPickerItem_FilterValue(t *testing.T) {
	item := pickerItem{item: kube.CompositeItem{Kind: "XMyApp", Name: "my-app", Namespace: "prod"}}
	fv := item.FilterValue()
	if !strings.Contains(fv, "XMyApp") {
		t.Error("FilterValue should contain kind")
	}
	if !strings.Contains(fv, "my-app") {
		t.Error("FilterValue should contain name")
	}
	if !strings.Contains(fv, "prod") {
		t.Error("FilterValue should contain namespace")
	}
}

func TestPickerItemDelegate(t *testing.T) {
	d := pickerItemDelegate{}
	if d.Height() != 2 {
		t.Errorf("Height() = %d, want 2", d.Height())
	}
	if d.Spacing() != 0 {
		t.Errorf("Spacing() = %d, want 0", d.Spacing())
	}
	cmd := d.Update(nil, nil)
	if cmd != nil {
		t.Error("Update should return nil")
	}
}

func TestNewPickerViewModel(t *testing.T) {
	items := []kube.CompositeItem{
		{Kind: "XMyApp", Name: "app-1", Status: "Healthy"},
		{Kind: "XMyApp", Name: "app-2", Status: "Error"},
		{Kind: "MyAppClaim", Name: "claim-1", Namespace: "default", Status: "Healthy"},
	}

	m := newPickerViewModel(items, 80, 24)

	if len(m.items) != 3 {
		t.Errorf("items = %d, want 3", len(m.items))
	}
}

func TestSelectedItem(t *testing.T) {
	items := []kube.CompositeItem{
		{Kind: "XMyApp", Name: "app-1", Status: "Healthy"},
	}
	m := newPickerViewModel(items, 80, 24)

	item := m.selectedItem()
	if item == nil {
		t.Fatal("selectedItem() = nil")
	}
	if item.Name != "app-1" {
		t.Errorf("selectedItem().Name = %q, want %q", item.Name, "app-1")
	}
}

func TestSelectedItem_Empty(t *testing.T) {
	m := newPickerViewModel(nil, 80, 24)
	item := m.selectedItem()
	if item != nil {
		t.Errorf("selectedItem() = %v, want nil for empty list", item)
	}
}
