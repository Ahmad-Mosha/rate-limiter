# go-rate-limiter

[![CI](https://github.com/Ahmad-Mosha/rate-limiter/actions/workflows/ci.yml/badge.svg)](https://github.com/Ahmad-Mosha/rate-limiter/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.25-00ADD8)

A small rate limiter for Go HTTP services. Three interchangeable backends behind one
interface: in-memory **fixed window**, in-memory **token bucket**, and a **Redis fixed
window** for limits that need to hold across multiple instances.

I built this as a portfolio project to show how I approach a focused piece of
infrastructure — the interface, the concurrency, and the trade-offs behind each choice.
It is intentionally small and not a drop-in replacement for a hardened library like
`golang.org/x/time/rate` or `go-redis/redis_rate`.

## Contents

- [Features](#features)
- [Quick start](#quick-start)
- [Use it as a library](#use-it-as-a-library)
- [Algorithms](#algorithms)
- [Design decisions & trade-offs](#design-decisions--trade-offs)
- [Project layout](#project-layout)
- [Testing](#testing)
- [Limitations & what I'd do next](#limitations--what-id-do-next)

## Features

- One `RateLimiter` interface, three backends you can swap without touching call sites
- `net/http` middleware that sets `X-RateLimit-Remaining` and `Retry-After`
- Per-key limiting — client IP out of the box, any string key (user ID, API key) as a library
- Redis backend runs its check in a single atomic Lua script, so separate app instances
  can't race between the counter and its expiry
- Background eviction keeps the in-memory maps from growing without bound
- Race-tested; the Redis tests run against an in-process fake, so `go test` needs no server

## Quick start

Run the demo server:

```bash
go run ./cmd/server -limit 5 -window 10s
```

```bash
curl -i localhost:8080/
```

The first 5 requests in a 10-second window return `200`; the 6th returns `429` with a
`Retry-After` header.

Flags:

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `:8080` | listen address |
| `-limit` | `10` | requests allowed per window |
| `-window` | `1m` | window length |
| `-algo` | `fixed-window` | `fixed-window`, `token-bucket`, or `redis` |
| `-redis-addr` | `localhost:6379` | Redis address (only with `-algo=redis`) |

### Docker

```bash
docker build -t rate-limiter .
docker run -p 8080:8080 rate-limiter -limit 100 -window 1m
```

### With Redis

```bash
docker run -d -p 6379:6379 redis
go run ./cmd/server -algo redis -limit 100 -window 1m
```

## Use it as a library

```bash
go get github.com/Ahmad-Mosha/rate-limiter
```

As middleware:

```go
import (
    "github.com/Ahmad-Mosha/rate-limiter/limiter"
    "github.com/Ahmad-Mosha/rate-limiter/middleware"
)

rl := limiter.NewFixedWindow(100, time.Minute)
defer rl.Stop()

mux := http.NewServeMux()
mux.HandleFunc("/", handler)

http.ListenAndServe(":8080", middleware.RateLimiterMiddleware(rl, mux))
```

Directly, with your own key:

```go
rl := limiter.NewBucket(10, 1) // burst of 10, refills 1 token/sec
defer rl.Stop()

res, err := rl.Allow(ctx, "user:42")
if err != nil {
    // backend failure (Redis only) — decide whether to allow or block
}
if !res.Allowed {
    // tell the caller to retry after res.RetryAfter
}
```

Redis-backed:

```go
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
rl := limiter.NewRedisFixedWindow(client, 100, time.Minute)
```

The interface is the whole contract:

```go
type Result struct {
    Allowed    bool
    Remaining  int
    RetryAfter time.Duration
}

type RateLimiter interface {
    Allow(ctx context.Context, key string) (Result, error)
}
```

## Algorithms

| Backend | Shared across instances | Burst behaviour | Per-key state | Reach for it when |
|---|---|---|---|---|
| `FixedWindow` | no | up to 2× at a window boundary | one counter + timestamp | single instance, simple quotas |
| `Bucket` (token bucket) | no | up to `capacity`, then steady rate | one float + timestamp | you want to absorb spikes smoothly |
| `RedisFixedWindow` | yes | up to 2× at a window boundary | stored in Redis | several instances must share one limit |

**Fixed window** counts requests in the current window and resets when it elapses. Cheap
and easy to reason about. Its weakness: a client can send `limit` requests just before the
boundary and `limit` more just after, so a determined caller gets 2× the intended rate for
a moment.

**Token bucket** holds up to `capacity` tokens and refills continuously at `refillRate` per
second. Each request spends one token. This allows a controlled burst and then settles to a
steady rate — usually the nicer behaviour for a public API. Tokens are tracked as a float
and refilled lazily on each call, so there's no per-key goroutine.

**Redis fixed window** is the same algorithm as `FixedWindow`, but the counter lives in
Redis so every app instance sees the same number.

## Design decisions & trade-offs

**One interface returning a `Result`, not a bare `bool`.** The middleware needs `Remaining`
and `RetryAfter` to set response headers; returning a struct means it gets them without a
second call. `Allow` also takes a `context.Context` and returns an `error` purely for the
Redis backend — the in-memory ones never block and never fail. The cost is that in-memory
callers handle an error that is always `nil`; I think matching the interface to the backend
that actually does I/O is worth that.

**In-memory vs Redis is a real fork, not a default.** In-memory limiting is a sub-microsecond
map lookup with zero dependencies, but the limit is per-process — three replicas means three
times the configured rate. Redis gives you one shared limit that survives deploys, at the
cost of a round trip (~0.5 ms) and an operational dependency. The demo defaults to in-memory
because that's the honest default for a single binary.

**Fixed window as the default despite the boundary burst.** The 2× edge burst is a genuine
downside, but fixed window's state is one integer and one timestamp per key and the logic
fits on a screen. A sliding-window counter removes most of the burst for a small amount of
extra math; a sliding-window log removes all of it but stores a timestamp per request. Those
are listed under future work — I didn't want to ship a more complex default without a
benchmark asking for it.

**The Redis check is one Lua script, not `INCR` then `EXPIRE`.** Issued as two commands, a
crash in between leaves a key with no TTL that never resets — a client permanently locked
out. Redis runs a script atomically, so `INCR` + `PEXPIRE` + `PTTL` either all happen or
none do. Rejected requests still increment the counter (it climbs past the limit until the
window expires); DECR-ing on rejection is more code for no real benefit, since the key
expires on schedule regardless.

**Keying by `RemoteAddr` only.** Behind a proxy or load balancer, every request arrives with
the proxy's IP, so everyone shares one bucket. The fix is to trust `X-Forwarded-For` /
`X-Real-IP` — but *only* from known proxies, because otherwise any client can spoof its key
and dodge the limit entirely. Doing that safely needs a trusted-CIDR allowlist, which is
deployment-specific, so it's left out. The middleware limits on whatever string you give the
limiter; swapping IP for an API key or user ID is a few lines.

**One mutex per limiter.** Every `Allow` takes a single lock around a map operation. Simple,
obviously correct, and fine into the tens of thousands of requests per second. Past that the
map would need lock striping by key hash. I'd rather add that when a profile shows lock
contention than carry the complexity speculatively.

**Background eviction over lazy eviction.** The maps are keyed by client IP and would
otherwise grow forever — also a cheap denial-of-service, since spraying requests from many
source addresses inflates the map. A per-limiter goroutine sweeps entries once they've
returned to a state indistinguishable from a fresh one (window elapsed, or bucket refilled
to full). Evicting lazily on write instead would put an O(n) scan on the hot path, so a
timer wins. `Stop()` ends the goroutine and is safe to call more than once.

**Fail closed on backend error.** If the Redis backend errors, the middleware returns `500`
and the request does not reach the handler. The alternative — let it through — trades
protection for availability. Blocking is the safer default; a production service would make
this a config switch.

## Project layout

```
cmd/server/        demo HTTP service (flag parsing, backend selection)
limiter/           RateLimiter interface + the three backends
middleware/        net/http middleware
internal/server/   demo routing and wiring (not meant for reuse)
```

## Testing

```bash
go test -race ./...
```

- The Redis backend is tested against [`miniredis`](https://github.com/alicebob/miniredis),
  an in-process Redis fake, so neither local runs nor CI need a real server. The trade is
  that it's not exercising a real Redis; the command surface used here (`INCR`, `PEXPIRE`,
  `PTTL`, `EVAL`) is small and miniredis models it faithfully.
- Concurrency tests fire 20 goroutines at a limit of 5 and assert that exactly 5 are
  allowed.
- CI (GitHub Actions) runs `gofmt`, `go vet`, `go build`, and `go test -race` on every push
  and pull request.

## Limitations & what I'd do next

- **Sliding-window counter backend** — removes most of the fixed-window boundary burst
  without storing a per-request log.
- **Redis token bucket** — same atomic-Lua pattern, for smoothed limits shared across
  instances.
- **Proxy-aware key extraction** — `X-Forwarded-For` parsing gated by a trusted-CIDR list.
- **Configurable fail-open / fail-closed** on backend errors.
- **Tiered limits** — different limits per route or per caller class (anonymous vs
  authenticated).
- **Metrics** — Prometheus counters for allowed / denied.
- **Benchmarks** — `go test -bench` to put numbers behind the "one mutex is fine" claim.
