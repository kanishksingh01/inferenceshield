package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"
    "time"
)

func main() {
    mode := os.Getenv("MOCK_MODE")
    if mode == "" { mode = "healthy" }
    http.HandleFunc("/infer", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "POST required", 405)
            return
        }
        switch mode {
        case "error":
            http.Error(w, "synthetic failure", http.StatusServiceUnavailable)
        case "slow":
            time.Sleep(2 * time.Second)
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "mode": mode})
        default:
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "mode": mode})
        }
    })
    server := &http.Server{Addr: ":8081", ReadHeaderTimeout: 5*time.Second}
    log.Fatal(server.ListenAndServe())
}
