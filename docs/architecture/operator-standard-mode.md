# Retina Operator Standard Mode

The operator in standard mode will only create related RetinaEndpoint resources and reconcile them.

**Creates RetinaEndpoint resources** when:
- `RemoteContext` config is enabled
- `EnableRetinaEndpoint` config is enabled

**How it works:**
1. `RetinaEndpointController` watches Pod events
2. For each Pod, it extracts relevant metadata into a `RetinaEndpoint` object
3. This reduces API server pressure by providing a lightweight, optimized proxy to pod metadata

**Key code:**
```go
if oconfig.RemoteContext && oconfig.EnableRetinaEndpoint {
    retinaendpointchannel := make(chan cache.PodCacheObject, ...)
    ke := retinaendpointcontroller.New(...)
    go ke.ReconcilePod(ctrlCtx)  // Pre-populate cache before manager starts
    
    pc := podcontroller.New(..., retinaendpointchannel)
    pc.SetupWithManager(mgr)  // Creates RetinaEndpoints from cached Pods
}
```

The `PodController` reconciles Pods → creates/updates `RetinaEndpoint` resources.
The `RetinaEndpointController` watches Pods and populates the cache.