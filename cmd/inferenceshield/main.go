package main

import (
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"
    "github.com/kanishksingh01/inferenceshield/internal/gateway"
)

func main() {
    raw := os.Getenv("UPSTREAM_URLS")
    if raw == "" { log.Fatal("UPSTREAM_URLS must specify one or more test backends") }
    handler, err := gateway.New(strings.Split(raw, ","), 16, 1500*time.Millisecond)
    if err != nil { log.Fatal(err) }
    if streamRaw := os.Getenv("STREAM_TIMEOUT_SECONDS"); streamRaw != "" {
        seconds, err := strconv.Atoi(streamRaw)
        if err != nil || seconds < 1 || seconds > 600 {
            log.Fatal("STREAM_TIMEOUT_SECONDS must be 1..600")
        }
        if err := handler.SetStreamTimeout(time.Duration(seconds)*time.Second); err != nil {
            log.Fatal(err)
        }
    }
    rateString, burstString := os.Getenv("RATE_LIMIT_RPS"), os.Getenv("RATE_LIMIT_BURST")
    if rateString != "" || burstString != "" {
        rps, e1 := strconv.ParseFloat(rateString, 64)
        burst, e2 := strconv.Atoi(burstString)
        if e1 != nil || e2 != nil { log.Fatal("invalid RATE_LIMIT_RPS/RATE_LIMIT_BURST") }
        if err := handler.SetGlobalRateLimit(rps, burst); err != nil { log.Fatal(err) }
    }
    token := os.Getenv("AUTH_TOKEN")
    if token == "" {
        log.Println("warning: no AUTH_TOKEN configured; local lab mode only")
    }
    wrapped := gateway.RequireBearerToken(token, handler)
    server := &http.Server{Addr: ":8080", Handler: wrapped, ReadHeaderTimeout: 5*time.Second}
    log.Println("InferenceShield P0 gateway listening on :8080")
    log.Fatal(server.ListenAndServe())
}
