package limiter

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestFixedWindow_FirstRequestAllowed(t *testing.T) {
	fw := NewFixedWindow(3, time.Second)

	result, err := fw.Allow(context.Background(), "user1")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected first request to be allowed, got denied")
	}
	if result.Remaining != 2 {
		t.Errorf("expected 2 remaining, got %d", result.Remaining)
	}
}

func TestFixedWindow(t *testing.T) {
	fw := NewFixedWindow(3, time.Second)
	expected := []bool{true, true, true, false}
	for i, want := range expected {
		result, err := fw.Allow(context.Background(), "user1")
		if err != nil {
			t.Errorf("request %d: got unexpected error: %v", i+1, err)
		}
		if result.Allowed != want {
			t.Errorf("request %d: expected allowed=%v, got %v", i+1, want, result.Allowed)
		}
	}
}

func TestWindowExpiry(t *testing.T) {
	fw := NewFixedWindow(3, 50*time.Millisecond)
	expected := []bool{true, true, true, false}
	for i, want := range expected {
		result, err := fw.Allow(context.Background(), "user1")
		if err != nil {
			t.Errorf("request %d: got unexpected error: %v", i+1, err)
		}
		if result.Allowed != want {
			t.Errorf("request %d: expected allowed=%v, got %v", i+1, want, result.Allowed)
		}
	}
	time.Sleep(60 * time.Millisecond)

	// confirm a new window has started
	result, err := fw.Allow(context.Background(), "user1")
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Errorf("expected request to be allowed after window reset, got denied")
	}
}

func TestFixedWindow_DeniedRetryAfter(t *testing.T) {
	fw := NewFixedWindow(1, time.Second)

	// use up the single allowed request
	fw.Allow(context.Background(), "user1")

	// this one should be denied with a non-zero RetryAfter
	result, _ := fw.Allow(context.Background(), "user1")

	if result.Allowed {
		t.Errorf("expected request to be denied")
	}
	if result.RetryAfter <= 0 {
		t.Errorf("expected positive RetryAfter, got %v", result.RetryAfter)
	}
}

func TestFixedWindow_Concurrent(t *testing.T) {
	fw := NewFixedWindow(5, time.Second)

	results := make([]bool, 20)
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			result, _ := fw.Allow(context.Background(), "same-key")
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
