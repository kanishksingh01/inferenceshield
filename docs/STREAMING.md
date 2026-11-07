# Synthetic Server-Sent Events support

InferenceShield now supports an experimental SSE path: when clients send `Accept: text/event-stream`, the gateway forwards the header, relays response chunks as they arrive, and explicitly flushes to the client. The test mock produces three `data:` events, with no inference provider credentials.

Safety properties: streaming requests never retry, admission still enforces max concurrency, upstream requests use a single timeout, and the total streamed response is capped to 1 MiB. If an upstream stream fails **after headers have been sent**, the proxy can close early but cannot rewrite the already-sent HTTP status. Counters report started and failed streams.

**Limitations:** This is basic SSE pass-through only, not a complete OpenAI/Anthropic message protocol. It does not implement heartbeat parsing, token accounting, disconnect semantics, idle timeouts independent of total duration, event framing validation, resumable streams, authentication or multi-tenant policy. Production model traffic is not appropriate until those requirements are validated.
