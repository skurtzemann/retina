# Agent Node Controller

The NodeController is used by the `retina-agent` (not the operator).

Location: `cmd/standard/daemon.go:269-273`

```go
mainLogger.Info("Initializing Node controller")
nodeController := nc.New(mgr.GetClient(), controllerCache)
if err := nodeController.SetupWithManager(mgr); err != nil {
    mainLogger.Fatal("unable to create nodeController", zap.Error(err))
}
```

**What it does (`pkg/controllers/daemon/node/controller.go`):**
1. Watches Node CRUD events
2. Extracts zone from node labels (`topology.kubernetes.io/zone`)
3. Updates the cache with `RetinaNode` containing zone info

**The flow for AZ metrics:**

```
retina-agent (daemon)
    │
    ├── NodeController
    │   └── Watches Nodes → extracts zone → updates cache
    │
    └── RetinaEndpointController  
        └── Watches Pods → stores pod→node mapping → updates cache
             │
             ▼
        cache.GlobalCache
             │
             ▼
        packetparser (metrics module)
             │
             └── GetZoneByPodIP(ip)
                 └─> RetinaEndpoint → nodeName → RetinaNode → zone
```

The operator doesn't use this NodeController. The AZ feature relies on the **agent** running to populate the cache.