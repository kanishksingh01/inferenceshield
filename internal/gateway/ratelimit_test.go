package gateway

import (
    "testing"
    "time"
)

func TestTokenBucketDeterministicRefill(t *testing.T) {
    now := time.Unix(0, 0)
    b := &tokenBucket{rate: 2, capacity: 2, tokens: 2, last: now}
    if !b.take(now) || !b.take(now) { t.Fatal("initial burst not allowed") }
    if b.take(now) { t.Fatal("burst limit was exceeded") }
    if b.take(now.Add(200*time.Millisecond)) { t.Fatal("refilled too early") }
    if !b.take(now.Add(600*time.Millisecond)) { t.Fatal("expected one refilled token") }
    if b.take(now.Add(600*time.Millisecond)) { t.Fatal("double spending of token") }
}
