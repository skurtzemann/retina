# Flow Debug Logs - Changelog

Date: 2026-01-28  
Commit: `a7f849ed`  
Author: Sébastien Kurtzemann

## Feature

Added flow debug logging capability in retina-agent

### Changes
- `pkg/config/config.go`: Added EnableFlowDebugLog config option
- `pkg/plugin/packetparser/packetparser_linux.go`: Added flow logging when enabled

### Description
New configuration option `enableFlowDebugLog to enable detailed flow logging in **retina-agent**. When enabled, flows are logged at INFO level with the following fields:
- `src_ip`, `src_port` - Source IP address and port
- `dst_ip`, `dst_port` - Destination IP address and port
- `proto` - Protocol (TCP/UDP)
- `dir` - Traffic direction (EGRESS/INGRESS)
- `verdict` - Flow verdict (FORWARDED/DROPPED)
- `is_reply - Reply flag 

### Configuration
`enableFlowDebugLog: true`

### Example output
```
2026-01-28T15:23:29Z    INFO    flow    {"src_ip": "10.244.0.5", "src_port": "54321", "dst_ip": "10.244.0.3", "dst_port": "80", "proto": "TCP", "dir": "EGRESS", "verdict": "FORWARDED", "is_reply": false}
```
