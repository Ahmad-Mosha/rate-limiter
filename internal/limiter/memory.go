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
