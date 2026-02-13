package kube

import (
	"testing"
	"time"

	"github.com/devriles/xpctl/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseConditions(t *testing.T) {
	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		wantLen  int
		wantType string // check first condition type if wantLen > 0
	}{
		{
			name:    "no status at all",
			obj:     &unstructured.Unstructured{Object: map[string]interface{}{}},
			wantLen: 0,
		},
		{
			name: "empty status",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{},
			}},
			wantLen: 0,
		},
		{
			name: "empty conditions array",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{},
				},
			}},
			wantLen: 0,
		},
		{
			name: "single Synced condition",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Synced",
							"status": "True",
							"reason": "ReconcileSuccess",
						},
					},
				},
			}},
			wantLen:  1,
			wantType: "Synced",
		},
		{
			name: "Synced and Ready conditions",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Synced",
							"status": "True",
							"reason": "ReconcileSuccess",
						},
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
							"reason": "Available",
						},
					},
				},
			}},
			wantLen:  2,
			wantType: "Synced",
		},
		{
			name: "condition with valid timestamp",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":               "Ready",
							"status":             "True",
							"lastTransitionTime": "2025-01-15T10:30:00Z",
						},
					},
				},
			}},
			wantLen:  1,
			wantType: "Ready",
		},
		{
			name: "condition with invalid timestamp",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":               "Ready",
							"status":             "False",
							"lastTransitionTime": "not-a-timestamp",
						},
					},
				},
			}},
			wantLen:  1,
			wantType: "Ready",
		},
		{
			name: "condition with message",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":    "Synced",
							"status":  "False",
							"reason":  "ReconcileError",
							"message": "cannot create external resource: access denied",
						},
					},
				},
			}},
			wantLen:  1,
			wantType: "Synced",
		},
		{
			name: "malformed condition entry (not a map)",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						"not-a-map",
						map[string]interface{}{
							"type":   "Synced",
							"status": "True",
						},
					},
				},
			}},
			wantLen:  1,
			wantType: "Synced",
		},
		{
			name: "condition with missing fields",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type": "Synced",
							// no status, reason, or message
						},
					},
				},
			}},
			wantLen:  1,
			wantType: "Synced",
		},
		{
			name: "conditions field is not a slice",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": "not-a-slice",
				},
			}},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseConditions(tt.obj)
			if len(got) != tt.wantLen {
				t.Errorf("ParseConditions() returned %d conditions, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen > 0 && len(got) > 0 && got[0].Type != tt.wantType {
				t.Errorf("first condition type = %q, want %q", got[0].Type, tt.wantType)
			}
		})
	}
}

func TestParseConditions_FieldValues(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":               "Synced",
					"status":             "False",
					"reason":             "ReconcileError",
					"message":            "some error message",
					"lastTransitionTime": "2025-06-15T12:00:00Z",
				},
			},
		},
	}}

	conditions := ParseConditions(obj)
	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}

	c := conditions[0]
	if c.Type != "Synced" {
		t.Errorf("Type = %q, want %q", c.Type, "Synced")
	}
	if c.Status != "False" {
		t.Errorf("Status = %q, want %q", c.Status, "False")
	}
	if c.Reason != "ReconcileError" {
		t.Errorf("Reason = %q, want %q", c.Reason, "ReconcileError")
	}
	if c.Message != "some error message" {
		t.Errorf("Message = %q, want %q", c.Message, "some error message")
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2025-06-15T12:00:00Z")
	if !c.LastTransitionTime.Equal(expectedTime) {
		t.Errorf("LastTransitionTime = %v, want %v", c.LastTransitionTime, expectedTime)
	}
}

func TestFindCondition(t *testing.T) {
	conditions := []model.Condition{
		{Type: "Synced", Status: "True"},
		{Type: "Ready", Status: "False", Reason: "Creating"},
		{Type: "Healthy", Status: "Unknown"},
	}

	tests := []struct {
		name     string
		condType string
		wantNil  bool
		wantType string
	}{
		{"find Synced", "Synced", false, "Synced"},
		{"find Ready", "Ready", false, "Ready"},
		{"find Healthy", "Healthy", false, "Healthy"},
		{"not found", "NonExistent", true, ""},
		{"empty type", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindCondition(conditions, tt.condType)
			if tt.wantNil && got != nil {
				t.Errorf("FindCondition() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Error("FindCondition() = nil, want non-nil")
			}
			if !tt.wantNil && got != nil && got.Type != tt.wantType {
				t.Errorf("FindCondition().Type = %q, want %q", got.Type, tt.wantType)
			}
		})
	}

	// Test with nil/empty slice
	t.Run("nil conditions", func(t *testing.T) {
		if got := FindCondition(nil, "Synced"); got != nil {
			t.Errorf("FindCondition(nil) = %v, want nil", got)
		}
	})
	t.Run("empty conditions", func(t *testing.T) {
		if got := FindCondition([]model.Condition{}, "Synced"); got != nil {
			t.Errorf("FindCondition([]) = %v, want nil", got)
		}
	})
}

func TestDeriveStatus(t *testing.T) {
	tests := []struct {
		name       string
		conditions []model.Condition
		want       model.StatusSummary
	}{
		{
			name:       "no conditions → Unknown",
			conditions: nil,
			want:       model.StatusUnknown,
		},
		{
			name:       "empty conditions → Unknown",
			conditions: []model.Condition{},
			want:       model.StatusUnknown,
		},
		{
			name: "Synced=True, Ready=True → Healthy",
			conditions: []model.Condition{
				{Type: "Synced", Status: "True"},
				{Type: "Ready", Status: "True"},
			},
			want: model.StatusHealthy,
		},
		{
			name: "Synced=False → Error (regardless of Ready)",
			conditions: []model.Condition{
				{Type: "Synced", Status: "False", Reason: "ReconcileError"},
				{Type: "Ready", Status: "True"},
			},
			want: model.StatusError,
		},
		{
			name: "Synced=False only → Error",
			conditions: []model.Condition{
				{Type: "Synced", Status: "False"},
			},
			want: model.StatusError,
		},
		{
			name: "Synced=True, Ready=False, Reason=Creating → Progressing",
			conditions: []model.Condition{
				{Type: "Synced", Status: "True"},
				{Type: "Ready", Status: "False", Reason: "Creating"},
			},
			want: model.StatusProgressing,
		},
		{
			name: "Synced=True, Ready=False, Reason=Pending → Progressing",
			conditions: []model.Condition{
				{Type: "Synced", Status: "True"},
				{Type: "Ready", Status: "False", Reason: "Pending"},
			},
			want: model.StatusProgressing,
		},
		{
			name: "Synced=True, Ready=False, Reason=ReconcileError → Error",
			conditions: []model.Condition{
				{Type: "Synced", Status: "True"},
				{Type: "Ready", Status: "False", Reason: "ReconcileError"},
			},
			want: model.StatusError,
		},
		{
			name: "Synced=True, Ready=False, Reason=Unavailable → Error",
			conditions: []model.Condition{
				{Type: "Synced", Status: "True"},
				{Type: "Ready", Status: "False", Reason: "Unavailable"},
			},
			want: model.StatusError,
		},
		{
			name: "Synced=True, no Ready → Progressing",
			conditions: []model.Condition{
				{Type: "Synced", Status: "True"},
			},
			want: model.StatusProgressing,
		},
		{
			name: "only Ready=True, no Synced → Healthy",
			conditions: []model.Condition{
				{Type: "Ready", Status: "True"},
			},
			want: model.StatusHealthy,
		},
		{
			name: "only Ready=False (Creating), no Synced → Progressing",
			conditions: []model.Condition{
				{Type: "Ready", Status: "False", Reason: "Creating"},
			},
			want: model.StatusProgressing,
		},
		{
			name: "only Ready=False (other reason), no Synced → Error",
			conditions: []model.Condition{
				{Type: "Ready", Status: "False", Reason: "SomethingBad"},
			},
			want: model.StatusError,
		},
		{
			name: "non-standard conditions only → Unknown",
			conditions: []model.Condition{
				{Type: "Healthy", Status: "True"},
				{Type: "Degraded", Status: "False"},
			},
			want: model.StatusUnknown,
		},
		{
			name: "Synced=Unknown, Ready=Unknown → Unknown",
			conditions: []model.Condition{
				{Type: "Synced", Status: "Unknown"},
				{Type: "Ready", Status: "Unknown"},
			},
			want: model.StatusUnknown,
		},
		{
			name: "Synced=True, Ready=Unknown → Unknown (not progressing)",
			conditions: []model.Condition{
				{Type: "Synced", Status: "True"},
				{Type: "Ready", Status: "Unknown"},
			},
			want: model.StatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveStatus(tt.conditions)
			if got != tt.want {
				t.Errorf("DeriveStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStringField(t *testing.T) {
	m := map[string]interface{}{
		"name":   "test",
		"count":  42,
		"nested": map[string]interface{}{"key": "val"},
		"empty":  "",
	}

	tests := []struct {
		key  string
		want string
	}{
		{"name", "test"},
		{"count", ""},     // not a string
		{"nested", ""},    // not a string
		{"missing", ""},   // not present
		{"empty", ""},     // empty string
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := stringField(m, tt.key)
			if got != tt.want {
				t.Errorf("stringField(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
