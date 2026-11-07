# InferenceShield — End-to-end walkthrough

> **Status:** Working Go HTTP gateway, mock model backends, concurrency limits, request timeout and per-backend circuit breakers. Tested through Go unit/race tests and container builds; no production inference providers. Independent Anthropic-aligned project; not affiliated with Anthropic.

## 1. The problem

When a model-serving API returns errors or slows down, naive clients keep sending requests and may amplify an outage. A client-facing gateway should cap its own concurrency, enforce deadlines, and avoid repeatedly selecting a backend that is demonstrably unhealthy.

**What we built:** A small Go HTTP gateway with synthetic inference servers and failure-isolating circuit breakers. It implements optional, strictly bounded retries; they are disabled by default and require a forwarded Idempotency-Key.

## 2. End-to-end request flow

```text
  Client POST /infer
           |
     Gateway HTTP router
           |
       Admission cap (max in-flight)
           |
    Round-robin backend selection
           |
   Skip OPEN circuit / allow one HALF-OPEN probe
           |
   Downstream request with configured timeout
           |
   2xx success -> reset failure streak
   429 / 5xx / transport failure -> increment streak
           |
   Repeated failures -> OPEN circuit for cooldown
           |
    Return upstream response or gateway error
           |
       JSON counters at GET /metrics
```

## 3. Follow a failure example

1. Start two mock inference backends, one healthy and one configured to return HTTP 503.
2. Start the gateway pointing at both. The default implementation selects backends round-robin.
3. A failing backend increments its own failure count. After the configured number of consecutive failures, its circuit opens.
4. While open, the gateway skips that backend and tries another *eligible* backend. If none are eligible, it returns a 503 instead of hammering them.
5. After the cooldown, one request is permitted as a half-open probe. A healthy response closes the circuit; another error reopens it.
6. A separate global admission cap returns 429 when too many requests are already in flight. Requests have configured HTTP-client deadlines.

The default circuit policy has a threshold of three consecutive failures and a five-second cooldown. These are defaults for the lab, **not** researched best settings for a real provider. The test code exercises an alternate short cooldown to verify recovery.

## 4. How to review or run

- [Go tests + race detector + Docker Compose build CI](../.github/workflows/ci.yml)
- [Circuit-breaker implementation CI](https://github.com/kanishksingh01/inferenceshield/actions/runs/42399202132)

Optional, from repo root with Go 1.23 and Docker/Compose:

```bash
go test -race ./...
docker compose up --build
```

In a separate terminal, send a synthetic JSON request:

```bash
curl -i -X POST http://127.0.0.1:8080/infer \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"synthetic test"}'
curl http://127.0.0.1:8080/metrics
```

The Compose gateway only publishes the loopback port. It does **not** connect to Claude or other real model APIs. It supports optional Bearer authentication, but no TLS, managed identity or private-prompt protection; keep tests synthetic and local.

## Real Kubernetes and resilience benchmarks

The [Kind gateway integration](https://github.com/kanishksingh01/inferenceshield/actions/runs/42426432596) deploys the Go gateway and two synthetic inference servers as actual Kubernetes Pods, accessed through a ClusterIP Service. The gateway returns 15 healthy responses and three expected 503s from 18 requests, isolates the unhealthy backend and forwards three synthetic SSE events, with no automatic retries.

The [Compose overload experiment](https://github.com/kanishksingh01/inferenceshield/actions/runs/42424682130) sends 40 concurrent requests to a two-second backend under a 16-in-flight cap and 1.5-second deadline: 24 requests are rejected (429) and 16 time out (502), p95 ~1,503 ms, zero retries. Both are synthetic test environments—not production model-serving performance.

## 5. How to interpret the tests

The tests verify healthy forwarding, transport failure handling, bounded admission, circuit opening without repeated backend attempts, and a recovery probe after cooldown. The Go CI uses `-race` to detect concurrent state races and builds the mock/gateway container images.

**What it proves:** Basic per-process failure isolation logic and code-level concurrency behavior. **What it does not prove:** Large-scale, multi-instance reliability, real inference streaming behavior, recovery latency under realistic traffic, proper provider rate-limit semantics, or secure production deployment.

## 6. Code map

| Component | Location |
| --- | --- |
| Backend selection and breaker state | [internal/gateway/gateway.go](../internal/gateway/gateway.go) |
| Gateway executable | [cmd/inferenceshield/main.go](../cmd/inferenceshield/main.go) |
| Mock model server | [cmd/mock-backend/main.go](../cmd/mock-backend/main.go) |
| Docker service composition | [compose.yaml](../compose.yaml) |
| Tests | [internal/gateway/gateway_test.go](../internal/gateway/gateway_test.go) |
| Circuit explanation | [docs/CIRCUIT-BREAKER.md](CIRCUIT-BREAKER.md) |

## 7. Security gaps and remaining work

- Optional single Bearer authentication and a 1 MiB body cap are implemented, but TLS identity, token rotation, tenant isolation, sensitive-prompt redaction and billing controls are not.
- Basic SSE streaming and opt-in retry budgets are implemented; provider-specific stream contracts, retry jitter, queue policy and distributed circuit-state coordination are not.
- One synthetic overloaded-backend latency/admission test is complete; no statistically robust load comparison against naive client retries exists yet.

The next phase should expand bounded-retry benchmark coverage, secure streaming semantics and distributed failure-isolation tests. AI-powered incident explanation and model-aware routing remain deferred.

See [P0 issue](https://github.com/kanishksingh01/inferenceshield/issues/1) and [blueprint](https://github.com/kanishksingh01/inferenceshield/issues/6).
