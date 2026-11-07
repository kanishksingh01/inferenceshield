"""Real Docker Compose integration for the synthetic inference gateway.

One healthy backend and one failing backend; verifies the failing backend's
circuit opens and healthy traffic continues. No model API credentials required.
"""
import json
import time
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

ROOT = "http://127.0.0.1:8080"

def request(path, method="GET", body=None):
    req = Request(ROOT + path, data=body, method=method)
    try:
        with urlopen(req, timeout=3) as resp:
            return resp.status, resp.read()
    except HTTPError as err:
        return err.code, err.read()

def main():
    stop = time.monotonic() + 45
    while time.monotonic() < stop:
        try:
            if request("/health")[0] == 200:
                break
        except (URLError, OSError, TimeoutError):
            pass
        time.sleep(0.4)
    else:
        raise RuntimeError("gateway never became healthy")
    codes = []
    for _ in range(20):
        code, _ = request("/infer", "POST", b'{"prompt":"synthetic"}')
        codes.append(code)
    code, metrics_body = request("/metrics")
    if code != 200:
        raise AssertionError("metrics unavailable")
    metrics = json.loads(metrics_body)
    # Round robin sends at most 3 failures to the bad backend. After its
    # circuit opens, the healthy backend serves all remaining requests.
    if any(code != 200 for code in codes[6:]):
        raise AssertionError(f"unhealthy backend wasn't isolated: {codes}")
    if codes.count(503) != 3:
        raise AssertionError(f"expected exactly 3 synthetic backend 503s: {codes}")
    if metrics["downstream_failures"] != 3:
        raise AssertionError(f"downstream failure count mismatch: {metrics}")
    if metrics["retry_attempts"] != 0:
        raise AssertionError("automatic retries should be disabled by default")
    report = {
        "scenario": "one healthy and one HTTP-503 mock inference backend",
        "requests": len(codes),
        "http_200": codes.count(200),
        "http_503": codes.count(503),
        "gateway_metrics": metrics,
        "all_late_requests_successful": True,
        "real_backend_calls": True,
        "model_api_requests": False,
    }
    print(json.dumps(report, indent=2))
    with open("compose-report.json", "w", encoding="utf-8") as fp:
        json.dump(report, fp, indent=2)

if __name__ == "__main__":
    main()
