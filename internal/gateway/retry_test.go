package gateway

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "sync/atomic"
    "testing"
    "time"
)

func TestOptInRetryBudgetAndDifferentBackend(t *testing.T) {
    var failed atomic.Bool
    var callsA, callsB atomic.Int64
    a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        callsA.Add(1)
        if failed.Load() { w.WriteHeader(503); return }
        w.WriteHeader(200)
    }))
    defer a.Close()
    b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        callsB.Add(1)
        w.WriteHeader(200)
    }))
    defer b.Close()
    g, err := NewWithPolicies([]string{a.URL, b.URL}, 8, 1*time.Second,
                               3, time.Second, 1, 2)
    if err != nil { t.Fatal(err) }
    request := func(key string) int {
        r := httptest.NewRequest("POST", "/infer", strings.NewReader("{}"))
        if key != "" { r.Header.Set("Idempotency-Key", key) }
        w := httptest.NewRecorder()
        g.ServeHTTP(w, r)
        return w.Code
    }
    if request("") != 200 || request("") != 200 { t.Fatal("warmup failed") }
    failed.Store(true)
    if request("unique-test") != 200 { t.Fatal("retry should choose healthy alternate backend") }
    if callsA.Load() != 2 || callsB.Load() != 2 { t.Fatal("unexpected backend amplification") }
    if g.retryAttempts.Load() != 1 { t.Fatal("expected one retry token") }
}

func TestPostWithoutIdempotencyKeyNeverRetries(t *testing.T) {
    var calls atomic.Int64
    a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls.Add(1)
        w.WriteHeader(503)
    }))
    defer a.Close()
    b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        calls.Add(1)
        w.WriteHeader(200)
    }))
    defer b.Close()
    g, _ := NewWithPolicies([]string{a.URL, b.URL}, 2, time.Second, 3, time.Second, 1, 1)
    w := httptest.NewRecorder()
    g.ServeHTTP(w, httptest.NewRequest("POST", "/infer", nil))
    if w.Code != 503 || calls.Load() != 1 || g.retryAttempts.Load() != 0 {
        t.Fatal("POST without idempotency opt-in was retried")
    }
}

func TestRetryBudgetLimitsAmplification(t *testing.T) {
    a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(503)
    }))
    defer a.Close()
    b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(503)
    }))
    defer b.Close()
    g, _ := NewWithPolicies([]string{a.URL, b.URL}, 2, time.Second, 100, time.Second, 1, 10)
    for i := 0; i < 30; i++ {
        req := httptest.NewRequest("POST", "/infer", nil)
        req.Header.Set("Idempotency-Key", "distinct")
        g.ServeHTTP(httptest.NewRecorder(), req)
    }
    if g.retryAttempts.Load() > g.accepted.Load()/10 {
        t.Fatalf("budget violated: retries=%d accepted=%d",
            g.retryAttempts.Load(), g.accepted.Load())
    }
}

func TestBodyBound(t *testing.T) {
    b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Fatal("oversized body should never reach backend")
    }))
    defer b.Close()
    g, _ := New([]string{b.URL}, 1, time.Second)
    r := httptest.NewRequest("POST", "/infer", strings.NewReader(strings.Repeat("x", maxBodyBytes+1)))
    w := httptest.NewRecorder()
    g.ServeHTTP(w, r)
    if w.Code != 413 { t.Fatalf("expected 413, got %d", w.Code) }
}

func TestInvalidRetryPolicy(t *testing.T) {
    if _, err := NewWithPolicies([]string{"http://localhost"}, 1, time.Second, 3,
                                 time.Second, 4, 5); err == nil { t.Fatal("expected rejection") }
}
