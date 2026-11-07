# InferenceShield

**Resilience gateway for model inference APIs** · Independent portfolio project · Private during development

**Start here:** [Read the complete end-to-end walkthrough](docs/END-TO-END.md).

The walkthrough explains request routing, circuit-breaker states, synthetic backend failure demonstrations, relevant metrics and tests, remaining safety gaps, and next milestones.

**Verified:** Go tests/race detector and container builds for HTTP forwarding, concurrency limits, deadlines, and per-backend circuit breakers. **Not yet:** retries, streaming, real provider integration, or production security.

[Go and container CI](.github/workflows/ci.yml) · [Architecture](docs/ARCHITECTURE.md) · [Circuit-breaker details](docs/CIRCUIT-BREAKER.md)

Only synthetic model responses are used; no production API keys or private prompts.
