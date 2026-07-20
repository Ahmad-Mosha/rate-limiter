package limiter

import (
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
