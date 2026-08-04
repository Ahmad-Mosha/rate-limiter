package limiter

import (
	"context"
	"sync"
	"time"
)

var _ RateLimiter = (*FixedWindow)(nil)

type windowState struct {
	counter   int // counter of requests per window
	startTime time.Time
}

type FixedWindow struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string]*windowState
}

func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{
		limit:    limit,
		window:   window,
		requests: make(map[string]*windowState),
	}
}

func (f *FixedWindow) Allow(ctx context.Context, key string) (Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	state, exists := f.requests[key]
	if !exists {
		state = &windowState{counter: 0, startTime: time.Now()}
		f.requests[key] = state
	}
	elapsed := time.Since(state.startTime)
	if elapsed > f.window {
		state.counter = 0
		state.startTime = time.Now()
	}

	if state.counter >= f.limit {
		return Result{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: f.window - elapsed,
		}, nil
	}
	state.counter++
	return Result{
		Allowed:   true,
		Remaining: f.limit - state.counter,
	}, nil
}
