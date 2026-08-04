package limiter

import (
	"context"
	"sync"
	"time"
)

var _ RateLimiter = (*Bucket)(nil)

type bucketState struct {
	tokens         float64
	lastRefillTime time.Time
}

type Bucket struct {
	mu         sync.Mutex
	capacity   float64
	refillRate float64
	buckets    map[string]*bucketState
}

func NewBucket(capacity float64, refillRate float64) *Bucket {
	return &Bucket{
		capacity:   capacity,
		refillRate: refillRate,
		buckets:    make(map[string]*bucketState),
	}
}

func (b *Bucket) Allow(ctx context.Context, key string) (Result, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state, exists := b.buckets[key]
	if !exists {
		state = &bucketState{tokens: b.capacity, lastRefillTime: time.Now()}
		b.buckets[key] = state
	}
	elapsed := time.Since(state.lastRefillTime).Seconds()
	tokensToAdd := elapsed * b.refillRate
	state.tokens = min(state.tokens+tokensToAdd, b.capacity)
	state.lastRefillTime = time.Now()
	if state.tokens >= 1 {
		state.tokens -= 1
		return Result{
			Allowed:   true,
			Remaining: int(state.tokens),
		}, nil
	}
	secondsToWait := (1 - state.tokens) / b.refillRate
	return Result{
		Allowed:    false,
		Remaining:  0,
		RetryAfter: time.Duration(secondsToWait * float64(time.Second)),
	}, nil

}
