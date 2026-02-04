# Cache Architecture Analysis

## Overview

Retina creates **two separate cache instances** in different execution paths. This document analyzes both caches, their purposes, and the implications for debugging.

## Two Cache Creation Paths

### Cache 1: Local Controller Cache

**Location:** `cmd/standard/daemon.go:242`

```go
controllerCache := controllercache.New(pubSub)
// Used by: Pod, Node, Service, RetinaEndpoint controllers
// NO SetConfig() called
```

**Used By:**
- Pod controller (`podController`)
- Node controller (`nodeController`)
- Service controller (`svcController`)
- RetinaEndpoint controller (`retinaEndpointController`)

**Issue:**
- `cfg` field is `nil` (no `SetConfig()` call)
- Condition `c.cfg != nil && c.cfg.EnableCacheDebugLog` always evaluates to `false`
- **No debug logs** from this cache (even if `enableCacheDebugLog: true`)

---

### Cache 2: Global Cache (Controller Manager)

**Location:** `pkg/managers/controllermanager/controllermanager.go:83-84`

```go
m.cache = cache.New(m.pubsub)
m.cache.SetConfig(m.conf)  // ← Config IS set here
cache.GlobalCache = m.cache  // ← Assigned to GlobalCache
```

**Used By:**
- Plugin manager (packetparser, dns, etc.)
- Metrics module
- Any code accessing `cache.GlobalCache`

**Behavior:**
- `cfg` field is properly set
- Debug logs work correctly when `enableCacheDebugLog: true`
- Periodic cache statistics logged every 5 minutes

---

## Comparison Table

| Aspect | Local Cache (`controllerCache`) | Global Cache (`GlobalCache`) |
|--------|----------------------------------|------------------------------|
| **Location** | `cmd/standard/daemon.go:242` | `pkg/managers/controllermanager/controllermanager.go:83` |
| **Config Set** | ❌ No | ✅ Yes |
| **Used By** | Controllers (Pod, Node, Service, RetinaEndpoint) | Plugins, Metrics |
| **Debug Logs** | ❌ Silent | ✅ Working |
| **Global Access** | ❌ No | ✅ Yes (`cache.GlobalCache`) |
| **Stats Logging** | ✅ Yes (if enabled) | ✅ Yes (if enabled) |

---

## Current Behavior with `enableCacheDebugLog: true`

| Cache | Expected Logs | Actual Logs |
|-------|---------------|-------------|
| Local (`controllerCache`) | ❌ None | ❌ None (correct) |
| Global (`GlobalCache`) | ✅ Pod/Node logs, stats | ✅ Working |

### Example Logs from Global Cache

```
INFO: Storing pod in cache pod=kubernetes-apiserver/kubernetes-apiserver ips=10.1.1.184,10.1.2.163,172.20.0.1 node=
INFO: Cache statistics num_nodes=2 num_pods=26 num_services=16 num_ip_to_node=2 num_ip_to_pod=28
```

---

## Why Two Caches?

The codebase has two paths:

1. **Standard Control Plane** (`cmd/standard/daemon.go`):
   - Uses local `controllerCache`
   - Controllers directly use this cache instance

2. **Plugin-based Architecture** (`pkg/managers/controllermanager/`):
   - Creates separate `m.cache` with proper config
   - Assigns to `cache.GlobalCache` for shared access

---

## Implications for Debugging

### Missing Node Logs

The Node controller uses `controllerCache`, which has no config. This means:
- "Storing node in cache" logs are **not** appearing
- "Looking up node by IP" logs are **not** appearing
- This makes debugging node cache issues difficult

### Recommended Fix

Add `SetConfig()` call for `controllerCache` in `cmd/standard/daemon.go`:

```go
controllerCache := controllercache.New(pubSub)
controllerCache.SetConfig(daemonConfig)  // ← ADD THIS LINE
```

This ensures:
- Consistent logging behavior across both caches
- Debug logs appear for all cache operations
- Better debugging experience

---

## Code References

| File | Line | Purpose |
|------|------|---------|
| `cmd/standard/daemon.go` | 242 | Creates local `controllerCache` |
| `pkg/managers/controllermanager/controllermanager.go` | 83-84 | Creates global cache with config |
| `pkg/controllers/cache/cache.go` | 22-23 | Cache struct with `cfg` field |
| `pkg/controllers/cache/cache.go` | 551 | `SetConfig()` method |
| `pkg/controllers/cache/cache.go` | 567 | `LogStatistics()` method |

---

## Future Improvement: Unify Caches

A cleaner architecture would use a **single cache instance** shared by both:
- Controllers
- Plugins
- Metrics module

This would:
- Simplify debugging
- Avoid inconsistent state
- Reduce memory usage
- Eliminate duplicate cache entries

**Note:** The TODO comment at `cmd/standard/daemon.go:236` acknowledges this is a temporary solution that needs refactoring.