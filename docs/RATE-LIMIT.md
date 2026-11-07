# Optional bounded request admission (lab)

The gateway now supports an optional process-global token bucket in addition to the existing maximum in-flight semaphore.

- Configure **both** `RATE_LIMIT_RPS` (finite positive requests/s) and `RATE_LIMIT_BURST` (1..100000).
- Example: `RATE_LIMIT_RPS=20 RATE_LIMIT_BURST=40`.
- Without both settings, rate limiting remains disabled for backward-compatible lab behavior.
- Requests denied by the bucket receive HTTP 429, `Retry-After: 1`, and increment `inferenceshield_rate_limit_rejections_total`.
- Health and metrics are not rate-limited; when `AUTH_TOKEN` is configured, authentication still applies to all endpoints except health.

A process-global bucket does not provide fair per-tenant quotas, billing protection, or shared rate control across gateway replicas. Do not expose the synthetic gateway publicly or consider this alone adequate inference-provider protection.
