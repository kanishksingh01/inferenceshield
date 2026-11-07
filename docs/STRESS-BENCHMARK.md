# InferenceShield synthetic overload benchmark

The optional Compose `stress` profile launches a slow mock backend (2-second responses) and a separate gateway on runner loopback port 8082. This gateway has a 1.5-second HTTP client deadline and an in-flight cap of 16. A Python harness fires 40 concurrent synthetic POST requests, records HTTP status categories and p50/p95/p99 latency, and verifies that admission rejection (429) and bounded upstream timeout (502) both occur. Retries remain disabled.

The GitHub workflow saves `stress-report.json` alongside the normal circuit-breaker/SSE demonstration. This is a **single synthetic CI benchmark**, not evidence of enterprise-grade latency or long-duration performance. Distributed replicas, retries under saturation, real provider semantics, and load distributions have not yet been tested.
