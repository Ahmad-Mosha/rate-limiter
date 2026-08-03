package main

import (
	"log"
	"time"

	"github.com/ahmad-mosha/go-rate-limiter/internal/limiter"
	"github.com/ahmad-mosha/go-rate-limiter/internal/server"
)

func main() {
	rateLimiter := limiter.NewFixedWindow(10, time.Minute)
	srv := server.New(rateLimiter)
	log.Fatal(srv.Start(":8080"))
}
