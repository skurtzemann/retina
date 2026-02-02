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

**Flow diagram:**

```
retina-agent (daemon)
│
├── Controllers
│   ├── NodeController
│   │   └── Watches Nodes → updates cache with node metadata
│   │
│   └── RetinaEndpointController
│       └── Watches Pods → stores pod→node mapping → updates cache
│
├── GlobalCache
│   ├── RetinaNode (nodeName → node metadata)
│   └── RetinaEndpoint (podIP → pod info, nodeName)
│
├── packetparser Plugin
│   └── Creates flow.Flow objects from BPF events
│
├── Enricher (pkg/enricher/enricher.go)
│   └── flow.Flow → enriched with Kubernetes metadata
│       │
│       └── GetPodByIP(podIP) → cache.GlobalCache
│           └── Returns: podName, podNamespace, nodeName, labels, etc.
│
└── Metrics Module
    └── Processes enriched flows → Prometheus metrics
```

**Enricher integration:**

The enricher (`pkg/enricher/enricher.go`) adds Kubernetes context to flows:

1. Receives `flow.Flow` from packetparser plugin
2. Calls `GetPodByIP(podIP)` on GlobalCache
3. Returns enriched flow with:
   - Pod name and namespace
   - Node name
   - Pod labels
   - Workload info (deployment, statefulset, etc.)

**Cache population:**

- **NodeController**: Watches Node CRUD events, populates `RetinaNode` in cache
- **RetinaEndpointController**: Watches Pod CRUD events, populates `RetinaEndpoint` in cache

The operator doesn't use these controllers. They are exclusive to the **retina-agent** daemon.