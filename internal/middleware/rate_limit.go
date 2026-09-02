package middleware

import (
	"math"
	"net"
	"net/http"
	"strconv"

	"github.com/Ahmad-Mosha/rate-limiter/internal/limiter"
)

func RateLimiterMiddleware(limiter limiter.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		result, err := limiter.Allow(r.Context(), host)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		if !result.Allowed {
			retrySeconds := int(math.Ceil(result.RetryAfter.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
