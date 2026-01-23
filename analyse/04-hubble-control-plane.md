# Hubble Control Plane Architecture

The Hubble Control Plane is an alternative to the Standard Control Plane that leverages **Cilium Hubble** for network observability. It provides deeper integration with Hubble CLI and UI tools.

## Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        DATA PLANE                               │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐ │
│  │  eBPF    │    │  eBPF    │    │  eBPF    │    │  External│ │
│  │ Plugins  │───▶│ perf array│───▶│ flow     │───▶│ Channel  │ │
│  │(kernel)  │    │(buffers) │    │objects   │    │          │ │
│  └──────────┘    └──────────┘    └──────────┘    └────┬─────┘ │
└─────────────────────────────────────────────────────────────┘     │
                                                              │
                              ┌──────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     CONTROL PLANE                               │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                 MONITOR AGENT                            │  │
│   │  1. Monitors external channel for plugin events          │  │
│   │  2. Manages listeners (Hubble Observer)                 │  │
│   │  3. Manages consumers (decoded message handlers)         │  │
│   └──────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                HUBBLE OBSERVER                           │  │
│   │  1. Registered as Monitor Agent consumer                 │  │
│   │  2. Receives flow events from Monitor Agent              │  │
│   │  3. Parses flows (L4, DNS, Drop types)                   │  │
│   │  4. Enriches with K8s context (Cilium libraries)        │  │
│   └──────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                    OUTPUT ENDPOINTS                       │  │
│   │  - :9965  - Prometheus metrics (hubble_*)                │  │
│   │  - :4244  - Hubble Relay (flow logs)                     │  │
│   │  - unix:///var/run/cilium/hubble.sock - Local socket    │  │
│   └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

> **Mermaid Diagram**: See [`04-hubble-control-plane-diagram.mmd`](./04-hubble-control-plane-diagram.mmd) for an enhanced visual representation.

## Key Components

| Component | Purpose |
|-----------|---------|
| **External Channel** | Buffer where plugins write flow objects (replaces Enricher in Standard) |
| **Monitor Agent** | Manages consumers/listeners, forwards plugin events to Hubble Observer |
| **Hubble Observer** | Receives flows, parses them using custom parsers, enriches with K8s context using Cilium libraries |
| **Hubble Relay** | Aggregates flow logs from all nodes for Hubble CLI/UI access |
| **Hubble CLI** | Command-line tool for real-time flow observation |
| **Hubble UI** | Graphical service map and flow visualization |

## Hubble Metrics

| Metric Name | Description | Labels |
|-------------|-------------|---------|
| `hubble_dns_queries_total` | Total DNS requests by query | source/destination, query, qtypes |
| `hubble_dns_responses_total` | Total DNS responses | source/destination, query, qtypes, rcode, ips_returned |
| `hubble_drop_total` | Total dropped packet count | source/destination, protocol, reason |
| `hubble_tcp_flags_total` | TCP packets by flag | source/destination, flag |
| `hubble_flows_processed_total` | Total network flows (L4/L7) | source/destination, protocol, verdict, type, subtype |

## Supported On

- **Linux**: Full Hubble Control Plane support
- **Windows**: Not supported (requires Hubble)

## Comparison: Standard vs Hubble Control Plane

| Aspect | Standard Control Plane | Hubble Control Plane |
|--------|------------------------|----------------------|
| **Platforms** | Linux, Windows | Linux only |
| **Data Path** | Enricher → Metrics Module | External Channel → Monitor Agent → Hubble Observer |
| **Enrichment** | Custom Enricher component | Cilium Hubble libraries |
| **Metrics Prefix** | `networkobservability_*` | `hubble_*` |
| **Modes** | Basic, Advanced Remote, Advanced Local | N/A (fixed metric structure) |
| **Capture CRD** | Supported | Not supported |
| **CLI** | kubectl retina | Hubble CLI |
| **UI** | Grafana | Hubble UI + Grafana |
| **Output Ports** | :9965 (Prometheus) | :9965 (Prometheus), :4244 (Relay), Unix socket |
| **Flow Logs** | Via CRD capture | Via Hubble Relay |

## Why Use Hubble Control Plane?

1. **Hubble Integration** - Access to Hubble CLI and UI for real-time observability
2. **Service Maps** - Visualize service dependencies with Hubble UI
3. **Flow Tracing** - CLI-based real-time flow observation
4. **Cilium Ecosystem** - Tight integration with Cilium networking stack
5. **Advanced Visualization** - Better tooling for network troubleshooting

## Installation

```shell
VERSION=$(curl -sL https://api.github.com/repos/microsoft/retina/releases/latest | jq -r .name)
helm upgrade --install retina oci://ghcr.io/microsoft/retina/charts/retina-hubble \
    --version $VERSION \
    --namespace kube-system \
    --set os.windows=true \
    --set operator.enabled=true \
    --set agent.enabled=true \
    --set agent.init.enabled=true \
    --set logLevel=info \
    --set hubble.tls.enabled=false \
    --set hubble.relay.tls.server.enabled=false
```

## Accessing Hubble UI/CLI

```bash
# Port-forward for Hubble UI
kubectl port-forward -n kube-system svc/hubble-ui 8081:80

# Port-forward for Hubble CLI
kubectl port-forward -n kube-system svc/hubble-relay 4245:80

# Use Hubble CLI
hubble observe --follow --namespace default
```