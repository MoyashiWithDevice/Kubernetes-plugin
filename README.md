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
| `kubectl detective pods` | List pods in the cluster |
| `kubectl detective services` | List services in the cluster |
| `kubectl detective endpointslices` | List EndpointSlices in the cluster |
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

### Real-time Flow Capture

```bash
# Show TCP flows with service name resolution
kubectl detective flows

# Show raw IPs (no Kubernetes resolution)
kubectl detective flows -n

# Resolve to Pod names only
kubectl detective flows --pod
```

### Service Dependency Map

```bash
# ASCII art dependency map (10 second capture)
kubectl detective map -d 10s

# Mermaid diagram
kubectl detective map -f mermaid

# HTML report
kubectl detective map -f html -o report.html

# CSV export
kubectl detective map -f csv -o flows.csv
```

### Throughput Monitoring

```bash
# Top talkers for 30 seconds
kubectl detective top -d 30s

# Live updating display
kubectl detective top -w

# Per-endpoint aggregation
kubectl detective top --endpoints

# Specific unit
kubectl detective top -M  # MB
```

### Retransmission Analysis

```bash
# Retransmission ranking
kubectl detective retrans -d 30s

# Live watch mode
kubectl detective retrans -w
```

### Latency Monitoring

```bash
# RTT latency with p95/p99
kubectl detective latency -d 30s

# Live watch mode
kubectl detective latency -w
```

### DNS Analysis

```bash
# DNS query latency
kubectl detective dns -d 30s

# Live watch mode
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
kubectl detective map -f csv -o output.csv

# JSON
kubectl detective map -f json -o output.json

# HTML report
kubectl detective map -f html -o output.html
```

## License

[MIT](LICENSE)
