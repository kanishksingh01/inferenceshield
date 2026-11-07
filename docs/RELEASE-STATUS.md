# InferenceShield — Delivery and release gates

## Implemented and validated
- Go HTTP gateway with bounded in-flight requests, per-upstream timeouts, body caps and synthetic backends.
- Per-backend circuit breakers with cooldown and half-open recovery probes.
- Opt-in bounded POST retries requiring an Idempotency-Key, per-request max attempts, and a process-wide retry budget. Retries default OFF.
- JSON and Prometheus counters and containerized failure-isolation tests.
- Basic SSE pass-through (streaming tests **under CI validation**).

## Still required before claiming completion
- Verify live SSE container integration; then add valid provider-independent streaming semantics, cancellation and idle-timeout tests.
- Load-test p95/p99 latency, successful response rate and retry-amplification factor under sustained 429/5xx/slow failures.
- Add authentication, TLS identity policy, prompt/data redaction, rate limits, safe observability and tenant isolation.
- Validate distributed circuit/retry budgets across multiple gateway replicas; no real model API billing tests in current lab.

## Current release classification
**Experimental local inference resilience gateway**, not a secure hosted provider proxy. See [end-to-end walkthrough](END-TO-END.md).