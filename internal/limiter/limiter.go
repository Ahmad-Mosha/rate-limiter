package limiter

type RateLimiter interface {
	Allow(key string) (bool, error)
}
