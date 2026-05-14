package nonces

import (
	"sync"
	"testing"
)

func TestTrackerNextPeekAndSet(t *testing.T) {
	tracker := New(nil)
	address := "erd1qqqq"

	tracker.Set(address, 42)

	if got := tracker.Peek(address); got != 42 {
		t.Fatalf("expected peek nonce 42, got %d", got)
	}
	if got := tracker.Next(address); got != 42 {
		t.Fatalf("expected next nonce 42, got %d", got)
	}
	if got := tracker.Peek(address); got != 43 {
		t.Fatalf("expected peek nonce 43, got %d", got)
	}
}

func TestTrackerNextShouldBeConcurrencySafe(t *testing.T) {
	tracker := New(nil)
	address := "erd1qqqq"
	const numCalls = 100

	wg := sync.WaitGroup{}
	results := make(chan uint64, numCalls)
	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- tracker.Next(address)
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[uint64]struct{}, numCalls)
	for nonce := range results {
		seen[nonce] = struct{}{}
	}

	if len(seen) != numCalls {
		t.Fatalf("expected %d unique nonces, got %d", numCalls, len(seen))
	}
	if got := tracker.Peek(address); got != numCalls {
		t.Fatalf("expected next nonce %d, got %d", numCalls, got)
	}
}
