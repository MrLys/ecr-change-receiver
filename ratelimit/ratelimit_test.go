package ratelimit

import (
	"fmt"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	// Test code here
	r := RateLimiter{Limit: 3}
	for i := 0; i < 3; i++ {
		if r.RateLimitsExceeded("1.1.1.1") {
			t.Fatalf("Rate limit exceeded before limit reached %d %d", i, r.Limit)
		}
	}

	if !r.RateLimitsExceeded("1.1.1.1") {
		t.Fatalf("Rate limit not exceeded after limit reached %d %d", 3, r.Limit)
	}
}

func TestRateLimit(t *testing.T) {
	// Test code here
	r := RateLimiter{Limit: 10}
	rateLimitEntry := rateLimitEntry{
		remoteAddr: "1.1.1.1",
		limit:      0,
		start:      time.Now(),
	}
	for i := 0; i < 10; i++ {
		rateLimitEntry.start = time.Now()
		if checkRateLimit(&rateLimitEntry, &r) {
			t.Fatalf("Rate limit exceeded before limit reached %d %d", i, r.Limit)
		}
	}
	rateLimitEntry.start = time.Now()
	fmt.Printf("%v+\n", rateLimitEntry)
	if !checkRateLimit(&rateLimitEntry, &r) {
		t.Fatalf("Rate limit not exceeded after limit reached %d %d", 10, r.Limit)
	}
}
