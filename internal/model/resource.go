package model

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type StatusSummary string

const (
	StatusHealthy     StatusSummary = "Healthy"
	StatusProgressing StatusSummary = "Progressing"
	StatusError       StatusSummary = "Error"
	StatusUnknown     StatusSummary = "Unknown"
)

type Condition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime time.Time
}

type Resource struct {
	APIVersion   string
	Kind         string
	Name         string
	Namespace    string
	ExternalName string // crossplane.io/external-name annotation

	Synced     *Condition
	Ready      *Condition
	Conditions []Condition

	Status StatusSummary
	Age    time.Duration
	Raw    *unstructured.Unstructured
}

// FormatAge formats a duration as a compact age string (e.g. "5s", "3m", "2h", "7d").
func FormatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
