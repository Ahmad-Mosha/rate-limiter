package middleware

import (
	"net"
	"net/http"

	"github.com/ahmad-mosha/go-rate-limiter/internal/limiter"
)

func RateLimiterMiddleware(limiter limiter.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		allowed, err := limiter.Allow(host)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
