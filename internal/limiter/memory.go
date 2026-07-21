package limiter

import (
	"errors"
	"sync"
	"time"
)

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

func (f *FixedWindow) Allow(key string) (bool, error) {
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
		return false, errors.New("You have hit the limit, try again later")
	}
	state.counter++
	return true, nil

}
