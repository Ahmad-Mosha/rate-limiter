package main

import (
	"flag"
	"log"
	"time"

	"github.com/ahmad-mosha/go-rate-limiter/internal/limiter"
	"github.com/ahmad-mosha/go-rate-limiter/internal/server"
	"github.com/redis/go-redis/v9"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	limit := flag.Int("limit", 10, "requests allowed per window")
	window := flag.Duration("window", time.Minute, "length of the rate-limit window")
	algo := flag.String("algo", "fixed-window", "algorithm: fixed-window, token-bucket, redis")
	redisAddr := flag.String("redis-addr", "localhost:6379", "redis address (only used with -algo=redis)")
	flag.Parse()

	rl := buildLimiter(*algo, *limit, *window, *redisAddr)

	log.Printf("listening on %s (algo=%s, limit=%d per %s)", *addr, *algo, *limit, *window)
	log.Fatal(server.New(rl).Start(*addr))
}

func buildLimiter(algo string, limit int, window time.Duration, redisAddr string) limiter.RateLimiter {
	switch algo {
	case "fixed-window":
		return limiter.NewFixedWindow(limit, window)
	case "token-bucket":
		// spread the same allowance evenly: refill one full window's worth of
		// tokens per window
		return limiter.NewBucket(float64(limit), float64(limit)/window.Seconds())
	case "redis":
		return limiter.NewRedisFixedWindow(redis.NewClient(&redis.Options{Addr: redisAddr}), limit, window)
	default:
		log.Fatalf("unknown -algo %q (want fixed-window, token-bucket, or redis)", algo)
		return nil
	}
}
