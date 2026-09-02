package server

import (
	"net/http"

	"github.com/Ahmad-Mosha/rate-limiter/limiter"
	"github.com/Ahmad-Mosha/rate-limiter/middleware"
)

// Server holds the HTTP mux and its dependencies.
type Server struct {
	mux *http.ServeMux
}

// New creates a Server with all routes and middleware wired up.
func New(rateLimiter limiter.RateLimiter) *Server {
	mux := http.NewServeMux()

	mux.Handle("/", middleware.RateLimiterMiddleware(rateLimiter, http.HandlerFunc(helloHandler)))
	mux.HandleFunc("/health", healthHandler)

	return &Server{mux: mux}
}

// Start begins listening on the given address.
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
