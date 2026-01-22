# Data Plane Overview

The Data Plane is responsible for **collecting raw network data** from the kernel using eBPF programs.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        KERNEL SPACE                             │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐     │
│  │  tc/ingress  │    │  tc/egress   │    │  kprobes     │     │
│  │  bpf prog    │    │  bpf prog    │    │  bpf prog    │     │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘     │
│         │                   │                   │              │
│         ▼                   ▼                   ▼              │
│  ┌─────────────────────────────────────────────────────┐      │
│  │            eBPF MAPS (kernel)                         │      │
│  │  - packet counts    - drop reasons   - DNS queries   │      │
│  │  - TCP stats        - connection tracking            │      │
│  └─────────────────────────┬───────────────────────────┘      │
└────────────────────────────┼────────────────────────────────────┘
                             │ perf_event_array (per-CPU buffers)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       USER SPACE                               │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  PLUGIN MANAGER                        │   │
│  │  - Manages plugin lifecycle (Init, Start, Stop)       │   │
│  │  - Compiles eBPF code                                  │   │
│  │  - Reconciles plugins                                  │   │
│  └────────────────────────────┬──────────────────────────┘   │
│                               │                                 │
│       ┌───────────────────────┼───────────────────────┐        │
│       ▼                       ▼                       ▼        │
│  ┌─────────┐            ┌─────────┐            ┌─────────┐      │
│  │ Plugin1 │            │ Plugin2 │            │ PluginN │      │
│  │(packet  │            │  (dns)  │            │(drop    │      │
│  │parser)  │            │         │            │reason)  │      │
│  └────┬────┘            └────┬────┘            └────┬────┘      │
│       │                     │                     │           │
│       └─────────────────────┼─────────────────────┘           │
│                             │                                     │
│                             ▼                                     │
│                    ┌─────────────────┐                           │
│                    │  flow objects   │                           │
│                    │  (Cilium proto) │                           │
│                    └────────┬────────┘                           │
│                             │                                     │
│                             ▼                                     │
│                    CONTROL PLANE                                 │
│                    (Enricher)                                     │
└─────────────────────────────────────────────────────────────────┘
```

> **Mermaid Diagram**: See [`03-data-plane-overview-diagram.mmd`](./03-data-plane-overview-diagram.mmd) for an enhanced visual representation.

## Key Components

| Component | Purpose |
|-----------|---------|
| **eBPF Programs** | Kernel programs that hook into network paths (tc, kprobes, tracepoints) |
| **eBPF Maps** | In-kernel storage for counts, stats, and intermediate data |
| **Perf Arrays** | Transfer data from kernel to user space (per-CPU buffers) |
| **Plugin Manager** | Loads, initializes, and manages plugin lifecycle |
| **Plugins** | Individual collectors (packetparser, dns, dropreason, etc.) |
| **Watchers** | Endpoint Watcher, API Server Watcher for pod context |

## Plugin Lifecycle

```
┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐
│ Generate│───▶│Compile  │───▶│ Init    │───▶│ Start   │───▶│ Stop    │
│ (headers)│   │ (clang) │   │(maps)   │   │(read)   │   │(cleanup)│
└─────────┘   └─────────┘   └─────────┘   └─────────┘   └─────────┘
```

## Available Plugins

| Plugin | What It Collects | eBPF Hook |
|--------|------------------|-----------|
| **packetparser** | Full packet details, TCP flags, direction | tc ingress/egress |
| **dropreason** | Dropped packets with reason | kprobes |
| **dns** | DNS requests/responses | kprobes |
| **packetforward** | Forwarded packet/byte counts | tc |
| **linuxutil** | TCP/UDP/interface stats | syscalls/netstat |
| **tcpretrans** | TCP retransmissions | kprobes |
| **conntrack** | Connection tracking events | kprobes |
| **hnsstats** | Windows HNS/VFP stats | Windows API |

## Data Flow Example (packetparser)

```
1. Pod created → Endpoint Watcher detects new veth interface
2. tc qdisc + clsact attached to interface
3. BPF program attached to ingress/egress hooks
4. Packet passes through → eBPF program records to map
5. Kernel → perf array (per-CPU buffer)
6. User space reads perf buffer
7. Parse packet → create flow object
8. Send to Enricher (Standard) or external channel (Hubble)
```

## Performance Considerations

- **Perf arrays**: Use per-CPU buffers (no locking)
- **High-core systems**: 32+ CPUs may experience performance impact with packetparser
- **Sampling**: `DATA_SAMPLING_RATE` controls sampling (default: 1)
- **Aggregation**: `DATA_AGGREGATION_LEVEL` controls filtering level