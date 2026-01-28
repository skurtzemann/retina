# Retina Operator Architecture

The Retina Operator is a Kubernetes operator responsible for managing cluster resources and custom resources for the Retina observability platform. It supports two operating modes: **Standard** and **Cilium CRDs**.

## Overview

Location: `operator/cmd/`

## Entry Points

The operator provides two CLI commands:

| Command | Entry File | Description |
|---------|-----------|-------------|
| `retina-operator` | `root.go` + `standard/deployment.go` | Standard control plane mode |
| `retina-operator manage-cilium-crds` | `cilium_crds_cmd_linux.go` + `cilium-crds/*` | Cilium CRDs management mode |

---

## Standard Mode

The standard mode provides the primary Retina control plane using controller-runtime.

### Controllers

| Controller | Purpose | File |
|------------|---------|------|
| **CaptureReconciler** | Manages packet capture resources | `pkg/controllers/operator/capture` |
| **MetricsConfigurationReconciler** | Manages metrics configuration resources | `pkg/controllers/operator/metricsconfiguration` |
| **PodController** | Syncs pod events to RetinaEndpoint | `pkg/controllers/operator/pod` |
| **RetinaEndpointController** | Creates optimized endpoint objects from pods | `pkg/controllers/operator/retinaendpoint` |

### Features

- **Leader Election**: Optional HA support via Kubernetes leases (`LeaderElectionID: "16937e39.retina.sh"`)
- **Metrics Server**: Exposes metrics on configurable address (default `:8080`)
- **Health Probes**: Configurable health and readiness checks (default `:8081`)
- **CRD Installation**: Can auto-install Retina CRDs on startup
- **Telemetry**: Optional Application Insights integration
- **RetinaEndpoint**: Optimized pod metadata caching to reduce API server pressure

### Configuration

Flags:
- `--metrics-addr`: Metrics endpoint bind address (default: `:8080`)
- `--probe-addr`: Health probe bind address (default: `:8081`)
- `--enable-leader-election`: Enable HA mode
- `--config`: Config file path (default: `retina/operator-config.yaml`)

---

## Cilium CRDs Mode

The Cilium CRDs mode manages Cilium custom resources, derived from the Cilium operator codebase. It uses the Hive dependency injection framework.

### Custom Resources Managed

| Resource | Garbage Collection | Purpose |
|----------|-------------------|---------|
| **CiliumEndpoint** | EndpointGC | Removes leaked endpoints |
| **CiliumIdentity** | IdentityGC | Removes orphaned identities |

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Cilium CRDs Operator                     │
├─────────────────────────────────────────────────────────────┤
│  Hive Cell: Infrastructure                                  │
│  ├── Config (Cell)                                          │
│  ├── Telemetry                                              │
│  ├── PProf                                                  │
│  ├── Kubernetes Clientset                                   │
│  ├── Metrics (operatorMetrics.Cell)                         │
│  └── Runtime Scheme                                         │
├─────────────────────────────────────────────────────────────┤
│  Hive Cell: ControlPlane (only runs when elected leader)    │
│  ├── Leader Election (lease-based)                         │
│  ├── CRD Registration (RegisterCRDsCell)                   │
│  ├── Controller Manager                                     │
│  │   └── Endpoint Controller (Cell)                          │
│  ├── IdentityGC (Cell)                                      │
│  ├── EndpointGC (Cell)                                      │
│  └── Telemetry Heartbeat                                    │
└─────────────────────────────────────────────────────────────┘
```

### Leader Election

- **Resource Lock**: `cilium-operator-resource-lock` in `kube-system` namespace
- **HA Support**: Requires Kubernetes coordination.k8s.io/v1 (Leases) support
- **Fallback**: Non-HA mode if Leases unavailable
- **Operator ID**: Generated as `<hostname>-<random-10-char-suffix>`

### Features

- Hive-based dependency injection for modularity
- Lease-based leader election with configurable renew parameters
- CRD registration runs immediately after leader election
- Derive from Cilium operator (see `cells_linux.go:4-6`)

---

## CRD Installation

Both modes support installing CRDs:

### Standard Mode

```go
// operator/cmd/standard/deployment.go:146-157
if oconfig.InstallCRDs {
    crds, err = deploy.InstallOrUpdateCRDs(ctx, oconfig.EnableRetinaEndpoint, clientset)
    // ...
}
```

### Cilium CRDs Mode

```go
// operator/cmd/cilium-crds/cells_linux.go:186
apis.RegisterCRDsCell,  // Runs only after leader election
```

---

## Directory Structure

```
operator/cmd/
├── root.go                          # Main entry point (standard mode)
├── cilium_crds_cmd_linux.go         # Cilium CRDs mode entry
├── standard/
│   └── deployment.go                 # Standard operator deployment logic
└── cilium-crds/
    ├── root_linux.go                 # Cilium operator lifecycle & leader election
    ├── cells_linux.go                # Hive cell modules
    ├── flags.go                      # CLI flags
    ├── flags_provider.go             # Flag provider hooks
    ├── metrics.go                    # Metrics configuration
    ├── lifecycle.go                  # Leader lifecycle management
    └── zap_linux.go                  # Logging configuration
```

---

## Key Components

| Component | Purpose |
|-----------|---------|
| **Controller-runtime Manager** | Standard: Orchestrates reconcile loops |
| **Hive Framework** | Cilium CRDs: Dependency injection & lifecycle |
| **Leader Election** | Ensures single active operator in HA mode |
| **GC Controllers** | Cilium CRDs: Cleanup orphaned resources |
| **RetinaEndpoint** | Standard: Optimized pod metadata proxy |