# Explicitly bounded SSE lifetime

Unary HTTP requests and SSE inference responses have different timing semantics. The gateway now keeps the default unary upstream deadline at 1.5 seconds but uses a separate HTTP client with a bounded SSE request context (30 seconds by default). Thus a healthy stream that emits more tokens than fit into 1.5 seconds can finish successfully.

`STREAM_TIMEOUT_SECONDS` configures 1..600 seconds at startup; a Go test intentionally delays the second SSE event beyond the unary timeout while staying under the streaming deadline.

**Safety limits:** the stream still has a total lifetime cap, an approximately 1 MiB response cap and caller cancellation. The timeout is **not an inactivity timeout**: quiet streams consume capacity until that overall deadline. No provider-specific token framing, reconnect/resume, usage billing, or streaming authentication upgrade is implemented. These are experimental lab semantics, not a production streaming API guarantee.
