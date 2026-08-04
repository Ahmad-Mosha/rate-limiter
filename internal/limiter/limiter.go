package limiter

import (
	"context"
	"time"
)

type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type RateLimiter interface {
	Allow(ctx context.Context, key string) (Result, error)
}
