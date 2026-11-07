"""Bounded concurrency and timeout test using the real slow mock backend."""
import json
import math
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

ROOT = "http://127.0.0.1:8082"

def call(_):
    started = time.perf_counter()
    req = Request(ROOT + "/infer", data=b'{"prompt":"synthetic slow test"}', method="POST")
    try:
        with urlopen(req, timeout=5) as response:
            status = response.status
            response.read()
    except HTTPError as err:
        status = err.code
        err.read()
    except (URLError, OSError, TimeoutError):
        status = 0
    return status, (time.perf_counter()-started)*1000

def quantile(samples, percentile):
    ordered = sorted(samples)
    return round(ordered[max(0, math.ceil(len(ordered)*percentile/100)-1)], 2)

def main():
    # Readiness is a local synthetic gateway health endpoint.
    end = time.monotonic() + 45
    while time.monotonic() < end:
        try:
            with urlopen(ROOT + "/health", timeout=2) as r:
                if r.status == 200: break
        except (URLError, OSError, TimeoutError):
            pass
        time.sleep(.4)
    else:
        raise RuntimeError("slow gateway not ready")
    started = time.perf_counter()
    with ThreadPoolExecutor(max_workers=40) as pool:
        results = [f.result() for f in as_completed([pool.submit(call, i) for i in range(40)])]
    elapsed = time.perf_counter() - started
    codes = [code for code, duration in results]
    durations = [duration for code, duration in results]
    with urlopen(ROOT + "/metrics", timeout=3) as response:
        metrics = json.load(response)
    if 429 not in codes or 502 not in codes:
        raise AssertionError(f"expected both admission rejection and upstream timeout: {codes}")
    if set(codes) != {429, 502}:
        raise AssertionError(f"unexpected response categories: {codes}")
    if metrics["over_capacity"] < 1 or metrics["downstream_failures"] < 1:
        raise AssertionError(f"expected bounded admission and timeout counters: {metrics}")
    if metrics["retry_attempts"] != 0:
        raise AssertionError("stress scenario must not amplify retries")
    report = {
        "at_utc": datetime.now(timezone.utc).isoformat(),
        "scenario": "40 concurrent requests to a 2-second mock backend behind a 1.5-second gateway deadline; max 16 in-flight",
        "requests": len(results), "http_429": codes.count(429), "http_502": codes.count(502),
        "p50_ms": quantile(durations, 50), "p95_ms": quantile(durations, 95),
        "p99_ms": quantile(durations, 99), "elapsed_seconds": round(elapsed, 3),
        "gateway_counters": metrics,
        "limitations": "single CI run, synthetic backend, not a production latency SLO",
    }
    print(json.dumps(report, indent=2))
    with open("stress-report.json", "w", encoding="utf-8") as handle:
        json.dump(report, handle, indent=2)

if __name__ == "__main__":
    main()
