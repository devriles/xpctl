# xpctl

[![CI](https://github.com/devriles/xpctl/actions/workflows/ci.yml/badge.svg)](https://github.com/devriles/xpctl/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/devriles/xpctl/branch/main/graph/badge.svg)](https://codecov.io/gh/devriles/xpctl)
[![Go Report Card](https://goreportcard.com/badge/github.com/devriles/xpctl)](https://goreportcard.com/report/github.com/devriles/xpctl)
[![Go Reference](https://pkg.go.dev/badge/github.com/devriles/xpctl.svg)](https://pkg.go.dev/github.com/devriles/xpctl)

Interactive TUI for visualizing Crossplane composition pipelines.

Navigate composite resource hierarchies, see synced/ready status at a glance, drill into failed resources, and tail provider logs — all without leaving the terminal.

![xpctl demo](demo/demo.gif)

## Features

- **Tree View** — Visualize XR/Claim composition hierarchies with status icons
- **Detail View** — Inspect conditions, events, and Crossplane annotations
- **Log View** — Stream provider pod logs filtered to a specific resource
- **Interactive Picker** — Browse all XRs and Claims when no args are given
- **Bottom-up Traversal** — Start from any managed resource and walk up to the parent XR
- **Non-interactive Output** — `tree`, `json`, and `wide` modes for CI/scripting
- **k9s Plugin** — Launch directly from k9s with Shift-T

## Prerequisites

- A Kubernetes cluster with [Crossplane](https://crossplane.io/) installed
- `kubectl` configured to access the cluster

## Install

### Homebrew

```bash
brew install devriles/tap/xpctl
```

### Go

```bash
go install github.com/devriles/xpctl@latest
```

### Binary

Download from [GitHub Releases](https://github.com/devriles/xpctl/releases).

## Usage

```bash
# Trace a specific composite resource
xpctl XMyApp my-app

# Trace a claim in a namespace
xpctl XMyAppClaim my-claim -n default

# Interactive picker (no args)
xpctl

# Non-interactive output
xpctl XMyApp my-app -o tree
xpctl XMyApp my-app -o json
xpctl XMyApp my-app -o wide
```

### Flags

```
-n, --namespace string    Namespace (for claims; XRs are cluster-scoped)
-k, --kubeconfig string   Path to kubeconfig (defaults to $KUBECONFIG or ~/.kube/config)
-c, --context string      Kubeconfig context to use
-o, --output string       Non-interactive output: "tree", "json", "wide"
    --no-color            Disable color output
    --debug               Enable debug logging to ~/.xpctl/debug.log
-v, --version             Show version
```

## Keybindings

### List View

| Key | Action |
|-----|--------|
| `↑`/`k` | Move up |
| `↓`/`j` | Move down |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Enter` | Open detail view |
| `l` | Open log view |
| `r` | Refresh |
| `q` | Quit |

### Detail View

| Key | Action |
|-----|--------|
| `j`/`k` | Scroll |
| `l` | Open log view |
| `Esc` | Back to list |
| `q` | Quit |

### Log View

| Key | Action |
|-----|--------|
| `G` | Jump to bottom (resume auto-scroll) |
| `Esc` | Back to list |
| `q` | Quit |

## k9s Plugin

Copy `k9s/plugin.yaml` to your k9s plugin directory, or merge it with your existing plugins:

```bash
# Linux/macOS
mkdir -p ~/.config/k9s
cp k9s/plugin.yaml ~/.config/k9s/plugins.yaml
```

Then press `Shift-T` on any resource in k9s to launch xpctl.

## How It Works

1. **Discovery** — Resolves the resource kind via Kubernetes API discovery, fetches the root resource
2. **Tree Building** — Follows `.spec.resourceRef` (Claims) and `.spec.resourceRefs[]` (XRs) recursively with concurrent fetching (max 10 parallel)
3. **Status Derivation** — Parses Crossplane conditions (Synced/Ready) to derive Healthy/Error/Progressing/Unknown
4. **Provider Logs** — Maps managed resources to their provider pod via ProviderRevision objects, with heuristic fallback

## Development

### Prerequisites

- [Go](https://go.dev/) 1.25+
- [kind](https://kind.sigs.k8s.io/) — local Kubernetes clusters (integration tests)
- [Helm](https://helm.sh/) — installs Crossplane into the kind cluster
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [k9s](https://k9scli.io/) — optional, for testing the plugin

### Build & Test

```bash
make build           # Build binary to bin/xpctl
make test            # Run unit tests
make vet             # Run go vet
make lint            # Run golangci-lint (install separately)
```

### Integration Tests

Integration tests run against a real kind cluster with Crossplane and [provider-nop](https://github.com/crossplane-contrib/provider-nop).

```bash
make setup-integration    # Create kind cluster, install Crossplane + fixtures
make test-integration     # Run integration tests, then tear down cluster
```

To keep the cluster running for manual testing (e.g., with k9s):

```bash
make setup-integration
go test -race -tags integration -timeout 120s -count=1 -v ./internal/kube/
make teardown-integration  # When done
```

## License

Apache 2.0
