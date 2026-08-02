package middleware

import (
	"net/http"

	"github.com/ahmad-mosha/go-rate-limiter/internal/limiter"
)

func RateLimiterMiddleware(limiter limiter.RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, err := limiter.Allow(r.RemoteAddr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r) g
	})
}
