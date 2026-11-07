package gateway

import (
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "strings"
    "testing"
    "time"
)

func TestHealthyBackend(t *testing.T) {
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/infer" { t.Errorf("wrong path: %s", r.URL.Path) }
        w.WriteHeader(200)
        _, _ = w.Write([]byte("ok"))
    }))
    defer backend.Close()
    gate, err := New([]string{backend.URL}, 2, time.Second)
    if err != nil { t.Fatal(err) }
    rec := httptest.NewRecorder()
    gate.ServeHTTP(rec, httptest.NewRequest("POST", "/infer", strings.NewReader("{}")))
    if rec.Code != 200 { t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String()) }
}

func TestDeadBackendFailsWithinTimeout(t *testing.T) {
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
    url := backend.URL
    backend.Close()
    gate, err := New([]string{url}, 1, 100*time.Millisecond)
    if err != nil { t.Fatal(err) }
    rec := httptest.NewRecorder()
    gate.ServeHTTP(rec, httptest.NewRequest("POST", "/infer", nil))
    if rec.Code != 502 { t.Fatalf("expected 502, got %d", rec.Code) }
}

func TestConcurrencyBound(t *testing.T) {
    entered := make(chan struct{})
    release := make(chan struct{})
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        close(entered)
        <-release
        w.WriteHeader(200)
    }))
    defer backend.Close()
    gate, err := New([]string{backend.URL}, 1, time.Second*3)
    if err != nil { t.Fatal(err) }
    done := make(chan struct{})
    go func() {
        gate.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/infer", nil))
        close(done)
    }()
    <-entered
    rec := httptest.NewRecorder()
    gate.ServeHTTP(rec, httptest.NewRequest("POST", "/infer", nil))
    if rec.Code != 429 { t.Fatalf("expected bounded 429; got %d", rec.Code) }
    close(release)
    <-done
}

func TestRejectBadConfig(t *testing.T) {
    for _, target := range []string{"file:///tmp/data", "http://user:pass@host", ""} {
        if _, err := New([]string{target}, 1, time.Second); err == nil {
            t.Errorf("expected invalid target: %s", target)
        }
    }
}

func TestCircuitBreakerOpensAndRejectsWithoutRetryStorm(t *testing.T) {
    var calls atomic.Int64
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls.Add(1)
        w.WriteHeader(503)
    }))
    defer backend.Close()
    gate, err := NewWithCircuitPolicy([]string{backend.URL}, 3, time.Second, 2, time.Second)
    if err != nil { t.Fatal(err) }
    for i := 0; i < 3; i++ {
        rec := httptest.NewRecorder()
        gate.ServeHTTP(rec, httptest.NewRequest("POST", "/infer", nil))
        if rec.Code != 503 { t.Fatalf("unexpected HTTP %d", rec.Code) }
    }
    if calls.Load() != 2 { t.Fatalf("expected 2 backend calls before open, got %d", calls.Load()) }
    if gate.circuitOpenRejections.Load() != 1 { t.Fatal("circuit did not reject open request") }
}

func TestHalfOpenRecoveryAfterCooldown(t *testing.T) {
    var healthy atomic.Bool
    var calls atomic.Int64
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls.Add(1)
        if !healthy.Load() { w.WriteHeader(503); return }
        w.WriteHeader(200)
    }))
    defer backend.Close()
    gate, err := NewWithCircuitPolicy([]string{backend.URL}, 2, time.Second, 1, 30*time.Millisecond)
    if err != nil { t.Fatal(err) }
    one := func() int {
        rec := httptest.NewRecorder()
        gate.ServeHTTP(rec, httptest.NewRequest("POST", "/infer", nil))
        return rec.Code
    }
    if one() != 503 { t.Fatal("first failure not propagated") }
    if one() != 503 { t.Fatal("open circuit must return 503") }
    if calls.Load() != 1 { t.Fatal("open circuit reached backend") }
    healthy.Store(true)
    time.Sleep(45*time.Millisecond)
    if one() != 200 { t.Fatal("half-open healthy probe did not close circuit") }
    if one() != 200 { t.Fatal("closed circuit did not serve request") }
}
