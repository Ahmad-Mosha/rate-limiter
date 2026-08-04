package limiter

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBucket_BasicExhaustion(t *testing.T) {
	b := NewBucket(3, 1) // capacity 3, refill 1 token/sec

	expected := []bool{true, true, true, false}
	for i, want := range expected {
		result, err := b.Allow(context.Background(), "user1")
		if err != nil {
			t.Errorf("request %d: got unexpected error: %v", i+1, err)
		}
		if result.Allowed != want {
			t.Errorf("request %d: expected allowed=%v, got %v", i+1, want, result.Allowed)
		}
	}
}

func TestBucket_RefillOverTime(t *testing.T) {
	// capacity 2, refill 20 tokens/sec -> refills fast so the test stays quick
	b := NewBucket(2, 20)

	// drain the bucket
	for i := 0; i < 2; i++ {
		result, err := b.Allow(context.Background(), "user1")
		if err != nil {
			t.Errorf("drain request %d: got unexpected error: %v", i+1, err)
		}
		if !result.Allowed {
			t.Errorf("drain request %d: expected allowed, got denied", i+1)
		}
	}

	// bucket should now be empty
	result, err := b.Allow(context.Background(), "user1")
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
	}
	if result.Allowed {
		t.Errorf("expected request to be denied immediately after draining bucket")
	}

	// wait for refill (at 20 tokens/sec, 100ms should add ~2 tokens)
	time.Sleep(100 * time.Millisecond)

	result, err = b.Allow(context.Background(), "user1")
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected request to be allowed after refill, got denied")
	}
}

func TestBucket_DeniedRetryAfter(t *testing.T) {
	b := NewBucket(1, 10) // capacity 1, refill 10 tokens/sec

	// use up the single token
	b.Allow(context.Background(), "user1")

	// this one should be denied with a non-zero RetryAfter
	result, _ := b.Allow(context.Background(), "user1")

	if result.Allowed {
		t.Errorf("expected request to be denied")
	}
	if result.RetryAfter <= 0 {
		t.Errorf("expected positive RetryAfter, got %v", result.RetryAfter)
	}
}

func TestBucket_Concurrent(t *testing.T) {
	b := NewBucket(5, 0) // capacity 5, no refill during the test

	results := make([]bool, 20)
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result, _ := b.Allow(context.Background(), "same-key")
			results[index] = result.Allowed
		}(i)
	}

	wg.Wait()

	allowedCount := 0
	for _, wasAllowed := range results {
		if wasAllowed {
			allowedCount++
		}
	}

	if allowedCount != 5 {
		t.Errorf("expected exactly 5 allowed requests, got %d", allowedCount)
	}
}