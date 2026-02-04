# Flow Architecture in Retina

## Overview

A **flow** in Retina is a structured network observation object that represents a single network connection or communication pattern. It is based on the [Cilium Flow protocol buffer](https://github.com/cilium/cilium) definition (`github.com/cilium/cilium/api/v1/flow`).

## What is a Flow?

A flow represents a discrete network communication event between two endpoints. It provides a comprehensive view of network traffic with rich metadata that enables observability, troubleshooting, and security analysis.

### Flow Components

A flow contains the following key components:

| Component | Description |
|-----------|-------------|
| **Network Layer (L3)** | Source and destination IP addresses, IP version (IPv4/IPv6) |
| **Transport Layer (L4)** | Source and destination ports, protocol (TCP/UDP), TCP flags |
| **Observation Point** | Location in the network stack where the flow was captured |
| **Traffic Direction** | Ingress (into the pod) or Egress (out of the pod) |
| **Verdict** | Forwarded, dropped, or error state |
| **Enriched Metadata** | Kubernetes context (pod names, namespaces, services) |
| **Application Layer (L7)** | DNS queries/responses, HTTP metadata when available |

## Flow Creation Pipeline

The flow creation follows a multi-stage pipeline from raw kernel events to enriched observability data:

```
BPF Programs → Plugins → Enricher → Metrics Module
              ↓
         flow.Flow Object
```

### Stage 1: BPF Programs

eBPF programs running in the kernel capture raw network events at various observation points:
- Network interface (FROM_NETWORK, TO_NETWORK)
- Network stack (TO_STACK, TO_ENDPOINT)
- Connection tracking events

### Stage 2: Plugins

Plugins process raw BPF events and convert them into structured `flow.Flow` objects using `utils.ToFlow()`. The primary handler is the **packetparser plugin** (`pkg/plugin/packetparser/packetparser_linux.go:589`):

```go
fl := utils.ToFlow(
    p.l,
    ktime.MonotonicOffset.Nanoseconds()+int64(bpfEvent.T_nsec),
    utils.Int2ip(bpfEvent.SrcIp).To4(),
    utils.Int2ip(bpfEvent.DstIp).To4(),
    sourcePortShort,
    destinationPortShort,
    bpfEvent.Proto,
    bpfEvent.ObservationPoint,
    flow.Verdict_FORWARDED,
)
```

Key plugins that create flows:
- **packetparser**: Main plugin converting BPF events to flows
- **dropreason**: Creates flows for dropped packets with drop reasons
- **dns**: Creates flows for DNS queries and responses
- **tcpretrans**: Creates flows for TCP retransmissions

### Stage 3: Enricher

The enricher (`pkg/enricher/enricher.go`) adds Kubernetes context to flows:
- Pod name and namespace
- Service name and namespace
- Deployment information
- Workload type identification

### Stage 4: Metrics Module

The metrics module (`pkg/module/metrics/types.go:85`) processes enriched flows to generate Prometheus metrics:

```go
func ProcessFlow(f *flow.Flow)
```

This generates industry-standard metrics for:
- Network traffic volume (bytes, packets)
- Latency measurements
- Drop rates and reasons
- TCP flag distributions
- DNS query statistics

## Example Flow Structure

```protobuf
message Flow {
  FlowType type = 1;                    // L3_L4, L7, etc.
  IP ip = 2;                            // Network layer info
  L4 l4 = 3;                            // Transport layer info (TCP/UDP)
  L7 l7 = 4;                            // Application layer info
  TrafficDirection traffic_direction = 5;  // INGRESS, EGRESS
  TraceObservationPoint observation_point = 6;  // Where captured
  Verdict verdict = 7;                  // FORWARDED, DROPPED, etc.
  Endpoint source = 8;                  // Source endpoint
  Endpoint destination = 9;            // Destination endpoint
  repeated Endpoint destinations = 10; // For multicast
  bool is_reply = 11;                  // Is this a reply packet
 google.protobuf.Timestamp time = 12;  // Timestamp
  CiliumEventType event_type = 13;      // Event type info
  Extensions extensions = 14;           // Custom extensions (RetinaMetadata)
}
```

## Key Source Files

| File | Purpose |
|------|---------|
| `pkg/plugin/packetparser/packetparser_linux.go` | Main plugin converting BPF events to flows |
| `pkg/utils/flow_utils.go` | Flow creation and manipulation utilities |
| `pkg/enricher/enricher.go` | Kubernetes metadata enrichment |
| `pkg/module/metrics/types.go` | Flow processing for metrics |
| `pkg/hubble/parser/` | Hubble parsers for flow parsing |

## Summary

Flows are the central abstraction in Retina's network observability pipeline:

1. **Originate** from BPF programs capturing network events
2. **Transformed** by plugins into structured `flow.Flow` objects
3. **Enriched** with Kubernetes context
4. **Processed** for metrics generation and export

This architecture enables Retina to provide comprehensive, cloud-agnostic network observability for Kubernetes workloads.

## Flow Pipeline (within retina-agent)

```
┌─────────────────────────────────────────────────────────────────┐
│                    retina-agent (daemon)                        │
│                                                                 │
│   ┌─────────────┐                                              │
│   │ BPF Programs│  (kernel - raw network events)              │
│   │  - tc       │                                              │
│   │  - packet   │                                              │
│   │  - dns      │                                              │
│   └──────┬──────┘                                              │
│          │                                                     │
│          ▼                                                     │
│   ┌─────────────┐                                              │
│   │   Plugins   │  (convert BPF events → flow.Flow objects)   │
│   │             │                                              │
│   │ packetparser│  ← main flow creator                         │
│   │ dropreason  │                                              │
│   │ dns         │                                              │
│   │ tcpretrans  │                                              │
│   └──────┬──────┘                                              │
│          │                                                     │
│          ▼                                                     │
│   ┌─────────────┐                                              │
│   │  Enricher   │  (add pod/node metadata via cache)           │
│   └──────┬──────┘                                              │
│          │                                                     │
│          ▼                                                     │
│   ┌─────────────┐                                              │
│   │   Metrics   │  (generate Prometheus metrics from flows)    │
│   │   Module    │                                              │
│   └─────────────┘                                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```


In code

| Plugin | File | Line |
|--------|------|------|
| packetparser | pkg/plugin/packetparser/packetparser_linux.go | 589 |
| dropreason | pkg/plugin/dropreason/dropreason_linux.go | 302 |
| dns | pkg/plugin/dns/dns_linux.go | 127 |
| tcpretrans | pkg/plugin/tcpretrans/tcpretrans_linux.go | 124 |
All use utils.ToFlow() from pkg/utils/flow_utils.go:33-128.
Logging flows: No built-in debug logging exists. There's only a warning when flow creation fails (p.l.Warn("Could not convert bpfEvent to flow")).