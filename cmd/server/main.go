package main

import (
	"net/http"
	"time"

	"github.com/ahmad-mosha/go-rate-limiter/internal/limiter"
	"github.com/ahmad-mosha/go-rate-limiter/internal/middleware"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello, world!"))

}

func main() {
	fixedWindow := limiter.NewFixedWindow(10, time.Minute)
	wrappedHandler := middleware.RateLimiterMiddleware(fixedWindow, http.HandlerFunc(helloHandler))
	http.Handle("/", wrappedHandler)
	http.ListenAndServe(":8080", nil)

}
