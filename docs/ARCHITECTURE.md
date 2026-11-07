# InferenceShield P0 architecture

Synthetic HTTP client → InferenceShield gateway → bounded concurrency + deadline → mock inference backend.

P0 implements HTTP POST /infer forwarding, round-robin choice, an in-flight request cap, a per-upstream deadline, and JSON counters at /metrics. Backends are lab services returning healthy, 503, or slow responses. All demonstrations use synthetic traffic and fake model replies.

No retry logic or circuit breakers yet. These are P1/P2 tasks with explicit retry budgets, jitter, health scoring and failure isolation. No API keys or real model providers are required.

The gateway currently has no production authentication, request-body size limiter, TLS policy, prompt/data redaction, or provider rate-limit integration. It must remain in an isolated lab and should not be exposed to the public internet.
