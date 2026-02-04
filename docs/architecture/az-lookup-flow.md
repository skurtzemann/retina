# Availability Zone Lookup Flow

## AZ Lookup Flow Diagram for Metrics

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        AZ Lookup Flow for Metrics                           │
└─────────────────────────────────────────────────────────────────────────────┘

Step 1: Packet captured → Flow created with IPs
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Enricher: Look up pod by source IP in cache                                 │
│  cache.GetObjByIP(flow.IP.Source)                                            │
└─────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
        ┌───────────────────────────────────────┐
        │  Found: RetinaEndpoint{               │
        │    name="nginx-abc123",               │
        │    namespace="default",               │
        │    nodeName="ip-10-0-1-23.ec2.internal"│
        │  }                                    │
        └───────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  ContextOptions.getByDirectionValues(): Get zone for source IP               │
│  ip := flow.IP.Source (e.g., "10.244.0.5")                                   │
│  zone := cache.GlobalCache.GetZoneByPodIP(ip)                                │
└─────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Cache.GetZoneByPodIP(ip):                                                   │
│  1. ep := cache.GetPodByIP(ip)  ──► RetinaEndpoint{nodeName="..."}         │
│  2. node := cache.GetNodeByName(ep.nodeName) ──► RetinaNode{name="...",zone="us-east-1a"} │
│  3. Return node.Zone()                                                      │
└─────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
        ┌───────────────────────────────────────┐
        │  Zone found: "us-east-1a"             │
        └───────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Emit metric with zone label:                                                │
│  adv_forward_count{source_zone="us-east-1a", ...}                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Cache Data Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        Cache (in-memory)                        │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  epMap: "default/nginx" ──► RetinaEndpoint{               │ │
│  │    name="nginx"                                            │ │
│  │    namespace="default"                                     │ │
│  │    nodeName="ip-10-0-1-23"  ◄──┐                          │ │
│  │  }                             │                          │ │
│  │                                │                          │ │
│  │  ipToEpKey:                     │                          │ │
│  │    "10.244.0.5" ──► "default/nginx"                       │ │
│  │                                │                          │ │
│  │  nodeMap:                       │                          │ │
│  │    "ip-10-0-1-23" ──► RetinaNode{                        │ │
│  │      name="ip-10-0-1-23"          │                          │ │
│  │      zone="us-east-1a"  ─────────┘                          │ │
│  │    }                                                        │ │
│  │                                                             │ │
│  │  ipToNodeName:                                              │ │
│  │    "10.0.1.23" ──► "ip-10-0-1-23"                          │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## Single Source of Truth

```
┌─────────────┐                    ┌──────────────────┐
│   Node      │    has labels      │  RetinaNode      │
│  (K8s API)  │ ─────────────────►│  (in cache)      │
│             │                    │                  │
│ topology.   │                    │  name="node-1"   │
│ kubernetes. │                    │  zone="us-east-1a"│
│ io/zone:    │                    │  ip="10.0.1.23"  │
│ "us-east-1a"│                    │                  │
└─────────────┘                    └──────────────────┘
        │                                   │
        │ (reconcile on change)             │
        │                                   │
        ▼                                   ▼
┌──────────────────────────────────────────────────────────────┐
│  Pods reference nodeName → RetinaEndpoint.nodeName ──► lookup │
│  RetinaNode.zone ──► metrics label                           │
│                                                              │
│  Single source: zone from Node, never duplicated              │
└──────────────────────────────────────────────────────────────┘
```

---

## Lookup Latency (in-memory)

```
GetZoneByPodIP(ip)
│
├── GetPodByIP(ip)           ──► O(1) map lookup
│   └── Returns: RetinaEndpoint{nodeName="..."}
│
└── GetNodeByName(nodeName)  ──► O(1) map lookup
    └── Returns: RetinaNode{zone="..."}
    
Total: ~2 map lookups = microseconds ⚡
```

---

## Why Single Source of Truth Matters

| Concern | Solution |
|---------|----------|
| **Data consistency** | Zone stored only in `RetinaNode`, never duplicated in `RetinaEndpoint` |
| **Updates** | When node zone changes, automatic reconciliation updates `RetinaNode`; all pods get new zone on next metric emission |
| **No sync needed** | No logic to sync zone between endpoint and node |

---

## Debug Logs

Enable DEBUG level logging to trace AZ lookups:

```bash
kubectl logs -n kube-system -l app=retina --tail=200 | grep -E "(zone|Zone)"
```

Expected output:
```
DEBUG: Found GA zone label on node label=topology.kubernetes.io/zone zone=us-east-1a
DEBUG: Zone found for pod IP ip=10.244.0.5 pod=default/nginx node=ip-10-0-1-23 zone=us-east-1a
```

---

## Files Involved

| File | Role |
|------|------|
| `pkg/common/types.go` | `RetinaEndpoint` with `nodeName` field |
| `pkg/common/node.go` | `RetinaNode` with `zone` field |
| `pkg/controllers/cache/cache.go` | `GetZoneByPodIP()` method |
| `pkg/controllers/daemon/node/controller.go` | Extracts zone from node labels |
| `pkg/module/metrics/types.go` | Calls `GetZoneByPodIP()` for metrics |