# InferenceShield Prometheus and JSON telemetry

The gateway exposes read-only metrics:
- `GET /metrics`: JSON counters for lab exploration.
- `GET /prometheus`: Prometheus text exposition, one counter per line.

Metrics cover accepted gateway requests, admission rejections, downstream failures, circuit-open rejections, actual retry attempts and retry-budget denials.

**Limitations:** This is process-local counter telemetry only. It does not yet include request latency histograms, per-backend labels, OpenTelemetry traces, token usage, streaming throughput, or distributed gateway metrics. Avoid placing secrets, prompts, or tenant identifiers in metric labels; none are emitted by this implementation.
