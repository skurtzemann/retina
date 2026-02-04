# Cache Architecture

## Control-Plane Mode Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│  Control-Plane Modes:                                               │
│                                                                     │
│  ┌─────────────────────┐    ┌─────────────────────┐                 │
│  │       STANDARD      │    │       HUBBLE        │                 │
│  │   (cmd/standard)    │    │  (cmd/hubble)       │                 │
│  └──────────┬──────────┘    └──────────┬──────────┘                 │
│             │                          │                            │
│             ▼                          ▼                            │
│  ┌─────────────────────┐    ┌─────────────────────┐                 │
│  │  ControllerCache    │    │  ipcache.IPCache    │ ◄── Cilium      │
│  │  (pkg/controllers/  │    │  ServiceCache       │ ◄── native      │
│  │   cache/cache.go)   │    │                     │                 │
│  └─────────────────────┘    └─────────────────────┘                 │
└─────────────────────────────────────────────────────────────────────┘
```

- This document describes the `ControllerCache` used by **STANDARD** control-plane.
- **HUBBLE** control-plane uses Cilium's native ipcache.IPCache instead.

## Cache Activation Flow

```
┌───────────────────────────────────────────────────────────────────────┐
│  CONFIGURATION GATEWAY                                                │
│                                                                       │
│  daemon.yaml / ControllerManager Config                               │
│  ┌──────────────────────────────────────────────────────────────┐     │
│  │ EnablePodLevel: true ──────────┬──► ACTIVATES CACHE          │     │
│  │                                │                             │     │
│  │ EnableCacheDebugLog: true ─────┼──► Enables debug logging    │     │
│  │                                │     (when cache active)     │     │
│  │                                │                             │     │
│  │ EnableAnnotations: true ───────┼──► Enables namespace filter │     │
│  │                                │     (when cache active)     │     │
│  │                                │                             │     │
│  │ RemoteContext: false ──────────┼──► Filters pods to local    │     │
│  │                                │     node (API-level)        │     │
│  └──────────────────────────────────────────────────────────────┘     │
└───────────────────────────────────────────────────────────────────────┘
```

## Cache Data Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DATA SOURCES                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│   │     Pod     │    │    Node     │    │   Service   │    │  Endpoint   │  │
│   │  API Server │    │  API Server │    │  API Server │    │  API Server │  │
│   └──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘  │
│          │                  │                  │                  │         │
│          ▼                  ▼                  ▼                  ▼         │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                     DAEMON CONTROLLERS                              │   │
│   │                                                                     │   │
│   │   PodReconciler ──────►│◄──────── RetinaEndpointReconciler          │   │
│   │   NodeReconciler ─────►│◄──────── ServiceReconciler                 │   │
│   │                                                                     │   │
│   │   (pkg/controllers/daemon/*/controller.go)                          │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                  │                                          │
│                                  ▼                                          │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    CONTROLLERCACHE                                  │   │
│   │   ┌─────────────────────────────────────────────────────────────┐   │   │
│   │   │  Internal Maps:                                             │   │   │
│   │   │  ┌─────────────┬─────────────────────────────────────────┐  │   │   │
│   │   │  │ ipToEpKey   │ pod IP  ──► pod key (RetinaEndpoint)    │  │   │   │
│   │   │  ├─────────────┼─────────────────────────────────────────┤  │   │   │
│   │   │  │ ipToSvcKey  │ svc IP  ──► svc key (RetinaSvc)         │  │   │   │
│   │   │  ├─────────────┼─────────────────────────────────────────┤  │   │   │
│   │   │  │ ipToNodeName│ node IP ──► node name (RetinaNode)      │  │   │   │
│   │   │  ├─────────────┼─────────────────────────────────────────┤  │   │   │
│   │   │  │ epMap       │ pod key ──► RetinaEndpoint              │  │   │   │
│   │   │  ├─────────────┼─────────────────────────────────────────┤  │   │   │
│   │   │  │ nodeMap     │ node name ──► RetinaNode                │  │   │   │
│   │   │  └─────────────┴─────────────────────────────────────────┘  │   │   │
│   │   └─────────────────────────────────────────────────────────────┘   │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                  │                                          │
│            ┌─────────────────────┼─────────────────────┐                    │
│            ▼                     ▼                     ▼                    │
│   ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐           │
│   │   ENRICHER      │   │   METRICS       │   │  PACKET PARSER  │           │
│   │   (enricher.go) │   │  (metrics/types)│   │  (packetparser) │           │
│   │                 │   │                 │   │                 │           │
│   │   Enriches      │   │  Adds AZ labels │   │  Tags packets   │           │
│   │   flows with    │   │  to metrics:    │   │  with src/dst   │           │
│   │   pod/node info │   │  zone=metadata  │   │  zones          │           │
│   └─────────────────┘   └─────────────────┘   └─────────────────┘           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Zone Detection

### Data Flow for GetZoneByPodIP

```
┌──────────────────────────────────────────────────────────────────────┐
│  Zone Detection Flow (GetZoneByPodIP)                                │
│                                                                      │
│  ┌──────────┐     ┌─────────────────────────────────────┐            │
│  │  Pod IP  │ ──► │ ipToEpKey                           │            │
│  └──────────┘     │ pod IP ──► pod key (RetinaEndpoint) │            │
│                   └───────────────────┬─────────────────┘            │
│                                       │                              │
│                                       ▼                              │
│                   ┌───────────────────────────────────┐              │
│                   │ RetinaEndpoint.ep.NodeName()      │              │
│                   └───────────────────┬───────────────┘              │
│                                       │                              │
│                                       ▼                              │
│                   ┌───────────────────────────────────┐              │
│                   │ nodeMap                           │              │
│                   │ node name ──► RetinaNode          │              │
│                   └───────────────────┬───────────────┘              │
│                                       │                              │
│                                       ▼                              │
│  ┌───────────────────────────────────────────────┐                   │
│  │ node.Zone                                     │                   │
│  │ "eastus-1"  or  "unknown" (fallback)          │                   │
│  └───────────────────────────────────────────────┘                   │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### Zone Label Precedence (Node Controller)

The node controller extracts the zone from node labels using this precedence order:

```
┌───────────────────────────────────────────────────────┐
│  ZONE LABEL PRECEDENCE ORDER                          │
│                                                       │
│  1. topology.kubernetes.io/zone  ◄── GA               │
│        │                                              │
│        ▼                                              │
│  2. failure-domain.beta.kubernetes.io/zone  ◄── BETA  │
│        │                                              │
│        ▼                                              │
│  3. "" (fallback) → "unknown"                         │
└───────────────────────────────────────────────────────┘
```

- **`topology.kubernetes.io/zone`** - GA label (preferred)
- **`failure-domain.beta.kubernetes.io/zone`** - Beta label (deprecated)
- **No label** → Returns `"unknown"`


## Configuration Matrix

| Config Option | Daemon | Controller-Manager | Effect |
|---------------|--------|-------------------|--------|
| `EnablePodLevel: false` | No cache created | No cache used | Cache completely disabled |
| `EnablePodLevel: true` | Creates local cache | Uses GlobalCache | Cache enabled |
| `EnableCacheDebugLog: true` | Periodic stats logging | Same | Verbose `GetZoneByPodIP` logging |
| `RemoteContext: false` | Filters to local node | N/A | Pods filtered to current node only |

## Debug & Observability

The `/debug/cache` endpoint returns cache statistics and metrics:

```json
{
  "cache_stats": {
    "cache": "agent-cache",
    "num_nodes": 5,
    "num_pods": 100,
    "num_services": 20,
    "num_ip_to_node": 5,
    "num_ip_to_pod": 150,
    "num_ip_to_services": 20
  },
  "get_zone_by_pod_ip_calls": 12345
}
```

### Fields

| Field | Description |
|-------|-------------|
| `cache` | Cache name (e.g., "agent-cache") |
| `num_nodes` | Number of nodes in cache |
| `num_pods` | Number of pods in cache |
| `num_services` | Number of services in cache |
| `num_ip_to_node` | Number of IP → node mappings |
| `num_ip_to_pod` | Number of IP → pod mappings |
| `num_ip_to_services` | Number of IP → service mappings |
| `get_zone_by_pod_ip_calls` | Number of times `GetZoneByPodIP` was called |