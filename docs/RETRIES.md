# Bounded retries and admission safety

Retries of POST inference calls are **disabled by default**. The gateway allows opt-in retries only when:
1. `NewWithPolicies` sets `maxRetries` to 1 or 2;
2. the caller supplies a non-empty Idempotency-Key (at most 128 characters);
3. at least two backend endpoints are configured;
4. the failed response is 502, 503, 504 or a transport error (never 429);
5. there is a distinct eligible backend;
6. the process-wide retry budget has capacity; and
7. the end-to-end request deadline has not expired.

A single incoming request cannot exceed the per-request retry count or timeout, and the **global** budget permits at most one extra backend attempt per `budgetEvery` accepted requests. State is atomic and shared between concurrent handlers in one gateway process.

Request and response bodies are each limited to 1 MiB; this simple reference gateway buffers bodies and **does not support streaming**. There is no automatic backoff, replay protection or distributed retry budget. An Idempotency-Key does not guarantee that the downstream provider actually deduplicates requests. This is strictly synthetic-lab functionality; do not proxy real personal prompts or billable model calls.

Metrics: `GET /metrics` exposes accepted requests, capacity rejects, downstream failures, circuit-open rejects, actual retry attempts and retry-budget refusals.

Benchmarking against a naive retry baseline and structured OpenTelemetry instrumentation remain outstanding.

The gateway forwards the supplied Idempotency-Key unchanged to each attempted backend. This enables—but does not enforce—deduplication if the downstream service implements idempotency. Tests verify the same key reaches the first and retry attempts.
