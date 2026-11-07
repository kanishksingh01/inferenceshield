# Optional client and metrics authentication

Setting `AUTH_TOKEN` on the InferenceShield process enables a constant-time Bearer token check before the gateway accepts requests to `/infer`, `/metrics` or `/prometheus`. The `/health` endpoint remains unauthenticated for readiness checks.

For example, supply a synthetic, non-sensitive token in a local development environment and send `Authorization: Bearer ...`. **Do not commit a real token** or put it in Docker Compose examples. With no token, the gateway logs a warning and runs in unauthenticated **lab-only** mode. Current Compose binds only to loopback.

This is a single shared secret, not a complete authentication/authorization system. It does not supply transport encryption, tenant isolation, key rotation, expiry, per-client quotas, streaming user identity, or secrets-manager integration. A reverse proxy with TLS and properly managed identity is required before any real production traffic.
