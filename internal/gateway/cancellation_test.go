package gateway

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

func TestClientCancellationDoesNotPoisonHTTPBackend(t *testing.T) {
    entered := make(chan struct{})
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        close(entered)
        <-r.Context().Done()
    }))
    defer backend.Close()
    gate, err := NewWithCircuitPolicy([]string{backend.URL}, 1, 5*time.Second, 1, time.Minute)
    if err != nil { t.Fatal(err) }
    ctx, cancel := context.WithCancel(context.Background())
    request := httptest.NewRequest("POST", "/infer", nil).WithContext(ctx)
    done := make(chan struct{})
    go func() {
        gate.ServeHTTP(httptest.NewRecorder(), request)
        close(done)
    }()
    select {
    case <-entered:
    case <-time.After(3*time.Second):
        t.Fatal("backend never received test request")
    }
    cancel()
    select {
    case <-done:
    case <-time.After(3*time.Second):
        t.Fatal("cancelled request never returned")
    }
    if gate.downstreamFailures.Load() != 0 {
        t.Fatal("client disconnect incorrectly counted as upstream failure")
    }
    gate.backends[0].mu.Lock()
    opened := !gate.backends[0].openUntil.IsZero()
    gate.backends[0].mu.Unlock()
    if opened { t.Fatal("client disconnect opened backend circuit") }
}

func TestClientCancellationDoesNotPoisonStreamingBackend(t *testing.T) {
    entered := make(chan struct{})
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        w.WriteHeader(200)
        w.(http.Flusher).Flush()
        close(entered)
        <-r.Context().Done()
    }))
    defer backend.Close()
    gate, err := NewWithCircuitPolicy([]string{backend.URL}, 1, 5*time.Second, 1, time.Minute)
    if err != nil { t.Fatal(err) }
    ctx, cancel := context.WithCancel(context.Background())
    request := httptest.NewRequest("POST", "/infer", nil).WithContext(ctx)
    request.Header.Set("Accept", "text/event-stream")
    done := make(chan struct{})
    go func() {
        gate.ServeHTTP(httptest.NewRecorder(), request)
        close(done)
    }()
    select {
    case <-entered:
    case <-time.After(3*time.Second):
        t.Fatal("stream never started")
    }
    cancel()
    select {
    case <-done:
    case <-time.After(3*time.Second):
        t.Fatal("cancelled stream did not terminate")
    }
    if gate.downstreamFailures.Load() != 0 || gate.streamFailures.Load() != 0 {
        t.Fatal("client disconnect incorrectly counted as stream/upstream failure")
    }
    gate.backends[0].mu.Lock()
    opened := !gate.backends[0].openUntil.IsZero()
    gate.backends[0].mu.Unlock()
    if opened { t.Fatal("client disconnect opened streaming backend circuit") }
}
