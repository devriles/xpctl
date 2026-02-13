package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/devriles/xpctl/internal/kube"
	"github.com/devriles/xpctl/internal/model"
	"github.com/devriles/xpctl/internal/tui"
)

var (
	version    = "dev"
	namespace  string
	kubeconfig string
	kubeCtx    string
	output     string
	noColor    bool
	debug      bool
)

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:   "xpctl [kind] [name]",
	Short: "Interactive TUI for visualizing Crossplane composition pipelines",
	Long: `xpctl is a standalone TUI tool (and k9s plugin) that lets you navigate
Crossplane composite resource hierarchies interactively — see synced/ready
status at a glance, drill into failed resources, and tail provider logs —
all without leaving the terminal.`,
	Version: version,
	Args:    cobra.MaximumNArgs(2),
	RunE:    run,
}

func init() {
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace (for claims; XRs are cluster-scoped)")
	rootCmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "k", "", "Path to kubeconfig (defaults to $KUBECONFIG or ~/.kube/config)")
	rootCmd.Flags().StringVarP(&kubeCtx, "context", "c", "", "Kubeconfig context to use")
	rootCmd.Flags().StringVarP(&output, "output", "o", "", `Non-interactive output: "tree", "json", "wide"`)
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging to ~/.xpctl/debug.log")
}

func Execute() error {
	return rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	client, err := kube.NewClient(kubeconfig, kubeCtx)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	if len(args) < 2 {
		if output != "" {
			return fmt.Errorf("--output requires <kind> and <name> arguments")
		}
		return tui.RunPicker(client)
	}

	kind := args[0]
	name := args[1]

	if output != "" {
		ctx := context.Background()
		tree, err := kube.Discover(ctx, client, kind, name, namespace)
		if err != nil {
			return fmt.Errorf("discovering resources: %w", err)
		}
		return printOutput(tree, output)
	}

	return tui.RunWithDiscovery(client, kind, name, namespace)
}

func printOutput(tree *kube.CompositionResult, mode string) error {
	switch mode {
	case "tree":
		printTree(tree.Root, "", true)
		return nil
	case "json":
		return printJSON(tree)
	case "wide":
		return printWide(tree)
	default:
		return fmt.Errorf("unknown output mode: %q (valid: tree, json, wide)", mode)
	}
}

func printTree(r *kube.ResourceNode, prefix string, isLast bool) {
	if r == nil {
		return
	}
	printTreeNode(r, prefix, isLast, true)
}

func printTreeNode(r *kube.ResourceNode, prefix string, isLast, isRoot bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if isRoot {
		connector = ""
	}

	statusIcon := "?"
	switch r.Status {
	case "Healthy":
		statusIcon = "✔"
	case "Error":
		statusIcon = "✘"
	case "Progressing":
		statusIcon = "⟳"
	}

	fmt.Fprintf(os.Stdout, "%s%s%s %s/%s\n", prefix, connector, statusIcon, r.Kind, r.Name)

	childPrefix := prefix
	if !isRoot {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range r.Children {
		printTreeNode(child, childPrefix, i == len(r.Children)-1, false)
	}
}

type jsonResource struct {
	Kind         string          `json:"kind"`
	APIVersion   string          `json:"apiVersion"`
	Name         string          `json:"name"`
	Namespace    string          `json:"namespace,omitempty"`
	ExternalName string          `json:"externalName,omitempty"`
	Status       string          `json:"status"`
	Synced       string          `json:"synced,omitempty"`
	Ready        string          `json:"ready,omitempty"`
	Age          string          `json:"age,omitempty"`
	Conditions   []jsonCondition `json:"conditions,omitempty"`
	Children     []jsonResource  `json:"children,omitempty"`
}

type jsonCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

func nodeToJSON(n *kube.ResourceNode) jsonResource {
	jr := jsonResource{
		Kind:       n.Kind,
		APIVersion: n.APIVersion,
		Name:       n.Name,
		Namespace:  n.Namespace,
		Status:     n.Status,
	}
	if n.Resource != nil {
		jr.ExternalName = n.Resource.ExternalName
		if n.Resource.Age > 0 {
			jr.Age = model.FormatAge(n.Resource.Age)
		}
		if n.Resource.Synced != nil {
			jr.Synced = n.Resource.Synced.Status
		}
		if n.Resource.Ready != nil {
			jr.Ready = n.Resource.Ready.Status
		}
		for _, c := range n.Resource.Conditions {
			jr.Conditions = append(jr.Conditions, jsonCondition{
				Type:    c.Type,
				Status:  c.Status,
				Reason:  c.Reason,
				Message: c.Message,
			})
		}
	}
	for _, child := range n.Children {
		jr.Children = append(jr.Children, nodeToJSON(child))
	}
	return jr
}

func printJSON(tree *kube.CompositionResult) error {
	root := nodeToJSON(tree.Root)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(root)
}

func printWide(tree *kube.CompositionResult) error {
	fmt.Fprintf(os.Stdout, "%-30s %-40s %-12s %-8s %-8s %-8s %s\n",
		"KIND", "NAME", "STATUS", "SYNCED", "READY", "AGE", "EXTERNAL-NAME")
	fmt.Fprintln(os.Stdout, strings.Repeat("─", 120))

	for _, node := range tree.Flat {
		synced, ready, extName, age := "-", "-", "-", "-"
		if node.Resource != nil {
			if node.Resource.Synced != nil {
				synced = node.Resource.Synced.Status
			}
			if node.Resource.Ready != nil {
				ready = node.Resource.Ready.Status
			}
			if node.Resource.ExternalName != "" {
				extName = node.Resource.ExternalName
			}
			if node.Resource.Age > 0 {
				age = model.FormatAge(node.Resource.Age)
			}
		}
		fmt.Fprintf(os.Stdout, "%-30s %-40s %-12s %-8s %-8s %-8s %s\n",
			node.Kind, node.Name, node.Status, synced, ready, age, extName)
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%d/%d Ready", tree.Stats.Healthy, tree.Stats.Total)
	if tree.Stats.Error > 0 {
		fmt.Fprintf(os.Stdout, " | %d Error", tree.Stats.Error)
	}
	if tree.Stats.Progressing > 0 {
		fmt.Fprintf(os.Stdout, " | %d Progressing", tree.Stats.Progressing)
	}
	fmt.Fprintln(os.Stdout)

	return nil
}
