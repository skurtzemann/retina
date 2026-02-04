# Availability Zone Metrics - Changelog

## Overview

This feature adds availability zone (AZ) labels to advanced metrics in the packetparser plugin for the Standard Control Plane mode.

## Changes

### 1. New Constants (`pkg/common/const.go`)

Added topology label constants for availability zone lookups:

- `TopologyZoneLabelGA` - `topology.kubernetes.io/zone` (GA label)
- `TopologyZoneLabelBeta` - `failure-domain.beta.kubernetes.io/zone` (beta label, for backward compatibility)
- `TopologyZoneLabelFallback` - `"unknown"` (fallback when AZ is not available)

### 2. Extended `RetinaNode` (`pkg/common/types.go`, `pkg/common/node.go`)

Added `zone` field to store the availability zone:

```go
type RetinaNode struct {
    name string
    ip   net.IP
    zone string  // NEW: availability zone
}

func (n *RetinaNode) Zone() string {
    return n.zone
}
```

Updated `NewRetinaNode()` to accept zone parameter:

```go
func NewRetinaNode(name string, ip net.IP, zone string) *RetinaNode
```

### 3. Extended `RetinaEndpoint` (`pkg/common/types.go`)

Added `nodeName` field and `NodeName()` method to track which node a pod is scheduled on:

```go
type RetinaEndpoint struct {
    // ... existing fields ...
    nodeName string  // NEW: node where pod is scheduled
}

func (ep *RetinaEndpoint) NodeName() string {
    return ep.nodeName
}
```

Updated `RetinaEndpointCommonFromPod()` to capture `pod.Spec.NodeName`.

### 4. Node Controller Updates (`pkg/controllers/daemon/node/controller.go`)

Added `extractZoneWithLog()` helper function to extract AZ from node labels with debug logging:

```go
func (r *NodeReconciler) extractZoneWithLog(labels map[string]string) string {
    if labels == nil {
        r.l.Debug("Node has no labels, using fallback zone",
            zap.String("reason", "labels_is_nil"))
        return retinaCommon.TopologyZoneLabelFallback
    }
    if az, ok := labels[retinaCommon.TopologyZoneLabelGA]; ok {
        r.l.Debug("Found GA zone label on node",
            zap.String("label", retinaCommon.TopologyZoneLabelGA),
            zap.String("zone", az))
        return az
    }
    if az, ok := labels[retinaCommon.TopologyZoneLabelBeta]; ok {
        r.l.Debug("Found beta zone label on node (deprecated)",
            zap.String("label", retinaCommon.TopologyZoneLabelBeta),
            zap.String("zone", az))
        return az
    }
    r.l.Debug("No zone labels found on node",
        zap.Strings("available_labels", getLabelKeys(labels)))
    return retinaCommon.TopologyZoneLabelFallback
}
```

Added `getLabelKeys()` helper to log available node labels for debugging.

Updated `Reconcile()` to pass zone when creating `RetinaNode` with debug logging:

```go
zone := r.extractZoneWithLog(node.Labels)
retinaNodeCommon := retinaCommon.NewRetinaNode(node.Name, ip, zone)
r.l.Debug("Updated RetinaNode in Cache",
    zap.String("node", node.Name),
    zap.String("zone", zone))
```

### 5. Cache Updates (`pkg/controllers/cache/cache.go`, `pkg/controllers/cache/types.go`)

Added new methods to `CacheInterface`:

```go
GetNodeByName(nodeName string) *common.RetinaNode
GetZoneByPodIP(ip string) string
```

Added global cache instance:

```go
var GlobalCache CacheInterface
```

Added debug logging to `GetZoneByPodIP()`:

```go
func (c *Cache) GetZoneByPodIP(ip string) string {
    ep := c.GetPodByIP(ip)
    if ep == nil {
        c.l.Debug("Pod not found for IP, using fallback zone",
            zap.String("ip", ip))
        return common.TopologyZoneLabelFallback
    }
    node := c.GetNodeByName(ep.NodeName())
    if node == nil {
        c.l.Debug("Node not found for pod, using fallback zone",
            zap.String("pod", ep.Key()),
            zap.String("node_name", ep.NodeName()))
        return common.TopologyZoneLabelFallback
    }
    zone := node.Zone()
    if zone == "" {
        c.l.Debug("Node has empty zone, using fallback",
            zap.String("node", node.Name()))
        return common.TopologyZoneLabelFallback
    }
    c.l.Debug("Zone found for pod IP",
        zap.String("ip", ip),
        zap.String("pod", ep.Key()),
        zap.String("node", node.Name()),
        zap.String("zone", zone))
    return zone
}
```

### 6. Metrics Context Options (`pkg/module/metrics/types.go`)

Added zone support to `ContextOptions`:

- New constant: `zoneCtxOption = "zone"`
- New field: `Zone bool` in `ContextOptions` struct
- Updated `NewCtxOption()` to handle `"zone"` option
- Updated `getLabels()` to include `source_zone` / `destination_zone`
- Updated `getByDirectionValues()` to look up AZ via `cache.GlobalCache.GetZoneByPodIP()`

## Usage

### Enable Zone Labels

Add `"zone"` to your metrics context options:

```yaml
apiVersion: retina.sh/v1alpha1
kind: Metrics
metadata:
  name: advanced-metrics
spec:
  contextOptions:
    - metricName: forward
      sourceLabels:
        - "ip"
        - "namespace"
        - "podname"
        - "zone"  # NEW: include zone
      destinationLabels:
        - "ip"
        - "namespace"
        - "podname"
        - "zone"  # NEW: include zone
```

### Resulting Metrics

With zone enabled, metrics will include labels like:

```
adv_forward_count{
    direction="egress",
    source_namespace="default",
    source_podname="nginx-abc123",
    source_zone="us-east-1a",
    destination_namespace="default",
    destination_podname="frontend-xyz789",
    destination_zone="us-east-1b"
}
```

## Backward Compatibility

- **Default behavior unchanged**: Zone labels are not added by default to `DefaultCtxOptions()`
- **Opt-in only**: Users must explicitly add `"zone"` to their context options
- **Fallback value**: When AZ cannot be determined, `"unknown"` is used as the label value

## Requirements

- Kubernetes cluster with node topology labels
- Nodes must have either `topology.kubernetes.io/zone` or `failure-domain.beta.kubernetes.io/zone` labels
- Advanced metrics mode must be enabled (`EnablePodLevel: true`)

## Debug Logging

All AZ lookups are logged at DEBUG level for troubleshooting:

### Node Controller Logs
```
DEBUG: Node has no labels, using fallback zone reason=labels_is_nil
DEBUG: Found GA zone label on node label=topology.kubernetes.io/zone zone=us-east-1a
DEBUG: Found beta zone label on node (deprecated) label=failure-domain.beta.kubernetes.io/zone zone=us-east-1a
DEBUG: No zone labels found on node available_labels=[...]
DEBUG: Updated RetinaNode in Cache node=ip-10-0-1-23 zone=us-east-1a
```

### Cache Logs
```
DEBUG: Pod not found for IP, using fallback zone ip=10.244.0.5
DEBUG: Node not found for pod, using fallback zone pod=default/nginx-abc123 node_name=ip-10-0-1-23
DEBUG: Zone found for pod IP ip=10.244.0.5 pod=default/nginx-abc123 node=ip-10-0-1-23 zone=us-east-1a
```

## Troubleshooting

### Check Node Labels Exist
```bash
kubectl get nodes --show-labels | grep topology.kubernetes.io/zone
```

### Verify Retina Logs
```bash
kubectl logs -n kube-system -l app=retina --tail=200 | grep -E "(zone|Zone)"
```

### Query Metrics for Zones
```promql
adv_forward_count{source_zone!="unknown"}
```

### Expected Behavior on AWS EKS
1. EKS nodes should have `topology.kubernetes.io/zone` label
2. First reconcile logs: `DEBUG: Found GA zone label on node ... zone=us-east-1a`
3. Pod metrics should show zone labels after enrichment

### Common Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| `zone=unknown` for all pods | Node missing AZ labels | Add `topology.kubernetes.io/zone` label to nodes |
| `zone=unknown` for some pods | Pod scheduled on node not yet reconciled | Wait for node controller to reconcile |
| No zone logs | DEBUG logs may be disabled | Check log level configuration |

## Files Changed

| File | Change Type |
|------|-------------|
| `pkg/common/const.go` | NEW |
| `pkg/common/types.go` | Modified |
| `pkg/common/node.go` | Modified |
| `pkg/controllers/daemon/node/controller.go` | Modified |
| `pkg/controllers/cache/cache.go` | Modified |
| `pkg/controllers/cache/types.go` | Modified |
| `pkg/module/metrics/types.go` | Modified |
| `pkg/managers/controllermanager/controllermanager.go` | Modified |