package limiter

import (
	"sync"
	"time"
)

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
