"""End-to-end Kubernetes Service test with real Go gateway/mock Pods."""
import json
import os
import time
from urllib.error import URLError
from urllib.request import Request, urlopen

ROOT = "http://127.0.0.1:18080"

def do(url, *, method="GET", accept=None):
    headers = {}
    if accept:
        headers["Accept"] = accept
    req = Request(ROOT + url, data=b'{"prompt":"synthetic"}' if method == "POST" else None,
                  headers=headers, method=method)
    with urlopen(req, timeout=5) as response:
        return response.status, response.headers.get("Content-Type", ""), response.read()

def main():
    deadline = time.monotonic() + 45
    while time.monotonic() < deadline:
        try:
            if do("/health")[0] == 200:
                break
        except (URLError, OSError, TimeoutError):
            pass
        time.sleep(.4)
    else:
        raise RuntimeError("Kind gateway port-forward unavailable")
    from urllib.error import HTTPError
    statuses = []
    for _ in range(18):
        try:
            status, _, _ = do("/infer", method="POST")
        except HTTPError as exc:
            status = exc.code
            exc.close()
        statuses.append(status)
    if statuses.count(503) != 3 or statuses[6:] != [200] * 12:
        raise AssertionError(f"circuit breaker failed to isolate unhealthy backend: {statuses}")
    _, typ, streamed = do("/infer", method="POST", accept="text/event-stream")
    if "text/event-stream" not in typ or streamed.count(b"data:") != 3:
        raise AssertionError("SSE not forwarded by Kubernetes-hosted gateway")
    _, _, body = do("/metrics")
    metrics = json.loads(body)
    if metrics.get("downstream_failures", 0) < 3 or metrics.get("retry_attempts") != 0:
        raise AssertionError(f"unexpected gateway metrics: {metrics}")
    report = {
        "test": "real disposable Kubernetes Pods, ClusterIP, circuit breaker, SSE",
        "requests": len(statuses),
        "backend_503": statuses.count(503),
        "backend_200": statuses.count(200),
        "synthetic_sse_events": streamed.count(b"data:"),
        "gateway_metrics": metrics,
        "production_api_calls": 0,
    }
    print(json.dumps(report, indent=2))
    with open("kubernetes-gateway-report.json", "w", encoding="utf-8") as file:
        json.dump(report, file, indent=2)

if __name__ == "__main__":
    main()
