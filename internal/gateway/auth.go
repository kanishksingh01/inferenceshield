package gateway

import (
    "crypto/subtle"
    "net/http"
)

// RequireBearerToken wraps a handler without mutating its state.
// Health remains publicly readable for platform readiness. When token is
// empty authentication is disabled ONLY for the isolated synthetic lab.
func RequireBearerToken(token string, next http.Handler) http.Handler {
    if token == "" { return next }
    expected := []byte("Bearer " + token)
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/health" {
            next.ServeHTTP(w, r)
            return
        }
        authorization := []byte(r.Header.Get("Authorization"))
        if subtle.ConstantTimeCompare(authorization, expected) != 1 {
            w.Header().Set("WWW-Authenticate", "Bearer")
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
