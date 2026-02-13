package kube

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// StreamLogs opens a log stream for a pod. If resourceName is non-empty, only
// lines containing that string are forwarded (client-side filtering).
func StreamLogs(ctx context.Context, c *Client, podName, containerName, resourceName string) (io.ReadCloser, error) {
	clientset, err := kubernetes.NewForConfig(c.Config)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}

	var tailLines int64 = 100
	opts := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    true,
		TailLines: &tailLines,
	}

	stream, err := clientset.CoreV1().Pods("crossplane-system").GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening log stream for pod %s: %w", podName, err)
	}

	if resourceName == "" {
		return stream, nil
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		defer stream.Close()
		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, resourceName) {
				fmt.Fprintln(pw, line)
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(pw, "[log scanner error: %v]\n", err)
		}
	}()

	return pr, nil
}
