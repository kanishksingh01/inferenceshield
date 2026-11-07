# Engineering case study — InferenceShield

## Problem
Model-serving APIs can become slow or return 429/5xx errors. A naive intermediary can worsen incidents by retrying blindly, admitting too many concurrent requests, and repeatedly contacting unhealthy backends.

## Approach
I built an independent Go HTTP gateway and synthetic model backends. It enforces bounded in-flight concurrency, request deadlines, per-backend circuit breakers, cooldown and half-open probes. POST retries are OFF by default; opt-in retries require an Idempotency-Key, have per-request limits and consume a process-wide budget. The gateway also has JSON/Prometheus counters, experimental SSE pass-through, a 1 MiB body limit and optional Bearer authentication.

## Reproducible evidence
- Docker Compose runs one healthy and one HTTP-503 backend. After three failing backend requests, the circuit opens and healthy traffic is served through the surviving backend. Synthetic SSE events are forwarded through the live gateway.
- An independent overload experiment sent **40 concurrent** synthetic requests to a backend delayed 2 seconds, behind a **1.5-second** gateway deadline and a **16-request** in-flight cap.
- The run produced **24 HTTP 429 admission rejections** and **16 HTTP 502 timeout responses**. p95 observed latency was **~1,503 ms**; **0 retry attempts** were recorded. This was one synthetic CI measurement, not a statistically robust SLO.
- Go unit tests run with the race detector and verify retry-budget constraints, identical idempotency keys across attempts, breaker recovery, streaming behavior and optional authentication.

**Proof:** [Container-based circuit/SSE and overload benchmark](https://github.com/kanishksingh01/inferenceshield/actions/runs/42424682130) · [Latest authentication tests](https://github.com/kanishksingh01/inferenceshield/actions/runs/42425000117).

## Engineering decisions
- Rejected or unhealthy backends do not cause unbounded retry storms.
- Never automatically retry HTTP 429 or SSE streams.
- A single gateway instance uses atomic counters to cap retries relative to primary request volume.
- Client authorization can be configured using a constant-time Bearer check; the lab defaults to loopback-only and explicitly warns when authentication is disabled.
- Streaming forwards chunks as they arrive, but is bounded in size and total request duration.

## Important limitations
This is a synthetic inference-protocol laboratory, not a drop-in secure OpenAI/Anthropic proxy. It lacks TLS termination, tenant identity, key rotation, distributed retry/circuit state, production model schemas, robust idle/streaming semantics, usage billing controls and representative high-scale testing. No private prompts or billable API credentials are used.

## What would make this complete
Expand real protocol conformance, cancellation and SSE failure testing; add secure TLS identity and per-tenant policies; run realistic repeated latency/retry-amplification benchmarks and distributed gateway experiments. See [release gates](RELEASE-STATUS.md) and [full walkthrough](END-TO-END.md).
