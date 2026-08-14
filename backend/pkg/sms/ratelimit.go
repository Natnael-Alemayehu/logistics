package sms

import (
	"sync"
	"time"
)

type RateLimiterConfig struct {
	MaxMessagesPerMinute int
	MaxMessagesPerHour   int
	MaxMessagesPerDay    int
}

type InMemoryRateLimiter struct {
	config      RateLimiterConfig
	phoneLimits map[string]*phoneLimit
	mutex       sync.RWMutex
}

type phoneLimit struct {
	minuteCount int
	hourCount   int
	dayCount    int
	lastMinute  time.Time
	lastHour    time.Time
	lastDay     time.Time
}

func NewInMemoryRateLimiter(config RateLimiterConfig) *InMemoryRateLimiter {
	return &InMemoryRateLimiter{
		config:      config,
		phoneLimits: make(map[string]*phoneLimit),
	}
}

func (r *InMemoryRateLimiter) Allow(phone string) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	limit, exists := r.phoneLimits[phone]
	if !exists {
		r.phoneLimits[phone] = &phoneLimit{
			minuteCount: 0,
			hourCount:   0,
			dayCount:    0,
			lastMinute:  now,
			lastHour:    now,
			lastDay:     now,
		}
		return true
	}

	if now.Sub(limit.lastMinute) >= time.Minute {
		limit.minuteCount = 0
		limit.lastMinute = now
	}
	if now.Sub(limit.lastHour) >= time.Hour {
		limit.hourCount = 0
		limit.lastHour = now
	}
	if now.Sub(limit.lastDay) >= 24*time.Hour {
		limit.dayCount = 0
		limit.lastDay = now
	}

	if r.config.MaxMessagesPerMinute > 0 && limit.minuteCount >= r.config.MaxMessagesPerMinute {
		return false
	}
	if r.config.MaxMessagesPerHour > 0 && limit.hourCount >= r.config.MaxMessagesPerHour {
		return false
	}
	if r.config.MaxMessagesPerDay > 0 && limit.dayCount >= r.config.MaxMessagesPerDay {
		return false
	}

	return true
}

func (r *InMemoryRateLimiter) Record(phone string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	limit, exists := r.phoneLimits[phone]
	if !exists {
		return
	}

	limit.minuteCount++
	limit.hourCount++
	limit.dayCount++
}

func (r *InMemoryRateLimiter) Reset(phone string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.phoneLimits, phone)
}

func (r *InMemoryRateLimiter) Cleanup() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	for phone, limit := range r.phoneLimits {
		if now.Sub(limit.lastDay) >= 48*time.Hour {
			delete(r.phoneLimits, phone)
		}
	}
}

type InMemoryOptOutStore struct {
	optOuts map[string]bool
	mutex   sync.RWMutex
}

func NewInMemoryOptOutStore() *InMemoryOptOutStore {
	return &InMemoryOptOutStore{
		optOuts: make(map[string]bool),
	}
}

func (s *InMemoryOptOutStore) IsOptedOut(phone string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.optOuts[phone]
}

func (s *InMemoryOptOutStore) AddOptOut(phone string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.optOuts[phone] = true
}

func (s *InMemoryOptOutStore) RemoveOptOut(phone string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.optOuts, phone)
}

func (s *InMemoryOptOutStore) ListOptOuts() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]string, 0, len(s.optOuts))
	for phone := range s.optOuts {
		result = append(result, phone)
	}
	return result
}
