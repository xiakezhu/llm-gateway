package ratelimit

import (
	"container/list"
	"sync"
	"time"
)

const (
	defaultLimiterTTL       = 10 * time.Minute
	defaultCleanupInterval  = time.Minute
	defaultCleanupBatchSize = 128
)

type Manager struct {
	mu               sync.Mutex
	limiters         map[string]*keyedLimiter
	lru              *list.List
	limiterTTL       time.Duration
	cleanupInterval  time.Duration
	cleanupBatchSize int
	nextCleanup      time.Time
	now              func() time.Time
}

type keyedLimiter struct {
	key      string
	rpm      int
	tokens   float64
	lastFill time.Time
	lastSeen time.Time
	element  *list.Element
}

type managerConfig struct {
	limiterTTL       time.Duration
	cleanupInterval  time.Duration
	cleanupBatchSize int
	now              func() time.Time
}

type AllowResult int

const (
	AllowResultAllowed AllowResult = iota
	AllowResultOverLimit
	AllowResultInvalidKey
	AllowResultInvalidLimit
)

func (r AllowResult) Allowed() bool {
	return r == AllowResultAllowed
}

func NewManager() *Manager {
	return newManagerWithConfig(managerConfig{})
}

func newManagerWithConfig(cfg managerConfig) *Manager {
	if cfg.limiterTTL <= 0 {
		cfg.limiterTTL = defaultLimiterTTL
	}
	if cfg.cleanupInterval <= 0 {
		cfg.cleanupInterval = defaultCleanupInterval
	}
	if cfg.cleanupBatchSize <= 0 {
		cfg.cleanupBatchSize = defaultCleanupBatchSize
	}
	if cfg.now == nil {
		cfg.now = time.Now
	}

	now := cfg.now()
	return &Manager{
		limiters:         make(map[string]*keyedLimiter),
		lru:              list.New(),
		limiterTTL:       cfg.limiterTTL,
		cleanupInterval:  cfg.cleanupInterval,
		cleanupBatchSize: cfg.cleanupBatchSize,
		nextCleanup:      now.Add(cfg.cleanupInterval),
		now:              cfg.now,
	}
}

func (m *Manager) Allow(keyID string, rpmLimit int) AllowResult {
	if keyID == "" {
		return AllowResultInvalidKey
	}
	if rpmLimit <= 0 {
		return AllowResultInvalidLimit
	}

	now := m.now()
	m.mu.Lock()
	m.cleanupStaleLocked(now)

	entry := m.limiters[keyID]
	if entry == nil || entry.rpm != rpmLimit {
		if entry != nil {
			m.removeLimiterLocked(entry)
		}
		entry = &keyedLimiter{
			key:      keyID,
			rpm:      rpmLimit,
			tokens:   float64(rpmLimit),
			lastFill: now,
			lastSeen: now,
		}
		entry.element = m.lru.PushBack(entry)
		m.limiters[keyID] = entry
	} else {
		entry.refill(now)
		entry.lastSeen = now
		m.lru.MoveToBack(entry.element)
	}

	if entry.tokens < 1 {
		m.mu.Unlock()
		return AllowResultOverLimit
	}
	entry.tokens--
	m.mu.Unlock()

	return AllowResultAllowed
}

func (m *Manager) cleanupStaleLocked(now time.Time) {
	if now.Before(m.nextCleanup) {
		return
	}

	cutoff := now.Add(-m.limiterTTL)
	for cleaned := 0; cleaned < m.cleanupBatchSize; cleaned++ {
		front := m.lru.Front()
		if front == nil {
			break
		}

		entry, ok := front.Value.(*keyedLimiter)
		if !ok || !entry.lastSeen.Before(cutoff) {
			break
		}

		m.removeLimiterLocked(entry)
	}

	m.nextCleanup = now.Add(m.cleanupInterval)
}

func (m *Manager) removeLimiterLocked(entry *keyedLimiter) {
	delete(m.limiters, entry.key)
	if entry.element != nil {
		m.lru.Remove(entry.element)
		entry.element = nil
	}
}

func (l *keyedLimiter) refill(now time.Time) {
	if now.Before(l.lastFill) {
		l.lastFill = now
		return
	}

	elapsed := now.Sub(l.lastFill).Seconds()
	if elapsed <= 0 {
		return
	}

	l.tokens += elapsed * float64(l.rpm) / 60
	if burst := float64(l.rpm); l.tokens > burst {
		l.tokens = burst
	}
	l.lastFill = now
}
