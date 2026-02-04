# Understanding Option B: Cache Lookup for Pod Names

## The Data Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                     KUBERNETES CLUSTER                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────┐     CRUD Event      ┌─────────────────────────────┐│
│  │    Pod      │────────────────────▶│  RetinaEndpointController   ││
│  │ nginx-abcde │                     │  (pkg/controllers/daemon/   ││
│  │ (IP: ...)   │                     │   retinaendpoint/)          ││
│  │             │                     │                             ││
│  │             │                     │        retina-agent         ││
│  └─────────────┘                     └────────────┬────────────────┘│
│                                                   │                 │
│                                                   ▼                 │
│                                          ┌─────────────────────┐    │
│                                          │   GlobalCache       │    │
│                                          │   (pkg/controllers/ │    │
│                                          │    cache/cache.go)  │    │
│                                          └──────────┬──────────┘    │
│                                                     │               │
└─────────────────────────────────────────────────────┼───────────────┘
                                                      │
                         ┌────────────────────────────┼────────────────────────────┐
                         │                            │                            │
                         ▼                            ▼                            ▼
               ┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
               │  packetparser   │         │     Metrics     │         │   Option B      │
               │   (logs flow)   │         │   (adds zone)   │         │ (our new code)  │
               └─────────────────┘         └─────────────────┘         └─────────────────┘
```

## Step-by-Step

**1. Pod created in Kubernetes**
```
kubectl run nginx --image=nginx
# Pod: default/nginx-abcde, IP: 10.244.0.5, Node: ip-10-0-1-23
```

**2. RetinaEndpointController watches Pod events** (`pkg/controllers/daemon/retinaendpoint/controller.go`)

**3. Creates RetinaEndpoint in GlobalCache** (`pkg/common/types.go:37-44`)
```go
type RetinaEndpoint struct {
    BaseObject        // name, namespace
    labels      map[string]string
    annotations map[string]string
    nodeName    string          // The node where pod is scheduled
}
```

**4. GlobalCache stores mapping:** `podIP → RetinaEndpoint`

**5. Our code calls:** `cache.GlobalCache.GetPodByIP("10.244.0.5")`

**6. Returns RetinaEndpoint with:**
- `Name()` → `"nginx-abcde"`
- `Namespace()` → `"default"`
- `NodeName()` → `"ip-10-0-1-23"`
- `Labels()` → `map[string]string{...}`

## Code References

| File | Purpose |
|------|---------|
| `pkg/controllers/cache/cache.go:69` | `GetPodByIP(ip string) *RetinaEndpoint` |
| `pkg/common/types.go:37` | `RetinaEndpoint` struct definition |
| `pkg/controllers/daemon/retinaendpoint/controller.go` | Watches pods, populates cache |

## Summary

**Data source:** Kubernetes API via RetinaEndpointController  
**Cache:** `GlobalCache` singleton  
**Lookup method:** `cache.GlobalCache.GetPodByIP(ip)`  
**Returns:** Pod name, namespace, node, labels

**Note:** If pod IP is not in cache (pod not yet reconciled, or external IP), we show `"<unknown>"`.