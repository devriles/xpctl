package kube

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Event struct {
	Type      string // "Normal" or "Warning"
	Reason    string
	Message   string
	Count     int32
	FirstSeen metav1.Time
	LastSeen  metav1.Time
	Source    string
}

func FetchEvents(ctx context.Context, c *Client, kind, name, namespace string) ([]Event, error) {
	eventGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}

	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind)

	var events []Event
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	list, err := c.Dynamic.Resource(eventGVR).Namespace(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		// Try cluster-scoped events if namespace fails
		list, err = c.Dynamic.Resource(eventGVR).Namespace("").List(ctx, metav1.ListOptions{
			FieldSelector: fieldSelector,
		})
		if err != nil {
			return nil, fmt.Errorf("listing events: %w", err)
		}
	}

	for _, item := range list.Items {
		obj := item.Object
		ev := Event{
			Type:    stringField(obj, "type"),
			Reason:  stringField(obj, "reason"),
			Message: stringField(obj, "message"),
		}

		if count, ok := obj["count"].(int64); ok {
			ev.Count = int32(count)
		}

		if source, ok := obj["source"].(map[string]interface{}); ok {
			ev.Source = stringField(source, "component")
		}

		events = append(events, ev)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].LastSeen.After(events[j].LastSeen.Time)
	})

	return events, nil
}
