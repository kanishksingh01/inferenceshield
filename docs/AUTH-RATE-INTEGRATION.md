# Authentication + burst admission integration

The Compose Gateway Integration workflow now includes a separate **localhost-only** gateway with a synthetic bearer token and a 1 request/second, burst-2 token bucket. The test verifies unauthenticated POST /infer and GET /metrics are denied, two authorized synthetic requests succeed, the third immediate request gets 429, and the authenticated metrics show one rate-limit rejection.

This is an intentionally tiny verification profile, not recommended production capacity. It does not protect a cluster with multiple replicas or provide tenant-specific quotas, WAF rules, TLS or provider billing limits.

Evidence: the `rate-limit-report.json` artifact from the Compose Gateway Integration workflow.
