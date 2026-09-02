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
	refillRate float64 // tokens added per second
	buckets    map[string]*bucketState
	stop       chan struct{}
	stopOnce   sync.Once
}

func NewBucket(capacity float64, refillRate float64) *Bucket {
	b := &Bucket{
		capacity:   capacity,
		refillRate: refillRate,
		buckets:    make(map[string]*bucketState),
		stop:       make(chan struct{}),
	}
	// With no refill a bucket never returns to a "fresh" state, so there is
	// nothing safe to evict on a timer. That configuration is only really
	// useful in tests.
	if refillRate > 0 {
		go b.cleanup()
	}
	return b
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

// cleanup periodically drops buckets that have sat idle long enough to refill to
// full capacity. A full bucket is identical to a freshly created one, so this is
// safe and keeps the map from growing without bound.
func (b *Bucket) cleanup() {
	timeToFull := time.Duration(b.capacity / b.refillRate * float64(time.Second))
	if timeToFull < time.Second {
		timeToFull = time.Second
	}
	ticker := time.NewTicker(timeToFull)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.mu.Lock()
			for key, state := range b.buckets {
				if time.Since(state.lastRefillTime) >= timeToFull {
					delete(b.buckets, key)
				}
			}
			b.mu.Unlock()
		case <-b.stop:
			return
		}
	}
}

// Stop halts the background cleanup goroutine. Safe to call more than once.
func (b *Bucket) Stop() {
	b.stopOnce.Do(func() { close(b.stop) })
}
