# Retina Architecture Analysis

This folder contains architectural documentation for the Retina project.

## Documents

| File | Description |
|------|-------------|
| [01-deployment-modes.md](./01-deployment-modes.md) | Overview of control planes (Standard vs Hubble) and metric modes (Basic, Advanced Remote, Advanced Local) |
| [02-standard-control-plane.md](./02-standard-control-plane.md) | Detailed architecture of the Standard Control Plane, including components like Enricher, Cache, Metrics Module |
| [02-standard-control-plane-diagram.mmd](./02-standard-control-plane-diagram.mmd) | Mermaid diagram of Standard Control Plane data flow |
| [03-data-plane-overview.md](./03-data-plane-overview.md) | Deep dive into the Data Plane, eBPF architecture, plugins, and performance considerations |
| [03-data-plane-overview-diagram.mmd](./03-data-plane-overview-diagram.mmd) | Mermaid diagram of Data Plane architecture |
| [04-hubble-control-plane.md](./04-hubble-control-plane.md) | Detailed architecture of the Hubble Control Plane, including Monitor Agent, Hubble Observer, and Hubble CLI/UI |
| [04-hubble-control-plane-diagram.mmd](./04-hubble-control-plane-diagram.mmd) | Mermaid diagram of Hubble Control Plane data flow |

## Quick Reference

### Deployment Modes

- **Control Planes**: Standard (default, supports Captures) or Hubble
- **Metric Modes** (Standard only):
  - Basic: Node-level aggregation, low cardinality
  - Advanced Remote: Pod-level with source/dest pairs, full context
  - Advanced Local: Pod-level only, scaled context

### Control Plane Comparison

| Aspect | Standard | Hubble |
|--------|----------|--------|
| **Platforms** | Linux, Windows | Linux only |
| **Data Path** | Enricher → Metrics Module | Monitor Agent → Hubble Observer |
| **Metrics Prefix** | `networkobservability_*` | `hubble_*` |
| **Capture CRD** | Supported | Not supported |
| **CLI** | kubectl retina | Hubble CLI |
| **UI** | Grafana | Hubble UI + Grafana |

### Architecture Layers

```
┌─────────────────────────────────────┐
│         CONTROL PLANE               │
│  (Standard: Enricher → Metrics)     │
│  (Hubble: Observer → Hubble UI)     │
├─────────────────────────────────────┤
│          DATA PLANE                  │
│  (eBPF plugins → flow objects)      │
├─────────────────────────────────────┤
│           KERNEL                     │
│  (tc, kprobes, eBPF maps, perf)     │
└─────────────────────────────────────┘
```