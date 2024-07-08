package ratelimit

import (
	"sync"
	"time"
)

type RateLimiter struct {
	rateLimits sync.Map
	Limit      int
}
type rateLimitEntry struct {
	remoteAddr string
	limit      int
	start      time.Time
}

func (r *RateLimiter) RateLimitsExceeded(remateAddr string) bool {
	limit, ok := r.rateLimits.Load(remateAddr)
	if limit == nil || !ok {
		rateLimit := rateLimitEntry{remateAddr, 1, time.Now()}
		r.rateLimits.Store(remateAddr, rateLimit)
		return false
	}
	rateLimit := limit.(rateLimitEntry)
	if checkRateLimit(&rateLimit, r) {
		return true
	}
	// update time to nearest minute
	rateLimit.start = time.Now().Truncate(time.Minute)
	r.rateLimits.Store(remateAddr, rateLimit)
	return false
}

func checkRateLimit(rl *rateLimitEntry, r *RateLimiter) bool {
	rl.limit++
	if time.Since(rl.start) > 1*time.Minute {
		rl.limit = 0
	} else if rl.limit > r.Limit {
		// rate limit exceeded
		return true
	}
	return false
}
