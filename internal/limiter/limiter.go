package limiter

type RateLimiter interface {
	Allow(ip string) (bool, error)
}
