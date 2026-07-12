# kubectl-detective

[![CI](https://github.com/kubernetes-sigs/kubectl-detective/actions/workflows/ci.yml/badge.svg)](https://github.com/kubernetes-sigs/kubectl-detective/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/kubernetes-sigs/kubectl-detective)](https://github.com/kubernetes-sigs/kubectl-detective/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Kubernetes network diagnostic tool using eBPF. Capture TCP flows, visualize service dependencies, and monitor network health in real-time.

```text
frontend → api → redis
```

## Features

| Command | Description |
|---|---|
| `kubectl detective flows` | Capture and display TCP flows in real-time |
| `kubectl detective map` | Show service dependency map (ASCII, Mermaid, CSV, JSON, HTML) |
| `kubectl detective top` | Show throughput top talkers |
| `kubectl detective retrans` | Show TCP retransmission ranking |
| `kubectl detective latency` | Show Pod-to-Pod TCP latency (RTT) ranking |
| `kubectl detective dns` | Show DNS query latency statistics |
| `kubectl detective status` | Show cluster-wide network status |
| `kubectl detective agent` | Run the eBPF agent on a node (DaemonSet) |
| `kubectl detective aggregator` | Run the gRPC aggregator server |

## Quick Start

### Install

```bash
# Binary (Linux/macOS/Windows)
curl -sL https://github.com/kubernetes-sigs/kubectl-detective/releases/latest/download/kubectl-detective_$(uname -s)_$(uname -m).tar.gz | tar xz -C /usr/local/bin kubectl-detective

# kubectl plugin
chmod +x /usr/local/bin/kubectl-detective
```

### Helm Install

```bash
helm install kubectl-detective ./charts/kubectl-detective \
  --namespace detective --create-namespace
```

### From Source

```bash
git clone https://github.com/kubernetes-sigs/kubectl-detective.git
cd kubectl-detective
make install
```

## Usage

All data-collection commands share a consistent interface:

- **Default**: Collects for 10 seconds (`-d`), then exits with a summary.
- **`-w` (watch)**: Continuous display. `Ctrl+D` shows results and exits; `Ctrl+C` exits immediately.
- **`-n`**: Skip Kubernetes name resolution (show IPs only).
- **`--pod`**: Resolve to Pod names only.
- **`--svc`**: Resolve to Service names only (default).
- **`--no-headers`**: Suppress progress messages (useful for piping).

### Real-time Flow Capture

```bash
# Show TCP flows (10 second capture)
kubectl detective flows

# Continuous display (Ctrl+D or Ctrl+C to stop)
kubectl detective flows -w

# Show raw IPs (no Kubernetes resolution)
kubectl detective flows -n

# Capture for 30 seconds
kubectl detective flows -d 30s
```

### Service Dependency Map

```bash
# ASCII art dependency map (10 second capture)
kubectl detective map

# Continuous collection (Ctrl+D to show map, Ctrl+C to quit)
kubectl detective map -w

# Mermaid diagram
kubectl detective map -f mermaid

# HTML report
kubectl detective map -f html -F report.html

# CSV export
kubectl detective map -f csv -F flows.csv
```

### Throughput Monitoring

```bash
# Top talkers (10 second capture)
kubectl detective top

# Continuous display (Ctrl+D or Ctrl+C to stop)
kubectl detective top -w

# Per-endpoint aggregation
kubectl detective top --endpoints

# Specific unit
kubectl detective top -M  # MB
```

### Retransmission Analysis

```bash
# Retransmission ranking (10 second capture)
kubectl detective retrans

# Continuous display (Ctrl+D or Ctrl+C to stop)
kubectl detective retrans -w
```

### Latency Monitoring

```bash
# RTT latency with p95/p99 (10 second capture)
kubectl detective latency

# Continuous display (Ctrl+D or Ctrl+C to stop)
kubectl detective latency -w
```

### DNS Analysis

```bash
# DNS query latency (10 second capture)
kubectl detective dns

# Continuous display (Ctrl+D or Ctrl+C to stop)
kubectl detective dns -w
```

### Cluster-wide Status (DaemonSet Mode)

```bash
# Deploy agent + aggregator
helm install kubectl-detective ./charts/kubectl-detective -n detective --create-namespace

# View cluster status
kubectl detective status

# View specific section
kubectl detective status -o top
kubectl detective status -o latency
kubectl detective status -o retrans
kubectl detective status -o dns
kubectl detective status -o flows
```

## Architecture

```text
┌─────────────┐     gRPC      ┌──────────────┐
│ detective   │──────────────▶│  aggregator   │
│ agent       │               │  (Deployment) │
│ (DaemonSet) │               └──────┬───────┘
└─────────────┘                      │
       │                             │
       ▼                             ▼
  ┌──────────┐              ┌──────────────┐
  │  eBPF    │              │   kubectl    │
  │  probes  │              │   detective  │
  └──────────┘              │   status     │
                            └──────────────┘
```

- **Agent**: DaemonSet running on each node. Uses eBPF to capture TCP connect events, retransmissions, RTT samples, and DNS queries. Sends periodic snapshots to the aggregator via gRPC.
- **Aggregator**: Central Deployment that receives and aggregates metrics from all agents.
- **CLI**: kubectl plugin that connects to the aggregator to display cluster-wide views, or runs locally with eBPF privileges for single-node analysis.

## Requirements

- Linux kernel 5.8+ (for eBPF)
- Kubernetes 1.25+
- eBPF capabilities (CAP_BPF, CAP_NET_ADMIN, CAP_PERFMON)

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint

# Format + Vet + Test + Build
make all

# Build eBPF object
make bpf

# Generate protobuf code
make proto

# Create kind cluster
make kind-up

# Deploy to kind
make kind-deploy
```

## Export Formats

```bash
# ASCII (default)
kubectl detective map -f ascii

# Mermaid diagram
kubectl detective map -f mermaid

# CSV
kubectl detective map -f csv -F output.csv

# JSON
kubectl detective map -f json -F output.json

# HTML report
kubectl detective map -f html -F output.html
```

## License

[MIT](LICENSE)
