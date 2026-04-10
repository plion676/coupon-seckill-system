package ratelimit

import (
	"sync"
	"time"
)

type TokenBucket struct {
	Capacity   int64
	Rate       int64
	Current    int64
	LastRefill time.Time
	Mutex      sync.RWMutex
}

func NewTokenBucket(capacity int64, rate int64) *TokenBucket {
	return &TokenBucket{
		Capacity:   capacity,
		Rate:       rate,
		Current:    capacity,
		LastRefill: time.Now(),
	}
}

func (bucket *TokenBucket) SetRate(newRate int64) {
	bucket.Mutex.Lock()
	defer bucket.Mutex.Unlock()
	bucket.Rate = newRate
}

func (bucket *TokenBucket) SetCapacity(newCapacity int64) {
	bucket.Mutex.Lock()
	defer bucket.Mutex.Unlock()
	bucket.Capacity = newCapacity
	if bucket.Current > newCapacity {
		bucket.Current = newCapacity
	}
}

func (bucket *TokenBucket) TryConsume(numTokens int64) bool {
	bucket.Mutex.Lock()
	defer bucket.Mutex.Unlock()
	now := time.Now()
	timeDiff := now.Sub(bucket.LastRefill)
	newTokens := int64(timeDiff.Seconds()) * bucket.Rate
	if newTokens > 0 {
		bucket.Current += newTokens
		if bucket.Current > bucket.Capacity {
			bucket.Current = bucket.Capacity
		}
		bucket.LastRefill = now
	}

	if bucket.Current >= numTokens {
		bucket.Current -= numTokens
		return true
	}
	return false
}
