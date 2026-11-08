package gateway

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

const maxBodyBytes = 1 << 20

type backendState struct {
    url *url.URL
    mu sync.Mutex
    failures int
    openUntil time.Time
    probing bool
}

func (b *backendState) eligible(now time.Time) bool {
    b.mu.Lock()
    defer b.mu.Unlock()
    if now.Before(b.openUntil) { return false }
    if !b.openUntil.IsZero() {
        if b.probing { return false }
        b.probing = true
    }
    return true
}

func (b *backendState) finish(success bool, limit int, cooldown time.Duration) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if success {
        b.failures = 0
        b.probing = false
        b.openUntil = time.Time{}
        return
    }
    b.failures++
    if b.failures >= limit || b.probing {
        b.failures = 0
        b.openUntil = time.Now().Add(cooldown)
    }
    b.probing = false
}

// releaseProbe clears an in-flight half-open reservation without declaring
// upstream success or failure (for example when the caller disconnects).
func (b *backendState) releaseProbe() {
    b.mu.Lock()
    b.probing = false
    b.mu.Unlock()
}

type Gateway struct {
    backends []*backendState
    client *http.Client
    streamClient *http.Client
    streamTimeout time.Duration
    permits chan struct{}
    threshold int
    cooldown time.Duration
    timeout time.Duration
    maxRetries int
    budgetEvery uint64
    next atomic.Uint64
    accepted atomic.Uint64
    rejected atomic.Uint64
    downstreamFailures atomic.Uint64
    circuitOpenRejections atomic.Uint64
    retryAttempts atomic.Uint64
    retryBudgetDenied atomic.Uint64
    streamsStarted atomic.Uint64
    streamFailures atomic.Uint64
    rateLimiterMu sync.RWMutex
    rateLimiter *tokenBucket
    rateLimitRejected atomic.Uint64
}

func New(targets []string, maxInFlight int, timeout time.Duration) (*Gateway, error) {
    return NewWithCircuitPolicy(targets, maxInFlight, timeout, 3, 5*time.Second)
}

func NewWithCircuitPolicy(targets []string, maxInFlight int, timeout time.Duration,
    failureThreshold int, cooldown time.Duration) (*Gateway, error) {
    return NewWithPolicies(targets, maxInFlight, timeout, failureThreshold, cooldown, 0, 5)
}

// Retries are OFF unless configured. POST replay additionally requires a bounded
// Idempotency-Key header. This is a lab API; downstream idempotency remains the
// responsibility of the backend, not something a header alone guarantees.
func NewWithPolicies(targets []string, maxInFlight int, timeout time.Duration,
    failureThreshold int, cooldown time.Duration, maxRetries int, budgetEvery uint64) (*Gateway, error) {
    if maxInFlight < 1 || timeout <= 0 || len(targets) == 0 ||
        failureThreshold < 1 || cooldown <= 0 || maxRetries < 0 || maxRetries > 2 || budgetEvery < 1 {
        return nil, &ConfigError{"invalid backends, capacity, timeout or retry/circuit policy"}
    }
    g := &Gateway{permits: make(chan struct{}, maxInFlight),
        threshold: failureThreshold, cooldown: cooldown, timeout: timeout,
        maxRetries: maxRetries, budgetEvery: budgetEvery,
        client: &http.Client{Timeout: timeout,
            CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
                return http.ErrUseLastResponse
            }},
        streamClient: &http.Client{
            // Streaming has a separately bounded context lifetime; a client-wide
            // short timeout would terminate even a healthy SSE response body.
            CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
                return http.ErrUseLastResponse
            }},
        streamTimeout: 30*time.Second}
    for _, raw := range targets {
        u, err := url.Parse(strings.TrimSpace(raw))
        if err != nil || u == nil || u.Host == "" ||
            (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
            return nil, &ConfigError{"invalid backend URL"}
        }
        g.backends = append(g.backends, &backendState{url: u})
    }
    return g, nil
}

type ConfigError struct{ Message string }
func (e *ConfigError) Error() string { return e.Message }

func (g *Gateway) selectBackendExcept(previous *backendState) *backendState {
    start := g.next.Add(1) - 1
    for offset := 0; offset < len(g.backends); offset++ {
        candidate := g.backends[(start+uint64(offset))%uint64(len(g.backends))]
        if candidate != previous && candidate.eligible(time.Now()) {
            return candidate
        }
    }
    return nil
}

// A global concurrency-safe budget: at most one retry per budgetEvery accepted
// primary requests, in addition to the per-request maxRetries bound.
func (g *Gateway) acquireRetryToken() bool {
    for {
        used := g.retryAttempts.Load()
        if used >= g.accepted.Load()/g.budgetEvery {
            g.retryBudgetDenied.Add(1)
            return false
        }
        if g.retryAttempts.CompareAndSwap(used, used+1) { return true }
    }
}

type attemptResult struct {
    status int
    body []byte
    contentType string
    retryable bool
}

func (g *Gateway) attempt(ctx context.Context, callerCtx context.Context, backend *backendState, body []byte, contentType string, idempotencyKey string) attemptResult {
    target := *backend.url
    target.Path = "/infer"
    target.RawQuery = ""
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(body))
    if err != nil {
        backend.finish(false, g.threshold, g.cooldown)
        g.downstreamFailures.Add(1)
        return attemptResult{status: 502, body: []byte("upstream request unavailable"), retryable: true}
    }
    req.Header.Set("Content-Type", contentType)
    if idempotencyKey != "" { req.Header.Set("Idempotency-Key", idempotencyKey) }
    resp, err := g.client.Do(req)
    if err != nil {
        if callerCtx.Err() != nil {
            backend.releaseProbe()
            return attemptResult{status: 502, body: []byte("client cancelled"), retryable: false}
        }
        backend.finish(false, g.threshold, g.cooldown)
        g.downstreamFailures.Add(1)
        return attemptResult{status: 502, body: []byte("upstream request failed"), retryable: true}
    }
    defer resp.Body.Close()
    data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
    if readErr != nil && callerCtx.Err() != nil {
        backend.releaseProbe()
        return attemptResult{status: 502, body: []byte("client cancelled"), retryable: false}
    }
    if readErr != nil || len(data) > maxBodyBytes {
        backend.finish(false, g.threshold, g.cooldown)
        g.downstreamFailures.Add(1)
        return attemptResult{status: 502, body: []byte("invalid or oversized upstream response"), retryable: false}
    }
    unhealthy := resp.StatusCode == 429 || resp.StatusCode >= 500
    if unhealthy { g.downstreamFailures.Add(1) }
    backend.finish(!unhealthy, g.threshold, g.cooldown)
    // Never retry rate-limit responses; these usually indicate quota/backpressure.
    retryable := resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504
    return attemptResult{status: resp.StatusCode, body: data,
        contentType: resp.Header.Get("Content-Type"), retryable: retryable}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/health" {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
        return
    }
    if r.URL.Path == "/prometheus" {
        w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        values := []struct {
            name string
            val uint64
        }{
            {"inferenceshield_requests_accepted_total", g.accepted.Load()},
            {"inferenceshield_admission_rejected_total", g.rejected.Load()},
            {"inferenceshield_upstream_failures_total", g.downstreamFailures.Load()},
            {"inferenceshield_circuit_rejections_total", g.circuitOpenRejections.Load()},
            {"inferenceshield_retry_attempts_total", g.retryAttempts.Load()},
            {"inferenceshield_retry_budget_denied_total", g.retryBudgetDenied.Load()},
            {"inferenceshield_stream_requests_total", g.streamsStarted.Load()},
            {"inferenceshield_stream_failures_total", g.streamFailures.Load()},
            {"inferenceshield_rate_limit_rejections_total", g.rateLimitRejected.Load()},
        }
        for _, metric := range values {
            _, _ = fmt.Fprintf(w, "# TYPE %s counter\n%s %d\n", metric.name, metric.name, metric.val)
        }
        return
    }
    if r.URL.Path == "/metrics" {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]uint64{
            "accepted": g.accepted.Load(), "over_capacity": g.rejected.Load(),
            "downstream_failures": g.downstreamFailures.Load(),
            "circuit_open_rejections": g.circuitOpenRejections.Load(),
            "retry_attempts": g.retryAttempts.Load(),
            "retry_budget_denied": g.retryBudgetDenied.Load(),
            "stream_requests": g.streamsStarted.Load(),
            "stream_failures": g.streamFailures.Load(),
            "rate_limit_rejections": g.rateLimitRejected.Load(),
        })
        return
    }
    if r.URL.Path != "/infer" {
        http.NotFound(w, r)
        return
    }
    if r.Method != http.MethodPost {
        http.Error(w, "POST required", http.StatusMethodNotAllowed)
        return
    }
    if !g.allowRate(time.Now()) {
        g.rateLimitRejected.Add(1)
        w.Header().Set("Retry-After", "1")
        http.Error(w, "global gateway rate limit exceeded", http.StatusTooManyRequests)
        return
    }
    select {
    case g.permits <- struct{}{}:
        defer func(){ <-g.permits }()
    default:
        g.rejected.Add(1)
        http.Error(w, "gateway concurrency exhausted", http.StatusTooManyRequests)
        return
    }
    body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
    if err != nil || len(body) > maxBodyBytes {
        http.Error(w, "invalid or oversized request body", http.StatusRequestEntityTooLarge)
        return
    }
    g.accepted.Add(1)
    if strings.EqualFold(strings.TrimSpace(r.Header.Get("Accept")), "text/event-stream") {
        g.serveEventStream(w, r, body)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), g.timeout)
    defer cancel()
    backend := g.selectBackendExcept(nil)
    if backend == nil {
        g.circuitOpenRejections.Add(1)
        http.Error(w, "all backend circuits open", http.StatusServiceUnavailable)
        return
    }
    key := r.Header.Get("Idempotency-Key")
    canRetry := g.maxRetries > 0 && len(key) > 0 && len(key) <= 128 && len(g.backends) > 1
    result := g.attempt(ctx, r.Context(), backend, body, r.Header.Get("Content-Type"), key)
    for i := 0; canRetry && i < g.maxRetries && result.retryable && ctx.Err() == nil; i++ {
        next := g.selectBackendExcept(backend)
        if next == nil { break }
        if !g.acquireRetryToken() {
            // A half-open candidate was reserved; release it without crediting
            // a successful probe or sending an extra request.
            next.mu.Lock()
            next.probing = false
            next.mu.Unlock()
            break
        }
        backend = next
        result = g.attempt(ctx, r.Context(), backend, body, r.Header.Get("Content-Type"), key)
    }
    if result.contentType != "" { w.Header().Set("Content-Type", result.contentType) }
    w.WriteHeader(result.status)
    _, _ = w.Write(result.body)
}
