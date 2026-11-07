package gateway

import (
    "net/http"
    "net/http/httptest"
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
