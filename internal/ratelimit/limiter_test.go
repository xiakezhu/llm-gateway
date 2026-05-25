package ratelimit

import (
	"testing"
	"time"
)

func TestManagerAllowWithinLimit(t *testing.T) {
	manager := NewManager()

	if got := manager.Allow("key_1", 2); got != AllowResultAllowed {
		t.Fatalf("expected first request to be allowed")
	}
	if got := manager.Allow("key_1", 2); got != AllowResultAllowed {
		t.Fatalf("expected second request to be allowed")
	}
}

func TestManagerRejectsOverLimit(t *testing.T) {
	manager := NewManager()

	if got := manager.Allow("key_1", 1); got != AllowResultAllowed {
		t.Fatalf("expected first request to be allowed")
	}
	if got := manager.Allow("key_1", 1); got != AllowResultOverLimit {
		t.Fatalf("expected second immediate request to be over limit, got %v", got)
	}
}

func TestManagerTracksKeysIndependently(t *testing.T) {
	manager := NewManager()

	if got := manager.Allow("key_1", 1); got != AllowResultAllowed {
		t.Fatalf("expected first key request to be allowed")
	}
	if got := manager.Allow("key_2", 1); got != AllowResultAllowed {
		t.Fatalf("expected second key request to be allowed")
	}
}

func TestManagerRecreatesLimiterWhenRPMLimitChanges(t *testing.T) {
	manager := NewManager()

	if got := manager.Allow("key_1", 1); got != AllowResultAllowed {
		t.Fatalf("expected first request to be allowed")
	}
	if got := manager.Allow("key_1", 1); got != AllowResultOverLimit {
		t.Fatalf("expected second immediate request to be over limit, got %v", got)
	}
	if got := manager.Allow("key_1", 2); got != AllowResultAllowed {
		t.Fatalf("expected request to be allowed after rpm limit increase")
	}
}

func TestManagerRejectsInvalidInputs(t *testing.T) {
	manager := NewManager()

	if got := manager.Allow("", 1); got != AllowResultInvalidKey {
		t.Fatalf("expected empty key id to be invalid key, got %v", got)
	}
	if got := manager.Allow("key_1", 0); got != AllowResultInvalidLimit {
		t.Fatalf("expected zero rpm limit to be invalid limit, got %v", got)
	}
}

func TestManagerEvictsStaleLimitersAfterTTL(t *testing.T) {
	clock := newFakeClock()
	manager := newManagerWithConfig(managerConfig{
		limiterTTL:       time.Minute,
		cleanupInterval:  time.Second,
		cleanupBatchSize: 10,
		now:              clock.Now,
	})

	if got := manager.Allow("stale_key", 1); got != AllowResultAllowed {
		t.Fatalf("expected stale_key request to be allowed, got %v", got)
	}

	clock.Advance(59 * time.Second)
	if got := manager.Allow("fresh_key", 1); got != AllowResultAllowed {
		t.Fatalf("expected fresh_key request to be allowed, got %v", got)
	}
	if got := limiterCount(manager); got != 2 {
		t.Fatalf("expected no TTL eviction before limiter is stale, got %d limiters", got)
	}

	clock.Advance(2 * time.Second)
	if got := manager.Allow("new_key", 1); got != AllowResultAllowed {
		t.Fatalf("expected new_key request to be allowed, got %v", got)
	}

	if hasLimiter(manager, "stale_key") {
		t.Fatalf("expected stale limiter to be evicted")
	}
	if !hasLimiter(manager, "fresh_key") {
		t.Fatalf("expected fresh limiter to remain")
	}
	if !hasLimiter(manager, "new_key") {
		t.Fatalf("expected new limiter to be tracked")
	}
}

func TestManagerCleanupIsBounded(t *testing.T) {
	clock := newFakeClock()
	manager := newManagerWithConfig(managerConfig{
		limiterTTL:       time.Second,
		cleanupInterval:  time.Second,
		cleanupBatchSize: 2,
		now:              clock.Now,
	})

	for _, keyID := range []string{"stale_1", "stale_2", "stale_3"} {
		if got := manager.Allow(keyID, 1); got != AllowResultAllowed {
			t.Fatalf("expected %s request to be allowed, got %v", keyID, got)
		}
	}

	clock.Advance(2 * time.Second)
	if got := manager.Allow("active_1", 1); got != AllowResultAllowed {
		t.Fatalf("expected active_1 request to be allowed, got %v", got)
	}

	if hasLimiter(manager, "stale_1") || hasLimiter(manager, "stale_2") {
		t.Fatalf("expected first two stale limiters to be evicted")
	}
	if !hasLimiter(manager, "stale_3") {
		t.Fatalf("expected cleanup to leave stale_3 because batch size is bounded")
	}
	if got := limiterCount(manager); got != 2 {
		t.Fatalf("expected one stale limiter plus active limiter, got %d limiters", got)
	}

	clock.Advance(time.Second)
	if got := manager.Allow("active_2", 1); got != AllowResultAllowed {
		t.Fatalf("expected active_2 request to be allowed, got %v", got)
	}
	if hasLimiter(manager, "stale_3") {
		t.Fatalf("expected remaining stale limiter to be evicted on next cleanup")
	}
}

type fakeClock struct {
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1710000000, 0)}
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func limiterCount(manager *Manager) int {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	return len(manager.limiters)
}

func hasLimiter(manager *Manager, keyID string) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	_, ok := manager.limiters[keyID]
	return ok
}
