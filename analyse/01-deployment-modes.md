# Retina Deployment Modes

Retina has two main configurations:

## 1. Control Plane Choice

| Control Plane | Description |
|--------------|-------------|
| **Standard** | Original Retina implementation. Required for Capture CRD support. |
| **Hubble** | Alternative using Hubble stack. No modes apply (different metrics). |

## 2. Metric Modes (Standard Control Plane only)

| Mode | Aggregation | Scale | Use Case |
|------|------------|-------|----------|
| **Basic** | By Node | Low cardinality (per node) | General monitoring, large clusters |
| **Advanced + Remote Context** | By Pod (source/dest pair) | High/unbounded cardinality | Deep troubleshooting, smaller clusters |
| **Advanced + Local Context** | By Pod (local only) | Moderate cardinality | Scaled troubleshooting |

**Key differences:**
- **Basic**: `cluster`, `instance` labels only
- **Advanced**: Adds `source_*/destination_*` labels (IP, namespace, pod, workload)
- **Remote Context**: Tracks both ends of every connection (full picture, higher cardinality)
- **Local Context**: Only tracks local pod (source for egress, destination for ingress) - more scalable

**Mode selection** via Helm:
```bash
# Basic
--set enabledPlugin_linux="[dropreason,packetforward,linuxutil,dns]"

# Advanced (add packetparser plugin)
--set enabledPlugin_linux="[dropreason,packetforward,linuxutil,dns,packetparser]"
```