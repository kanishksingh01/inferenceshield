package gateway

import (
    "bytes"
    "context"
    "io"
    "net/http"
    "strings"
)

// serveEventStream passes SSE chunks through as they arrive, without buffering
// the entire stream. Streaming requests are NEVER retried after a response starts.
func (g *Gateway) serveEventStream(w http.ResponseWriter, r *http.Request, body []byte) {
    g.streamsStarted.Add(1)
    flusher, ok := w.(http.Flusher)
    if !ok {
        g.streamFailures.Add(1)
        http.Error(w, "streaming unsupported by response writer", http.StatusInternalServerError)
        return
    }
    backend := g.selectBackendExcept(nil)
    if backend == nil {
        g.circuitOpenRejections.Add(1)
        g.streamFailures.Add(1)
        http.Error(w, "all backend circuits open", http.StatusServiceUnavailable)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), g.streamTimeout)
    defer cancel()
    target := *backend.url
    target.Path = "/infer"
    target.RawQuery = ""
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(),
                                           bytes.NewReader(body))
    if err != nil {
        g.downstreamFailures.Add(1)
        g.streamFailures.Add(1)
        backend.finish(false, g.threshold, g.cooldown)
        http.Error(w, "upstream stream unavailable", http.StatusBadGateway)
        return
    }
    req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
    req.Header.Set("Accept", "text/event-stream")
    resp, err := g.streamClient.Do(req)
    if err != nil {
        if r.Context().Err() != nil {
            backend.releaseProbe()
            return // client disconnect is not an upstream failure
        }
        g.downstreamFailures.Add(1)
        g.streamFailures.Add(1)
        backend.finish(false, g.threshold, g.cooldown)
        http.Error(w, "upstream stream failed", http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        unhealthy := resp.StatusCode == 429 || resp.StatusCode >= 500
        if unhealthy { g.downstreamFailures.Add(1); g.streamFailures.Add(1) }
        backend.finish(!unhealthy, g.threshold, g.cooldown)
        w.WriteHeader(resp.StatusCode)
        _, _ = io.Copy(w, io.LimitReader(resp.Body, maxBodyBytes))
        return
    }
    if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
        g.downstreamFailures.Add(1)
        g.streamFailures.Add(1)
        backend.finish(false, g.threshold, g.cooldown)
        http.Error(w, "upstream is not an event stream", http.StatusBadGateway)
        return
    }
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.WriteHeader(http.StatusOK)
    buf := make([]byte, 4096)
    remaining := int64(maxBodyBytes)
    for {
        if remaining <= 0 {
            g.streamFailures.Add(1)
            backend.finish(false, g.threshold, g.cooldown)
            return // Hard cap; headers already sent, cannot change HTTP status.
        }
        n, readErr := resp.Body.Read(buf[:min(len(buf), int(remaining))])
        if n > 0 {
            wrote, writeErr := w.Write(buf[:n])
            remaining -= int64(n)
            flusher.Flush()
            if writeErr != nil || wrote != n {
                // A failed downstream write is not proof the upstream is unhealthy.
                backend.releaseProbe()
                return
            }
        }
        if readErr == io.EOF {
            backend.finish(true, g.threshold, g.cooldown)
            return
        }
        if readErr != nil {
            if r.Context().Err() != nil {
                backend.releaseProbe()
                return
            }
            g.streamFailures.Add(1)
            g.downstreamFailures.Add(1)
            backend.finish(false, g.threshold, g.cooldown)
            return
        }
    }
}
