package gateway

import (
    "errors"
    "math"
    "sync"
    "time"
)

// tokenBucket is process-local. It is not a tenant quota or a distributed
// provider-level rate limiter, and cannot ensure multi-replica fairness.
type tokenBucket struct {
    mu sync.Mutex
    rate float64
    capacity float64
    tokens float64
    last time.Time
}

func (b *tokenBucket) take(now time.Time) bool {
    b.mu.Lock()
    defer b.mu.Unlock()
    elapsed := now.Sub(b.last).Seconds()
    if elapsed > 0 {
        b.tokens = math.Min(b.capacity, b.tokens+elapsed*b.rate)
        b.last = now
    }
    if b.tokens < 1 { return false }
    b.tokens--
    return true
}

// SetGlobalRateLimit configures a bounded global limiter; call at startup.
// Off by default to preserve the isolated synthetic lab's existing behavior.
func (g *Gateway) SetGlobalRateLimit(rps float64, burst int) error {
    if !(rps > 0) || math.IsNaN(rps) || math.IsInf(rps, 0) ||
        burst < 1 || burst > 100000 {
        return errors.New("rate must be finite and positive; burst must be 1..100000")
    }
    now := time.Now()
    b := &tokenBucket{
        rate: rps, capacity: float64(burst), tokens: float64(burst), last: now,
    }
    g.rateLimiterMu.Lock()
    g.rateLimiter = b
    g.rateLimiterMu.Unlock()
    return nil
}

func (g *Gateway) allowRate(now time.Time) bool {
    g.rateLimiterMu.RLock()
    limiter := g.rateLimiter
    g.rateLimiterMu.RUnlock()
    return limiter == nil || limiter.take(now)
}
