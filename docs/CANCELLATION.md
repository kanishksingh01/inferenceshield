# Caller cancellation and upstream health

InferenceShield now distinguishes the caller disconnecting from a genuine upstream transport failure. Cancellation of the **incoming HTTP request context** does not increment backend failure counters or trip circuit breakers. This matters particularly during streaming, where mobile users and browsers frequently disconnect intentionally.

A gateway-controlled timeout remains an upstream failure because the backend did not meet its configured deadline. This implementation uses the request's original context, rather than only the derived timeout context, to distinguish the two paths.

Go race tests exercise client-initiated cancellation for both normal HTTP forwarding and SSE streaming. The semantics remain local to one instance; distributed cancellation and provider billing are not part of the experimental gateway.
