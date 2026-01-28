# Packet Capture to Flow Creation

## Overview

This document explains how network packets are captured by the eBPF packetparser plugin and transformed into Cilium Flow protobuf objects that are used for metrics enrichment.

---

## eBPF Packet Capture Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        eBPF Packet Capture Flow                             │
└─────────────────────────────────────────────────────────────────────────────┘

Packet travels through network interface
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  tc qdisc (clsact) attaches eBPF filter programs                           │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Four observation points (from packetparser.c):                    │   │
│  │                                                                     │   │
│  │  1. FROM_NETWORK  ──► host ingress (incoming from internet)       │   │
│  │  2. TO_NETWORK    ──► host egress (outgoing to internet)           │   │
│  │  3. FROM_ENDPOINT ──► pod ingress (incoming to pod veth)          │   │
│  │  4. TO_ENDPOINT   ──► pod egress (outgoing from pod veth)          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
        ┌───────────────────────────────────────┐
        │  eBPF program parses packet:          │
        │                                         │
        │  Ethernet header → IP header →         │
        │  TCP/UDP header → TCP flags, ports      │
        │                                         │
        │  Extracts:                             │
        │  - src_ip, dst_ip                       │
        │  - src_port, dst_port                   │
        │  - protocol (TCP/UDP)                   │
        │  - TCP flags (SYN, ACK, FIN, RST...)    │
        │  - observation_point                    │
        │  - bytes (packet size)                  │
        │  - timestamp (t_nsec)                  │
        └───────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Send raw data via BPF_MAP_TYPE_PERF_EVENT_ARRAY                           │
│  (per-CPU buffer, kernel → userspace)                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Userspace Processing

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                   packetparser plugin (userspace)                          │
└─────────────────────────────────────────────────────────────────────────────┘

perf reader reads from kernel (handleEvents goroutine)
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  processRecord() workers (2 workers)                                        │
│                                                                             │
│  Binary decode raw packet data into packetparserPacket struct:              │
│                                                                             │
│  type packetparserPacket struct {                                          │
│      t_nsec                    int64    // timestamp                       │
│      bytes                     uint32   // packet size                    │
│      src_ip, dst_ip            uint32   // IPs (network order)            │
│      src_port, dst_port        uint16   // ports (network order)          │
│      proto                     uint8    // TCP/UDP                         │
│      flags                     uint8    // TCP flags bitmask              │
│      observation_point         uint8    // FROM_NETWORK, TO_NETWORK, etc.  │
│      tcp_metadata              struct { seq, ack, tsval, tsecr }          │
│  }                                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Convert to Cilium Flow protobuf:                                          │
│                                                                             │
│  flow := &flow.Flow{                                                       │
│      IP: &flow.IP{                                                         │
│          Source:      utils.Int2ip(pkt.src_ip).String(),  // "10.244.0.5"   │
│          Destination: utils.Int2ip(pkt.dst_ip).String(),  // "10.244.0.6"   │
│          IpVersion:   flow.IPVersion_IPv4,                                 │
│      },                                                                    │
│      L4: &flow.L4{                                                         │
│          TCP: &flow.TCP{                                                   │
│              SourcePort:      uint16(pkt.src_port),                        │
│              DestinationPort: uint16(pkt.dst_port),                        │
│          },                                                                │
│      },                                                                    │
│      Verdict:        flow.Verdict_FORWARDED,                               │
│      TrafficDirection: flow.TrafficDirection(pkt.traffic_direction),       │
│      IsReply:        &wrappers.Bool{Value: pkt.is_reply},                   │
│  }                                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Write to Enricher (adds Kubernetes context):                              │
│                                                                             │
│  enricher.Write(event)                                                      │
│                                                                             │
│  Enricher looks up pod by IP, adds:                                         │
│  - Source: PodName, Namespace, Labels, Workloads                            │
│  - Destination: PodName, Namespace, Labels, Workloads                       │
└─────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Forward to Metrics Modules:                                                │
│                                                                             │
│  ProcessFlow(flow) → Aggregate and emit Prometheus metrics                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### What is the Cilium Flow Protobuf?

The **Cilium Flow** is a protobuf message (protocol buffer) defined by the [Cilium](https://cilium.io/) project - a popular CNI plugin for Kubernetes. It provides a standardized data structure for representing network flow information.

#### Flow Structure (from Cilium Hubble API)

```protobuf
// Simplified representation of the Flow message
message Flow {
    IP IP = 1;
    L4 L4 = 2;
    L7 L7 = 3;
    Endpoint Source = 4;
    Endpoint Destination = 5;
    Verdict Verdict = 6;
    TrafficDirection TrafficDirection = 7;
    bool IsReply = 8;
    int64 Time = 9;
    // ... many more fields
}

message IP {
    string Source = 1;
    string Destination = 2;
    IPVersion IpVersion = 3;
}

message Endpoint {
    string PodName = 1;
    string Namespace = 2;
    repeated string Labels = 3;
    repeated Workload Workloads = 4;
    string HostName = 5;
    // NOTE: No Zone or NodeName field in standard Cilium Flow
}

message TCP {
    uint32 SourcePort = 1;
    uint32 DestinationPort = 2;
    // TCP flags, sequence numbers, etc.
}
```

#### What Retina Uses from Flow

| Field | Usage | Source |
|-------|-------|--------|
| `flow.IP.Source` | Pod IP (e.g., "10.244.0.5") | Parsed from packet |
| `flow.IP.Destination` | Pod IP | Parsed from packet |
| `flow.L4.TCP` / `UDP` | Ports | Parsed from packet |
| `flow.Source.PodName` | Enricher adds | Kubernetes |
| `flow.Source.Namespace` | Enricher adds | Kubernetes |
| `flow.TrafficDirection` | EGRESS/INGRESS | From packet |
| `flow.Verdict` | FORWARDED/DROPPED | From packet |
| **Zone** | **NOT in Flow** | Cache lookup |

#### Why Zone is NOT in Flow

The Cilium Flow protobuf is an **external dependency** (imported from `github.com/cilium/cilium/api/v1/flow`). It's maintained by the Cilium project, not Retina.

This is why Retina doesn't add zone to Flow - it would require:
1. Modifying an external protobuf definition
2. Waiting for Cilium maintainers to accept the change
3. Updating the dependency version in Retina

Instead, Retina keeps zone in its own internal types (`RetinaNode`) and looks it up at metrics time via the cache.

#### Key Facts

| Aspect | Description |
|--------|-------------|
| **What** | Protocol Buffer message from Cilium project |
| **Purpose** | Standardized network flow representation |
| **Location** | `vendor/github.com/cilium/cilium/api/v1/flow/flow.pb.go` |
| **Usage** | Pod IPs, ports, TCP flags, traffic direction, verdict |
| **Limitations** | No zone/node topology fields in standard definition |

---

## Example: TCP Packet Flow

```
Pod A (10.244.0.5) ──► Pod B (10.244.0.6)
nginx:80           ←    frontend:5432
```

**eBPF capture at TO_ENDPOINT (Pod A's veth egress):**

```c
// In kernel: pkt structure
pkt.t_nsec = 1737891234567890;      // nanoseconds since boot
pkt.bytes = 64;                      // packet size
pkt.src_ip = 0x0AA40005;            // 10.244.0.5 (network order)
pkt.dst_ip = 0x0AA40006;            // 10.244.0.6 (network order)
pkt.src_port = 0x0019 (80);         // nginx
pkt.dst_port = 0x153E (5432);       // frontend
pkt.proto = IPPROTO_TCP;
pkt.flags = TCP_SYN;                // 0b00000010
pkt.observation_point = TO_ENDPOINT;
```

**Userspace Flow after processing:**

```go
flow := &flow.Flow{
    IP: &flow.IP{
        Source:      "10.244.0.5",
        Destination: "10.244.0.6",
        IpVersion:   flow.IPVersion_IPv4,
    },
    L4: &flow.L4{
        TCP: &flow.TCP{
            SourcePort:      80,
            DestinationPort: 5432,
        },
    },
    Verdict:        flow.Verdict_FORWARDED,
    TrafficDirection: flow.TrafficDirection_EGRESS,
    IsReply:        &wrappers.Bool{Value: false},
    // NOTE: Zone NOT yet available here
}
```

**After Enrichment:**

```go
flow.Source = &flow.Endpoint{
    PodName:   "nginx-abc123",
    Namespace: "default",
    Labels:    []string{"app=nginx", "version=v1"},
    // Zone NOT in flow.Endpoint (single source is RetinaNode)
}
flow.Destination = &flow.Endpoint{
    PodName:   "frontend-xyz789",
    Namespace: "default",
    Labels:    []string{"app=frontend"},
}
```

**Zone is looked up later in Metrics ContextOptions:**

```go
// In getByDirectionValues()
if c.Zone {
    ip := flow.IP.Source  // "10.244.0.5"
    zone := cache.GlobalCache.GetZoneByPodIP(ip)
    // Returns: "us-east-1a" (from RetinaNode)
}
```

---

## Key Point: Zone is NOT in Flow

| Component | What's in Flow | Where Zone Lives |
|-----------|---------------|------------------|
| `flow.IP` | Source/Destination IPs | - |
| `flow.Endpoint` | PodName, Namespace, Labels | - |
| **Zone** | **NOT here** | `RetinaNode.zone` (single source of truth) |

The zone is looked up **at metrics time** via cache, not during flow creation. This keeps the flow protobuf clean and avoids duplicating zone data.

---

## Related Documents

- [AZ Lookup Flow](az-lookup-flow.md) - How zone is looked up for metrics
- [PacketParser Plugin Overview](../../plugins/packetparser.md) - Full plugin documentation