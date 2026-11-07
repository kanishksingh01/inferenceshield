# InferenceShield containerized Kubernetes demonstration

A disposable Kind cluster runs three Pods: a healthy synthetic model backend, an HTTP-503 synthetic backend and a Go InferenceShield gateway. All Services are ClusterIP and all Pods use non-root identities, resource limits, no ServiceAccount token mounting and read-only root filesystems.

The Kind integration builds Go binaries, loads container images, deploys Kubernetes resources, waits for readiness and sends synthetic inference HTTP calls through a loopback port-forward to the gateway Service. It verifies failing backend circuit isolation, healthy alternative responses, absence of automatic retries, real SSE event forwarding and JSON metrics.

No model API credentials or private prompts are used. The gateway is unauthenticated in this isolated lab. Production deployment requires TLS, managed client identities, input/output data policy, billing limits and strong multi-tenant isolation.
