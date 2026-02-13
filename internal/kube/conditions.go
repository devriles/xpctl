package kube

import (
	"time"

	"github.com/devriles/xpctl/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ParseConditions(obj *unstructured.Unstructured) []model.Condition {
	conditionsRaw, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return nil
	}

	var conditions []model.Condition
	for _, c := range conditionsRaw {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		cond := model.Condition{
			Type:    stringField(cm, "type"),
			Status:  stringField(cm, "status"),
			Reason:  stringField(cm, "reason"),
			Message: stringField(cm, "message"),
		}
		if ts := stringField(cm, "lastTransitionTime"); ts != "" {
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				cond.LastTransitionTime = t
			}
		}
		conditions = append(conditions, cond)
	}
	return conditions
}

func FindCondition(conditions []model.Condition, condType string) *model.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// DeriveStatus computes health from Synced and Ready conditions.
func DeriveStatus(conditions []model.Condition) model.StatusSummary {
	synced := FindCondition(conditions, "Synced")
	ready := FindCondition(conditions, "Ready")

	if synced != nil && synced.Status == "False" {
		return model.StatusError
	}
	if ready != nil && ready.Status == "True" {
		return model.StatusHealthy
	}
	if ready != nil && ready.Status == "False" {
		if ready.Reason == "Creating" || ready.Reason == "Pending" {
			return model.StatusProgressing
		}
		return model.StatusError
	}
	if synced != nil && synced.Status == "True" && ready == nil {
		return model.StatusProgressing
	}
	return model.StatusUnknown
}

func stringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
