# InferenceShield

**Inference API resilience gateway featuring admission control, circuit breakers, bounded retries, and experimental SSE.** Independent cloud portfolio project; not affiliated with Anthropic. Private during development.

**Engineering case study:** [Problem, design, measured evidence and tradeoffs](docs/CASE-STUDY.md).\n\n**Start:** [Full end-to-end guide](docs/END-TO-END.md) · [Release criteria](docs/RELEASE-STATUS.md).

The Go gateway routes synthetic model requests to HTTP mock backends with concurrency caps, deadlines, per-backend circuit breakers, and opt-in retries controlled by a global budget. A local Docker Compose integration verifies a failing backend can be isolated without repeated amplification. Experimental SSE streaming is being validated.

**Current status:** Functional synthetic-inference lab; no production credentials, authentication or billing support. AI incident-investigation features deferred.

[Go CI](.github/workflows/ci.yml) · [Compose tests](.github/workflows/compose-integration.yml) · [Circuit breaker](docs/CIRCUIT-BREAKER.md) · [Retries](docs/RETRIES.md)
