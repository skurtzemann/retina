# PacketParser Plugin

**Retina** is a Kubernetes network observability platform using eBPF. The `packetparser` plugin is **required only for Advanced Standard Control Plane mode** to generate Pod-Level metrics.

## Architecture

```
eBPF Programs → perf events → packetparser plugin → Flow objects → Enricher → Metrics Module → Prometheus
```

## How It Works

### 1. eBPF Attachment (`packetparser_linux.go:415-533`)

- Attaches `clsact` qdisc to pod veth interfaces and host's default interface
- Four eBPF programs capture traffic at 4 observation points:
  - `FROM_ENDPOINT` / `TO_ENDPOINT` (pod interfaces)
  - `FROM_NETWORK` / `TO_NETWORK` (host interface)

### 2. Packet Parsing in Kernel (`_cprog/packetparser.c:118-247`)

- Parses Ethernet/IP/TCP/UDP headers
- Extracts: IPs, ports, TCP flags, sequence numbers, timestamps
- Optional: conntrack metrics, packet sampling (1/N)

### 3. Kernel-to-Userspace Transfer

- Uses `BPF_MAP_TYPE_PERF_EVENT_ARRAY` (per-CPU buffers)
- `handleEvents()` (producer) reads from perf array
- `processRecord()` workers (consumers) convert to Flow objects

### 4. Flow Enrichment (`packetparser_linux.go:558-679`)

- Converts raw packet data to Cilium `Flow` protobuf
- Adds: traffic direction, TCP flags, packet size, TCP metadata (seq/ack/tsval/tsecr)
- Writes to Enricher for Kubernetes context (pod name, namespace, workload, service)

## Metrics Modules Consuming Flows

| Module | Metrics |
|--------|---------|
| `forward.go` | `adv_forward_count`, `adv_forward_bytes` |
| `tcpflags.go` | TCP flag counters |
| `drops.go` | Dropped packet metrics |
| `tcpretrans.go` | TCP retransmission metrics |
| `latency.go` | API server latency metrics |
| `dns.go` | DNS request/response metrics |

## Key Files

| File | Purpose |
|------|---------|
| `pkg/plugin/packetparser/packetparser_linux.go` | Main plugin (734 lines) |
| `pkg/plugin/packetparser/types_linux.go` | Interfaces and types |
| `pkg/plugin/packetparser/_cprog/packetparser.c` | eBPF programs |
| `pkg/module/metrics/*.go` | Metrics modules consuming flows |

## Configuration for Advanced Mode

Requires in config:

- `EnablePodLevel: true`
- `DataAggregationLevel: Low` (attaches to host interface) or `High` (pod-level only)

## Performance Note

Uses perf event array (per-CPU buffers). On 32+ core systems under high load, users report scaling issues. Alternative: BPF ring buffers (not yet implemented).

## Capabilities Required

- `CAP_SYS_ADMIN` - Load eBPF maps and programs into kernel
- `CAP_NET_ADMIN` - Attach qdisc and filters via traffic control

## Extending Metrics

To add new metrics:

1. **Extend the eBPF packet struct** in `_cprog/packetparser.c` and `types_linux.go`
2. **Parse new fields** in the `parse()` function in `packetparser.c`
3. **Add to Flow object** in `processRecord()` in `packetparser_linux.go`
4. **Create or extend a metrics module** in `pkg/module/metrics/`