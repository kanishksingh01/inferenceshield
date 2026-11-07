package gateway

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestAuthenticationProtectsInferenceAndMetrics(t *testing.T) {
    underlying := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusNoContent)
    })
    h := RequireBearerToken("synthetic-lab-token", underlying)
    for _, path := range []string{"/infer", "/metrics", "/prometheus"} {
        r := httptest.NewRequest("GET", path, nil)
        recorder := httptest.NewRecorder()
        h.ServeHTTP(recorder, r)
        if recorder.Code != 401 { t.Fatalf("%s should be denied", path) }
        r.Header.Set("Authorization", "Bearer synthetic-lab-token")
        recorder = httptest.NewRecorder()
        h.ServeHTTP(recorder, r)
        if recorder.Code != 204 { t.Fatalf("%s should permit valid token", path) }
    }
    health := httptest.NewRecorder()
    h.ServeHTTP(health, httptest.NewRequest("GET", "/health", nil))
    if health.Code != 204 { t.Fatal("health must remain available") }
}

func TestDisabledAuthenticationIsExplicitLabOnly(t *testing.T) {
    h := RequireBearerToken("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    }))
    w := httptest.NewRecorder()
    h.ServeHTTP(w, httptest.NewRequest("GET", "/infer", nil))
    if w.Code != 200 { t.Fatal("expected local lab to bypass auth") }
}
