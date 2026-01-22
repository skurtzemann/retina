# Standard Control Plane Architecture

The Standard Control Plane processes data collected by the Data Plane (eBPF plugins) into metrics. Here's how it works:

## Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        DATA PLANE                               │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐ │
│  │  eBPF    │    │  eBPF    │    │  eBPF    │    │  eBPF    │ │
│  │ Plugins  │───▶│ perf array│───▶│ flow     │───▶│ Enricher │ │
│  │(kernel)  │    │(buffers) │    │objects   │    │(user)    │ │
│  └──────────┘    └──────────┘    └──────────┘    └────┬─────┘ │
└─────────────────────────────────────────────────────────────┘     │
                                                              │
                              ┌──────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     CONTROL PLANE                               │
│                                                                  │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                     ENRICHER                            │  │
│   │  1. Reads flow objects from plugins                     │  │
│   │  2. Uses K8s cache (IPs → Pods/Namespaces/Workloads)  │  │
│   │  3. Enriches flows with Kubernetes context             │  │
│   │  4. Writes to output ring                               │  │
│   └──────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│   ┌──────────────────────────────────────────────────────────┐  │
│   │                 METRICS MODULE                           │  │
│   │  1. Reads enriched flows from enricher output ring      │  │
│   │  2. Processes flows into Prometheus metrics            │  │
│   │  3. Supports filtering by namespace/annotations        │  │
│   │  4. Exports metrics on :9965 (Prometheus)              │  │
│   └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

> **Mermaid Diagram**: See [`02-standard-control-plane-diagram.mmd`](./02-standard-control-plane-diagram.mmd) for an enhanced visual representation.

## Key Components

| Component | Purpose |
|-----------|---------|
| **Enricher** | Adds K8s context (Pod name, namespace, labels, workload) to raw flow objects by mapping IPs to cached K8s objects |
| **Cache** | Maintains IP-to-Kubernetes-object mapping (Pods, Services, Nodes) |
| **Metrics Module** | Transforms enriched flows into `networkobservability_*` Prometheus metrics |
| **Filter Manager** | Filters which pods/namespaces to generate metrics for |

## Plugins Available

| Plugin | Purpose |
|--------|---------|
| `packetforward` | Packet/byte counts with direction |
| `dropreason` | Dropped packets with reason (iptables, conntrack, etc.) |
| `dns` | DNS requests/responses with query info |
| `linuxutil` | TCP/UDP/interface statistics |
| `packetparser` | Advanced per-pod metrics (TCP flags, retransmits, API server latency) |
| `tcpretrans` | TCP retransmission counts |
| `hnsstats` | Windows HNS/VFP metrics |

## Supported On

- **Linux**: Full Standard Control Plane support
- **Windows**: Standard Control Plane only (Hubble not supported)

## Why Use Standard Control Plane?

1. **Capture CRD support** - Required for packet capture functionality
2. **More metric plugins** - DNS, TCP retrans, API server latency metrics
3. **Pod-level filtering** - Filter metrics by namespace or pod annotations
4. **Windows support** - Only control plane that works on Windows nodes