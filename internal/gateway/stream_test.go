package gateway

import (
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestEventStreamForwardedWithoutRetries(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("Accept") != "text/event-stream" {
            t.Error("missing SSE accept header")
        }
        w.Header().Set("Content-Type", "text/event-stream")
        for i := 0; i < 3; i++ {
            _, _ = fmt.Fprintf(w, "data: %d\n\n", i)
            w.(http.Flusher).Flush()
        }
    }))
    defer server.Close()
    g, err := NewWithPolicies([]string{server.URL}, 2, time.Second, 3, time.Second, 2, 1)
    if err != nil { t.Fatal(err) }
    request := httptest.NewRequest("POST", "/infer", strings.NewReader("{}"))
    request.Header.Set("Accept", "text/event-stream")
    request.Header.Set("Idempotency-Key", "opted-in-but-stream-cannot-retry")
    recorder := httptest.NewRecorder()
    g.ServeHTTP(recorder, request)
    if recorder.Code != 200 { t.Fatalf("HTTP %d", recorder.Code) }
    if strings.Count(recorder.Body.String(), "data:") != 3 {
        t.Fatalf("missing SSE chunks: %q", recorder.Body.String())
    }
    if g.retryAttempts.Load() != 0 || g.streamsStarted.Load() != 1 {
        t.Fatal("streaming request was retried or not counted")
    }
}

func TestStreamingRejectsNonSSEResponse(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte("{}"))
    }))
    defer server.Close()
    g, _ := New([]string{server.URL}, 1, time.Second)
    r := httptest.NewRequest("POST", "/infer", nil)
    r.Header.Set("Accept", "text/event-stream")
    rec := httptest.NewRecorder()
    g.ServeHTTP(rec, r)
    if rec.Code != 502 || g.streamFailures.Load() != 1 {
        t.Fatalf("expected 502 for wrong content-type, got %d", rec.Code)
    }
}

func TestSSEStreamOutlivesUnaryDeadline(t *testing.T) {
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/event-stream")
        _, _ = w.Write([]byte("data: first\n\n"))
        w.(http.Flusher).Flush()
        time.Sleep(125*time.Millisecond)
        _, _ = w.Write([]byte("data: second\n\n"))
        w.(http.Flusher).Flush()
    }))
    defer backend.Close()
    g, err := New([]string{backend.URL}, 1, 40*time.Millisecond)
    if err != nil { t.Fatal(err) }
    if err := g.SetStreamTimeout(time.Second); err != nil { t.Fatal(err) }
    r := httptest.NewRequest("POST", "/infer", strings.NewReader("{}"))
    r.Header.Set("Accept", "text/event-stream")
    rec := httptest.NewRecorder()
    g.ServeHTTP(rec, r)
    if rec.Code != 200 { t.Fatalf("unexpected HTTP %d", rec.Code) }
    if strings.Count(rec.Body.String(), "data:") != 2 {
        t.Fatalf("SSE terminated at unary request deadline: %q", rec.Body.String())
    }
    if g.streamFailures.Load() != 0 { t.Fatal("healthy stream was counted as failure") }
}

func TestStreamTimeoutPolicyValidation(t *testing.T) {
    g, _ := New([]string{"http://127.0.0.1:8081"}, 1, time.Second)
    if err := g.SetStreamTimeout(0); err == nil { t.Fatal("zero timeout allowed") }
    if err := g.SetStreamTimeout(11*time.Minute); err == nil { t.Fatal("unbounded timeout allowed") }
}
