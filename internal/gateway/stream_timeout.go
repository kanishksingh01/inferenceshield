package gateway

import (
    "errors"
    "time"
)

// SetStreamTimeout configures the maximum total SSE request lifetime.
// This is separate from the unary HTTP client timeout; set at startup.
func (g *Gateway) SetStreamTimeout(duration time.Duration) error {
    if duration < time.Second || duration > 10*time.Minute {
        return errors.New("stream timeout must be between 1s and 10m")
    }
    g.streamTimeout = duration
    return nil
}
