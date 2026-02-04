# Flow Debug Logging

## Overview

The flow debug logging feature provides detailed visibility into every network flow processed by Retina. When enabled, flows are logged at INFO level with comprehensive metadata including source/destination information, Kubernetes context, and flow verdicts.

## Configuration

| Setting | Config Key | Default | Description |
|---------|------------|---------|-------------|
| `enableFlowDebugLog` | `yaml: enableFlowDebugLog` | `false` | Enable detailed flow logging |

### Example Configuration

```yaml
enableFlowDebugLog: true
```

## Control-Plane Compatibility

| Control-Plane | Supported | Notes |
|---------------|-----------|-------|
| **STANDARD** | ✅ Full | Full pod/node enrichment via ControllerCache |
| **HUBBLE** | ⚠️ Partial | Config accepted, but pod enrichment limited (shows "external" for all sources) |

### Why Hubble Has Limited Support

Flow debug logging relies on `GlobalCache.GetPodByIP()` and `GlobalCache.GetNodeByIP()` for enrichment. This cache is only populated in the STANDARD control-plane. Hubble uses Cilium's native `ipcache.IPCache` instead, which doesn't provide the same enrichment interface.

## Integration with Cache

Flow debug logging leverages the **ControllerCache** to enrich flows with Kubernetes metadata:

- **Pod information**: Source and destination pod names, namespaces
- **Availability zones**: Zone labels from node metadata
- **Node context**: Source and destination node names

This enrichment happens through the `GetZoneByPodIP()` lookup chain, providing full visibility into pod-to-pod traffic across zones.

## Output Fields

Each logged flow includes the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `src_ip` | string | Source IP address |
| `src_port` | string | Source port number |
| `dst_ip` | string | Destination IP address |
| `dst_port` | string | Destination port number |
| `proto` | string | Protocol (TCP/UDP/ICMP) |
| `dir` | string | Traffic direction (EGRESS/INGRESS) |
| `verdict` | string | Flow verdict (FORWARDED/DROPPED) |
| `is_reply` | boolean | Reply flag |

## Example Output

```
2026-01-28T15:23:29Z    INFO    flow    {"src_ip": "10.244.0.5", "src_port": "54321", "dst_ip": "10.244.0.3", "dst_port": "80", "proto": "TCP", "dir": "EGRESS", "verdict": "FORWARDED", "is_reply": false}
```

## Use Cases

- **Debug connectivity issues**: Verify traffic is flowing between pods as expected
- **Verify pod-to-pod traffic**: Confirm traffic paths and identify misconfigurations
- **Check flow verdicts**: Understand why traffic is being dropped
- **Zone-level debugging**: Identify cross-zone traffic patterns and issues

## Performance Considerations

Enabling flow debug logging has a **moderate performance impact**:
- Every flow is logged at INFO level
- Cache lookups are performed for each flow to enrich with metadata
- Recommended for debugging sessions only, not for production monitoring

For production monitoring, use metrics instead:
- `packet_count_total` - Total flow count
- `packet_drop_total` - Dropped flow count
- `flow_process_latency` - Flow processing latency