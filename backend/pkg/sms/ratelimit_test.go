package sms

import (
	"testing"
	"time"
)

func TestInMemoryRateLimiter_Allow_FirstMessage(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 5,
		MaxMessagesPerHour:   100,
		MaxMessagesPerDay:    1000,
	}
	limiter := NewInMemoryRateLimiter(config)

	allowed := limiter.Allow("+251912345678")
	if !allowed {
		t.Error("Allow() should return true for first message")
	}
}

func TestInMemoryRateLimiter_Allow_MinuteLimitExceeded(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 2,
		MaxMessagesPerHour:   100,
		MaxMessagesPerDay:    1000,
	}
	limiter := NewInMemoryRateLimiter(config)
	phone := "+251912345678"

	limiter.Allow(phone)
	limiter.Record(phone)
	limiter.Allow(phone)
	limiter.Record(phone)

	allowed := limiter.Allow(phone)
	if allowed {
		t.Error("Allow() should return false when minute limit exceeded")
	}
}

func TestInMemoryRateLimiter_Allow_HourLimitExceeded(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 100,
		MaxMessagesPerHour:   2,
		MaxMessagesPerDay:    1000,
	}
	limiter := NewInMemoryRateLimiter(config)
	phone := "+251912345678"

	limiter.Allow(phone)
	limiter.Record(phone)
	limiter.Allow(phone)
	limiter.Record(phone)

	allowed := limiter.Allow(phone)
	if allowed {
		t.Error("Allow() should return false when hour limit exceeded")
	}
}

func TestInMemoryRateLimiter_Allow_DayLimitExceeded(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 100,
		MaxMessagesPerHour:   100,
		MaxMessagesPerDay:    2,
	}
	limiter := NewInMemoryRateLimiter(config)
	phone := "+251912345678"

	limiter.Allow(phone)
	limiter.Record(phone)
	limiter.Allow(phone)
	limiter.Record(phone)

	allowed := limiter.Allow(phone)
	if allowed {
		t.Error("Allow() should return false when day limit exceeded")
	}
}

func TestInMemoryRateLimiter_Record_IncrementsCounters(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 5,
		MaxMessagesPerHour:   100,
		MaxMessagesPerDay:    100,
	}
	limiter := NewInMemoryRateLimiter(config)
	phone := "+251912345678"

	limiter.Allow(phone)
	limiter.Record(phone)

	limiter.Allow(phone)
	limiter.Record(phone)

	limiter.Allow(phone)
	limiter.Record(phone)

	limiter.Allow(phone)
	limiter.Record(phone)

	limiter.Allow(phone)
	limiter.Record(phone)

	allowed := limiter.Allow(phone)
	if allowed {
		t.Error("Allow() should return false after 5 messages (minute limit)")
	}
}

func TestInMemoryRateLimiter_Record_RequiresAllowFirst(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 1,
		MaxMessagesPerHour:   1,
		MaxMessagesPerDay:    1,
	}
	limiter := NewInMemoryRateLimiter(config)
	phone := "+251912345678"

	limiter.Record(phone)
	limiter.Record(phone)

	allowed := limiter.Allow(phone)
	if !allowed {
		t.Error("Allow() should return true when Record was called before Allow (no entry created)")
	}
}

func TestInMemoryRateLimiter_Reset_ClearsLimits(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 1,
		MaxMessagesPerHour:   1,
		MaxMessagesPerDay:    1,
	}
	limiter := NewInMemoryRateLimiter(config)
	phone := "+251912345678"

	limiter.Allow(phone)
	limiter.Record(phone)

	allowed := limiter.Allow(phone)
	if allowed {
		t.Error("Allow() should return false before reset")
	}

	limiter.Reset(phone)

	allowed = limiter.Allow(phone)
	if !allowed {
		t.Error("Allow() should return true after reset")
	}
}

func TestInMemoryRateLimiter_Cleanup_RemovesOldEntries(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 100,
		MaxMessagesPerHour:   100,
		MaxMessagesPerDay:    100,
	}
	limiter := NewInMemoryRateLimiter(config)

	limiter.Allow("+251912345678")
	limiter.Allow("+251923456789")

	limiter.mutex.Lock()
	for _, pl := range limiter.phoneLimits {
		pl.lastDay = time.Now().Add(-49 * time.Hour)
	}
	limiter.mutex.Unlock()

	limiter.Cleanup()

	limiter.mutex.RLock()
	count := len(limiter.phoneLimits)
	limiter.mutex.RUnlock()

	if count != 0 {
		t.Errorf("Cleanup() should remove old entries, got %d entries", count)
	}
}

func TestInMemoryRateLimiter_MultiplePhones(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 1,
		MaxMessagesPerHour:   1,
		MaxMessagesPerDay:    1,
	}
	limiter := NewInMemoryRateLimiter(config)

	phone1 := "+251912345678"
	phone2 := "+251923456789"

	limiter.Allow(phone1)
	limiter.Record(phone1)

	allowed1 := limiter.Allow(phone1)
	allowed2 := limiter.Allow(phone2)

	if allowed1 {
		t.Error("Allow() should return false for phone1 after limit")
	}
	if !allowed2 {
		t.Error("Allow() should return true for phone2 (different phone)")
	}
}

func TestInMemoryOptOutStore_IsOptedOut(t *testing.T) {
	store := NewInMemoryOptOutStore()
	phone := "+251912345678"

	if store.IsOptedOut(phone) {
		t.Error("IsOptedOut() should return false for phone not opted out")
	}

	store.AddOptOut(phone)

	if !store.IsOptedOut(phone) {
		t.Error("IsOptedOut() should return true for opted out phone")
	}
}

func TestInMemoryOptOutStore_AddOptOut(t *testing.T) {
	store := NewInMemoryOptOutStore()
	phone := "+251912345678"

	store.AddOptOut(phone)

	if !store.IsOptedOut(phone) {
		t.Error("AddOptOut() should add phone to opt-out list")
	}
}

func TestInMemoryOptOutStore_RemoveOptOut(t *testing.T) {
	store := NewInMemoryOptOutStore()
	phone := "+251912345678"

	store.AddOptOut(phone)
	if !store.IsOptedOut(phone) {
		t.Fatal("AddOptOut() failed")
	}

	store.RemoveOptOut(phone)

	if store.IsOptedOut(phone) {
		t.Error("RemoveOptOut() should remove phone from opt-out list")
	}
}

func TestInMemoryOptOutStore_ListOptOuts(t *testing.T) {
	store := NewInMemoryOptOutStore()

	phones := []string{"+251912345678", "+251923456789", "+251934567890"}
	for _, p := range phones {
		store.AddOptOut(p)
	}

	list := store.ListOptOuts()

	if len(list) != len(phones) {
		t.Errorf("ListOptOuts() returned %d entries, want %d", len(list), len(phones))
	}

	found := make(map[string]bool)
	for _, p := range list {
		found[p] = true
	}

	for _, p := range phones {
		if !found[p] {
			t.Errorf("ListOptOuts() missing phone %s", p)
		}
	}
}

func TestInMemoryOptOutStore_RemoveNonExistent(t *testing.T) {
	store := NewInMemoryOptOutStore()

	store.RemoveOptOut("+251912345678")

	if store.IsOptedOut("+251912345678") {
		t.Error("RemoveOptOut() for non-existent phone should not cause issues")
	}
}

func TestInMemoryRateLimiter_DisabledLimits(t *testing.T) {
	config := RateLimiterConfig{
		MaxMessagesPerMinute: 0,
		MaxMessagesPerHour:   0,
		MaxMessagesPerDay:    0,
	}
	limiter := NewInMemoryRateLimiter(config)
	phone := "+251912345678"

	limiter.Allow(phone)
	for i := 0; i < 100; i++ {
		limiter.Record(phone)
	}

	allowed := limiter.Allow(phone)
	if !allowed {
		t.Error("Allow() should return true when all limits are disabled (0)")
	}
}
