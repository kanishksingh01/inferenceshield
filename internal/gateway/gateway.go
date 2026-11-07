package gateway

import (
    "encoding/json"
    "io"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

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

type Gateway struct {
    backends []*backendState
    client *http.Client
    permits chan struct{}
    threshold int
    cooldown time.Duration
    next atomic.Uint64
    accepted atomic.Uint64
    rejected atomic.Uint64
    downstreamFailures atomic.Uint64
    circuitOpenRejections atomic.Uint64
}

func New(targets []string, maxInFlight int, timeout time.Duration) (*Gateway, error) {
    return NewWithCircuitPolicy(targets, maxInFlight, timeout, 3, 5*time.Second)
}

func NewWithCircuitPolicy(targets []string, maxInFlight int, timeout time.Duration,
                          failureThreshold int, cooldown time.Duration) (*Gateway, error) {
    if maxInFlight < 1 || timeout <= 0 || len(targets) == 0 ||
        failureThreshold < 1 || cooldown <= 0 {
        return nil, &ConfigError{"invalid backends, capacity, timeout or circuit policy"}
    }
    gateway := &Gateway{permits: make(chan struct{}, maxInFlight),
        threshold: failureThreshold, cooldown: cooldown,
        client: &http.Client{Timeout: timeout,
            CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
                return http.ErrUseLastResponse
            }}}
    for _, raw := range targets {
        u, err := url.Parse(strings.TrimSpace(raw))
        if err != nil || u == nil || u.Host == "" ||
            (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
            return nil, &ConfigError{"invalid backend URL"}
        }
        gateway.backends = append(gateway.backends, &backendState{url: u})
    }
    return gateway, nil
}

type ConfigError struct { Message string }
func (e *ConfigError) Error() string { return e.Message }

func (g *Gateway) selectBackend() *backendState {
    start := g.next.Add(1)-1
    for offset := 0; offset < len(g.backends); offset++ {
        index := (start + uint64(offset)) % uint64(len(g.backends))
        candidate := g.backends[index]
        if candidate.eligible(time.Now()) {
            return candidate
        }
    }
    return nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path == "/health" {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
        return
    }
    if r.URL.Path == "/metrics" {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]uint64{
            "accepted": g.accepted.Load(),
            "over_capacity": g.rejected.Load(),
            "downstream_failures": g.downstreamFailures.Load(),
            "circuit_open_rejections": g.circuitOpenRejections.Load(),
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
    select {
    case g.permits <- struct{}{}:
        defer func() { <-g.permits }()
    default:
        g.rejected.Add(1)
        http.Error(w, "gateway concurrency exhausted", http.StatusTooManyRequests)
        return
    }
    backend := g.selectBackend()
    if backend == nil {
        g.circuitOpenRejections.Add(1)
        http.Error(w, "all backend circuits open", http.StatusServiceUnavailable)
        return
    }
    g.accepted.Add(1)
    target := *backend.url
    target.Path = "/infer"
    target.RawQuery = ""
    req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target.String(), r.Body)
    if err != nil {
        g.downstreamFailures.Add(1)
        backend.finish(false, g.threshold, g.cooldown)
        http.Error(w, "upstream request unavailable", http.StatusBadGateway)
        return
    }
    req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
    response, err := g.client.Do(req)
    if err != nil {
        g.downstreamFailures.Add(1)
        backend.finish(false, g.threshold, g.cooldown)
        http.Error(w, "upstream request failed", http.StatusBadGateway)
        return
    }
    defer response.Body.Close()
    unhealthy := response.StatusCode == 429 || response.StatusCode >= 500
    if unhealthy { g.downstreamFailures.Add(1) }
    backend.finish(!unhealthy, g.threshold, g.cooldown)
    if ct := response.Header.Get("Content-Type"); ct != "" {
        w.Header().Set("Content-Type", ct)
    }
    w.WriteHeader(response.StatusCode)
    _, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}
