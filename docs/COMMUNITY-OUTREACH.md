# InferenceShield — Community outreach kit (draft, not posted)

**Target communities:** [Envoy Gateway GitHub Discussions](https://github.com/envoyproxy/gateway/discussions) ("Show and tell" if permitted or a design question in "General"), [Envoy community Discussions](https://github.com/envoyproxy/envoy/discussions/), and [CNCF AI technical community](https://contribute.cncf.io/community/tcgs/) when the topic relates to model-serving resilience.

## Draft title
Inference gateway reliability: circuit state, bounded retries, SSE timeouts, and caller cancellations

## Draft
I'm prototyping InferenceShield, a small Go inference gateway tested against synthetic model backends in Docker Compose and disposable Kind. It enforces an in-flight request cap, per-backend circuit breakers, opt-in idempotency-key-gated bounded retries, and a process-local rate limiter. SSE streams have a separate bounded lifetime, and caller cancellation does not count as an upstream-health failure.

I built it as a reliability experiment—not as an Envoy replacement and not as a production Claude/LLM proxy. Its main open problems are distributed circuit/retry budgets, idle timeouts on slow or stalled streams, and differences between HTTP success and actual model-serving completion.

Which metrics and negative test cases would you consider essential before calling a multi-replica inference gateway robust? I'd especially welcome references to existing Envoy Gateway policies so I can compare with, rather than reinvent, mature mechanisms.

I can share a focused lab demo or join a technical discussion once the public-safe reproduction is ready.

## Relevant upstream discussions (for technical comparison, not claims we copied issues)
- [Envoy Gateway Discussions](https://github.com/envoyproxy/gateway/discussions)
- [Envoy Gateway retry idle timeout proposal](https://github.com/envoyproxy/gateway/issues/10540)
- [Envoy Gateway per-Gateway rate limit backends](https://github.com/envoyproxy/gateway/issues/10520)

See [our end-to-end guide](END-TO-END.md), [SSE timeouts](STREAM-TIMEOUT.md) and [release constraints](RELEASE-STATUS.md).
