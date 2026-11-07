# InferenceShield real-container integration

CI starts two actual Go mock inference backends with Docker Compose: one always healthy and one returning HTTP 503. The gateway listens only on the runner's loopback port.

The integration sends 20 synthetic POSTs. Expected behavior: three requests receive the failing backend's 503, then its circuit opens and subsequent requests are routed exclusively to the healthy backend. The test verifies that late requests succeed, downstream failure counters match, and the default configuration performed zero retries.

A JSON report is uploaded as `inferenceshield-compose-report`. The workflow always destroys the containers afterward.

**Not demonstrated:** real model APIs, token streaming, downstream billing, distributed circuit states or handling high concurrent production traffic. The test is intended to prove isolated gateway behavior with actual HTTP backends, not deployment readiness.
