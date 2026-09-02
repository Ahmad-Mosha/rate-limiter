package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var _ RateLimiter = (*RedisFixedWindow)(nil)

// fixedWindowScript does the whole check in one atomic step so that requests
// hitting different app instances can't race between the INCR and the expiry.
// It increments the counter, sets the window TTL on the first request, and
// returns the current count plus the remaining TTL in milliseconds.
var fixedWindowScript = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return {count, redis.call("PTTL", KEYS[1])}
`)

// RedisFixedWindow is a fixed-window limiter backed by Redis. Use it when more
// than one process needs to share the same limit.
type RedisFixedWindow struct {
	client redis.Scripter
	limit  int
	window time.Duration
	prefix string
}

func NewRedisFixedWindow(client redis.Scripter, limit int, window time.Duration) *RedisFixedWindow {
	return &RedisFixedWindow{
		client: client,
		limit:  limit,
		window: window,
		prefix: "ratelimit:",
	}
}

func (r *RedisFixedWindow) Allow(ctx context.Context, key string) (Result, error) {
	res, err := fixedWindowScript.Run(ctx, r.client, []string{r.prefix + key}, r.window.Milliseconds()).Int64Slice()
	if err != nil {
		return Result{}, err
	}
	count, ttlMS := res[0], res[1]
	if ttlMS < 0 {
		// Key exists without a TTL (shouldn't happen via this limiter). Treat
		// the whole window as remaining rather than divide by zero downstream.
		ttlMS = r.window.Milliseconds()
	}

	if count > int64(r.limit) {
		return Result{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: time.Duration(ttlMS) * time.Millisecond,
		}, nil
	}
	return Result{
		Allowed:   true,
		Remaining: r.limit - int(count),
	}, nil
}
