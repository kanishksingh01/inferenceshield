package gateway

import (
    "encoding/json"
    "io"
    "net/http"
    "net/url"
    "strings"
    "sync/atomic"
    "time"
)

type Gateway struct {
    backends []*url.URL
    client *http.Client
    permits chan struct{}
    next atomic.Uint64
    accepted atomic.Uint64
    rejected atomic.Uint64
    downstreamFailures atomic.Uint64
}

func New(targets []string, maxInFlight int, timeout time.Duration) (*Gateway, error) {
    if maxInFlight < 1 || timeout <= 0 || len(targets) == 0 {
        return nil, &ConfigError{"at least one backend, positive capacity and positive timeout required"}
    }
    gateway := &Gateway{permits: make(chan struct{}, maxInFlight),
        client: &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
            return http.ErrUseLastResponse
        }}}
    for _, raw := range targets {
        u, err := url.Parse(strings.TrimSpace(raw))
        if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
            return nil, &ConfigError{"invalid backend URL"}
        }
        gateway.backends = append(gateway.backends, u)
    }
    return gateway, nil
}

type ConfigError struct { Message string }
func (e *ConfigError) Error() string { return e.Message }

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
    g.accepted.Add(1)
    index := (g.next.Add(1) - 1) % uint64(len(g.backends))
    target := *g.backends[index]
    target.Path = "/infer"
    target.RawQuery = ""
    request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target.String(), r.Body)
    if err != nil {
        g.downstreamFailures.Add(1)
        http.Error(w, "upstream request unavailable", http.StatusBadGateway)
        return
    }
    request.Header.Set("Content-Type", r.Header.Get("Content-Type"))
    response, err := g.client.Do(request)
    if err != nil {
        g.downstreamFailures.Add(1)
        http.Error(w, "upstream request failed", http.StatusBadGateway)
        return
    }
    defer response.Body.Close()
    if response.StatusCode >= 500 { g.downstreamFailures.Add(1) }
    if contentType := response.Header.Get("Content-Type"); contentType != "" {
        w.Header().Set("Content-Type", contentType)
    }
    w.WriteHeader(response.StatusCode)
    _, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}
