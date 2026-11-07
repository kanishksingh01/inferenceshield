# InferenceShield architecture and progress

## P0: Working lab gateway

Synthetic HTTP client → InferenceShield gateway → bounded concurrency + per-backend deadline → mock inference backends.

The Go server exposes POST /infer, GET /health and GET /metrics. Requests are routed round-robin to local synthetic backends. Mock backends can return healthy responses, 503 failures or slow responses. The gateway does not need credentials or access to real model APIs.

## P1: Per-backend failure isolation

A circuit breaker tracks consecutive transport, HTTP 429, and HTTP 5xx failures. After the failure threshold, an individual backend's circuit opens for a cooldown period; the router skips it and tries another eligible backend. After the cooldown it permits one half-open probe. A successful probe closes the breaker; another failure opens it again.

See [CIRCUIT-BREAKER.md](CIRCUIT-BREAKER.md) for limitations and test behavior.

## Future work

Explicit retry budgets (no unbounded automatic retry), deadline propagation, rate-aware routing, backpressure, structured tracing and multi-replica policy coordination. Controlled benchmarks must demonstrate reduced amplification and tail-latency impact before claims of reliability improvements.

## Security

This is an experimental local lab, not a production inference gateway. It lacks user authentication, request-size enforcement, TLS identity policy, prompt redaction, provider billing safety, distributed circuit state and tenant isolation. Never send private user prompts or production model credentials through it. Compose publishes only loopback gateway port.
