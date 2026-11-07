# Circuit breaker (experimental)

InferenceShield now isolates individual unhealthy backend endpoints. After three consecutive failures (HTTP 429, HTTP 5xx or transport errors) for one backend, its circuit opens for five seconds by default. Traffic routes to other eligible backends when possible. After cooldown, only one half-open probe is admitted; a successful response resets the breaker, while another failure opens it again.

`NewWithCircuitPolicy` exposes configurable failure thresholds and cooldowns for tests and future config validation. An all-open condition returns 503 rather than continuing to hammer failing services.

The circuit breaker is local to one gateway process. It is **not** a distributed health controller; no retries or automatic cross-provider fallback semantics have been implemented. There is no authentication, model validation, provider billing control or private data protection. Use this code in a synthetic lab only.

Next features: explicit retry budgets, exponential jitter, queue admission policies, telemetry and failure injection comparisons.
