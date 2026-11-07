"""Check real Docker Compose auth and burst limiter using synthetic requests."""
import json
import time
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

ROOT = "http://127.0.0.1:8083"
KEY = "synthetic-test-token"

def request(path, *, method="GET", authenticated=False):
    headers = {"Content-Type": "application/json"}
    if authenticated:
        headers["Authorization"] = f"Bearer {KEY}"
    req = Request(ROOT + path, method=method, headers=headers,
                  data=b'{"prompt":"synthetic"}' if method == "POST" else None)
    try:
        with urlopen(req, timeout=5) as r:
            return r.status, r.read()
    except HTTPError as e:
        return e.code, e.read()

def main():
    until = time.monotonic() + 40
    while time.monotonic() < until:
        try:
            if request("/health")[0] == 200:
                break
        except (OSError, URLError):
            pass
        time.sleep(.3)
    else:
        raise AssertionError("rate-limited gateway failed readiness")
    if request("/infer", method="POST")[0] != 401:
        raise AssertionError("unauthenticated inference must be denied")
    results = [request("/infer", method="POST", authenticated=True)[0] for _ in range(3)]
    if results != [200, 200, 429]:
        raise AssertionError(f"unexpected burst rate-limit behavior: {results}")
    code, body = request("/metrics", authenticated=True)
    if code != 200:
        raise AssertionError("authenticated metrics unavailable")
    metrics = json.loads(body)
    if metrics["rate_limit_rejections"] != 1:
        raise AssertionError(f"rate limiter counter mismatch: {metrics}")
    if request("/metrics")[0] != 401:
        raise AssertionError("unauthenticated metrics must be denied")
    report = {
        "scope": "synthetic localhost Docker Compose gateway",
        "auth_denied": True,
        "burst_statuses": results,
        "metrics_authenticated": True,
        "rate_limit_rejections": metrics["rate_limit_rejections"],
        "note": "single-process rate limiter, not a tenant quota or production security control"
    }
    print(json.dumps(report, indent=2))
    with open("rate-limit-report.json", "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2)

if __name__ == "__main__":
    main()
