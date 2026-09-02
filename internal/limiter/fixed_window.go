package limiter

import (
	"context"
	"sync"
	"time"
)

var _ RateLimiter = (*FixedWindow)(nil)

type windowState struct {
	counter   int // requests seen in the current window
	startTime time.Time
}

type FixedWindow struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string]*windowState
	stop     chan struct{}
	stopOnce sync.Once
}

func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	f := &FixedWindow{
		limit:    limit,
		window:   window,
		requests: make(map[string]*windowState),
		stop:     make(chan struct{}),
	}
	go f.cleanup()
	return f
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

// cleanup periodically drops keys whose window has fully elapsed. Such an entry
// would be reset on its next request anyway, so removing it is safe and keeps
// the map from growing without bound when the key space is large (one entry per
// client IP, for example).
func (f *FixedWindow) cleanup() {
	ticker := time.NewTicker(f.window)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			f.mu.Lock()
			for key, state := range f.requests {
				if time.Since(state.startTime) > f.window {
					delete(f.requests, key)
				}
			}
			f.mu.Unlock()
		case <-f.stop:
			return
		}
	}
}

// Stop halts the background cleanup goroutine. Safe to call more than once.
func (f *FixedWindow) Stop() {
	f.stopOnce.Do(func() { close(f.stop) })
}
