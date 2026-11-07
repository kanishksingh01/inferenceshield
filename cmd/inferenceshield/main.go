package main

import (
    "log"
    "net/http"
    "os"
    "strings"
    "time"
    "github.com/kanishksingh01/inferenceshield/internal/gateway"
)

func main() {
    raw := os.Getenv("UPSTREAM_URLS")
    if raw == "" { log.Fatal("UPSTREAM_URLS must specify one or more test backends") }
    handler, err := gateway.New(strings.Split(raw, ","), 16, 1500*time.Millisecond)
    if err != nil { log.Fatal(err) }
    server := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5*time.Second}
    log.Println("InferenceShield P0 gateway listening on :8080")
    log.Fatal(server.ListenAndServe())
}
