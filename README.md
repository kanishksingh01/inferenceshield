# InferenceShield

**Inference API resilience gateway featuring admission control, circuit breakers, bounded retries, and experimental SSE.** Independent cloud portfolio project; not affiliated with Anthropic. Private during development.

**Engineering case study:** [Problem, design, measured evidence and tradeoffs](docs/CASE-STUDY.md).

**Start:** [Full end-to-end guide](docs/END-TO-END.md) · [Release criteria](docs/RELEASE-STATUS.md).

The Go gateway routes synthetic model requests to HTTP mock backends with concurrency caps, deadlines, per-backend circuit breakers, and opt-in retries controlled by a global budget. A local Docker Compose integration verifies a failing backend can be isolated without repeated amplification. Synthetic SSE streaming is validated in both Docker Compose and real Kind Pods.

**Kubernetes demonstration:** [Go gateway plus two mock model backends in real Kind, 15/18 healthy responses, three 503s, three SSE events, zero retries](https://github.com/kanishksingh01/inferenceshield/actions/runs/42426432596).


**Additional safeguards:** Optional token-bucket rate admission and bearer auth passed [live Compose CI](https://github.com/kanishksingh01/inferenceshield/actions/runs/42426689606); separate bounded SSE lifetime and caller-cancellation regression tests passed [Go CI](https://github.com/kanishksingh01/inferenceshield/actions/runs/42427835414) and [Kind](https://github.com/kanishksingh01/inferenceshield/actions/runs/42427835525).

**Current status:** Functional synthetic-inference lab; optional lab bearer-token authentication but no production TLS/tenant isolation, model-provider credentials or billing support. AI incident-investigation features deferred.

[Go CI](.github/workflows/ci.yml) · [Compose tests](.github/workflows/compose-integration.yml) · [Circuit breaker](docs/CIRCUIT-BREAKER.md) · [Retries](docs/RETRIES.md)

[Kind deployment](.github/workflows/kind-integration.yml) · [Go backend lab manifests](deploy/kubernetes.yaml) · [Optional authentication](docs/AUTHENTICATION.md).
