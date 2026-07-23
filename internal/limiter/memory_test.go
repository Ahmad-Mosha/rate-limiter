package limiter

import (
	"sync"
	"testing"
	"time"
)

func TestFixedWindow_FirstRequestAllowed(t *testing.T) {
	fw := NewFixedWindow(3, time.Second)

	allowed, err := fw.Allow("user1")

	if err != nil {
		t.Errorf("expcted no error , got %v", err)
	}
	if !allowed {
		t.Errorf("expected fisrt request to be allowed, got denied")
	}
}

func TestFixedWindow(t *testing.T) {
	fw := NewFixedWindow(3, time.Second)
	expected := []bool{true, true, true, false}
	for i, want := range expected {
		allowed, err := fw.Allow("user1")
		if err != nil {
			t.Errorf("request %d: got unexptected error: %v", i+1, err)
		}
		if allowed != want {
			t.Errorf("request %d: expected allowed =%v, got %v", i+1, want, allowed)
		}
	}

}

func TestWindowExpiry(t *testing.T) {
	fw := NewFixedWindow(3, 50*time.Millisecond)
	expected := []bool{true, true, true, false}
	for i, want := range expected {
		allowed, err := fw.Allow("user1")
		if err != nil {
			t.Errorf("request %d: got unexptected error: %v", i+1, err)
		}
		if allowed != want {
			t.Errorf("request %d: expected allowed =%v, got %v", i+1, want, allowed)
		}
	}
	time.Sleep(60 * time.Millisecond)

	// Phase 3: confirm a new window has started
	allowed, err := fw.Allow("user1")
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected request to be allowed after window reset, got denied")
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
			allowed, _ := fw.Allow("same-key")
			results[index] = allowed
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
