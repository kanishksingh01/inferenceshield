package gateway

import (
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func TestPrometheusMetricsAreValidTextExposition(t *testing.T) {
    g, err := New([]string{"http://127.0.0.1:65533"}, 1, time.Second)
    if err != nil { t.Fatal(err) }
    g.accepted.Store(3)
    g.retryAttempts.Store(1)
    w := httptest.NewRecorder()
    g.ServeHTTP(w, httptest.NewRequest("GET", "/prometheus", nil))
    if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "text/plain") {
        t.Fatal("invalid Prometheus response")
    }
    content := w.Body.String()
    for _, item := range []string{
        "# TYPE inferenceshield_requests_accepted_total counter\n",
        "inferenceshield_requests_accepted_total 3\n",
        "inferenceshield_retry_attempts_total 1\n",
    } {
        if !strings.Contains(content, item) { t.Errorf("missing metric line %q", item) }
    }
}
