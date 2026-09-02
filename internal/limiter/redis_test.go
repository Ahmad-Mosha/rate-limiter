package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis spins up an in-process Redis fake so the tests need no running
// server. miniredis.RunT tears it down when the test finishes.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return client
}

func TestRedisFixedWindow_BasicExhaustion(t *testing.T) {
	rl := NewRedisFixedWindow(newTestRedis(t), 3, time.Minute)

	expected := []bool{true, true, true, false}
	for i, want := range expected {
		result, err := rl.Allow(context.Background(), "user1")
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i+1, err)
		}
		if result.Allowed != want {
			t.Errorf("request %d: expected allowed=%v, got %v", i+1, want, result.Allowed)
		}
	}
}

func TestRedisFixedWindow_KeysAreIsolated(t *testing.T) {
	rl := NewRedisFixedWindow(newTestRedis(t), 1, time.Minute)

	if r, _ := rl.Allow(context.Background(), "userA"); !r.Allowed {
		t.Fatal("userA first request should be allowed")
	}
	if r, _ := rl.Allow(context.Background(), "userB"); !r.Allowed {
		t.Error("userB should not be affected by userA's usage")
	}
}

func TestRedisFixedWindow_WindowReset(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	rl := NewRedisFixedWindow(client, 2, time.Minute)

	rl.Allow(context.Background(), "user1")
	rl.Allow(context.Background(), "user1")
	if r, _ := rl.Allow(context.Background(), "user1"); r.Allowed {
		t.Fatal("third request in the window should be denied")
	}

	mr.FastForward(time.Minute + time.Second) // let the window expire

	if r, _ := rl.Allow(context.Background(), "user1"); !r.Allowed {
		t.Error("request after the window expired should be allowed")
	}
}

func TestRedisFixedWindow_DeniedRetryAfter(t *testing.T) {
	rl := NewRedisFixedWindow(newTestRedis(t), 1, time.Minute)

	rl.Allow(context.Background(), "user1")
	result, _ := rl.Allow(context.Background(), "user1")

	if result.Allowed {
		t.Fatal("expected request to be denied")
	}
	if result.RetryAfter <= 0 || result.RetryAfter > time.Minute {
		t.Errorf("expected RetryAfter within the window, got %v", result.RetryAfter)
	}
}
