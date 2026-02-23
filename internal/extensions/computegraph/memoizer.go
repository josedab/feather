package computegraph

import (
	"sync"
	"time"
)

// MemoizerConfig holds tunables for the computation memoizer.
type MemoizerConfig struct {
	MaxEntries int
	DefaultTTL time.Duration
	Enabled    bool
}

// DefaultMemoizerConfig returns sensible defaults for the memoizer.
func DefaultMemoizerConfig() MemoizerConfig {
	return MemoizerConfig{
		MaxEntries: 10000,
		DefaultTTL: 5 * time.Minute,
		Enabled:    true,
	}
}

// memoEntry is a single cached computation result with expiration.
type memoEntry struct {
	value     interface{}
	expiresAt time.Time
}

// MemoizerStats reports hit/miss statistics for the memoizer.
type MemoizerStats struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Entries    int   `json:"entries"`
	Evictions  int64 `json:"evictions"`
	MaxEntries int   `json:"max_entries"`
}

// Memoizer caches computation results keyed by node name + input hash.
// It is safe for concurrent use.
type Memoizer struct {
	mu        sync.RWMutex
	config    MemoizerConfig
	entries   map[string]*memoEntry
	hits      int64
	misses    int64
	evictions int64
}

// NewMemoizer creates a new memoizer with the given configuration.
func NewMemoizer(cfg MemoizerConfig) *Memoizer {
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = 10000
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 5 * time.Minute
	}
	return &Memoizer{
		config:  cfg,
		entries: make(map[string]*memoEntry),
	}
}

// Get retrieves a cached value by key. Returns the value and true if found
// and not expired, or nil and false otherwise.
func (m *Memoizer) Get(key string) (interface{}, bool) {
	if !m.config.Enabled {
		return nil, false
	}

	m.mu.RLock()
	entry, ok := m.entries[key]
	m.mu.RUnlock()

	if !ok {
		m.mu.Lock()
		m.misses++
		m.mu.Unlock()
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		m.mu.Lock()
		delete(m.entries, key)
		m.misses++
		m.evictions++
		m.mu.Unlock()
		return nil, false
	}

	m.mu.Lock()
	m.hits++
	m.mu.Unlock()
	return entry.value, true
}

// Put stores a value in the cache with the default TTL. If the cache is full,
// expired entries are evicted first; if still full, the oldest entry is removed.
func (m *Memoizer) Put(key string, value interface{}) {
	if !m.config.Enabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Evict expired entries if at capacity.
	if len(m.entries) >= m.config.MaxEntries {
		if _, exists := m.entries[key]; !exists {
			m.evictExpired()
		}
	}

	// If still at capacity, evict the oldest entry.
	if len(m.entries) >= m.config.MaxEntries {
		if _, exists := m.entries[key]; !exists {
			m.evictOldest()
		}
	}

	m.entries[key] = &memoEntry{
		value:     value,
		expiresAt: time.Now().Add(m.config.DefaultTTL),
	}
}

// Invalidate removes the specified keys from the cache.
func (m *Memoizer) Invalidate(keys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range keys {
		delete(m.entries, key)
	}
}

// Stats returns current hit/miss statistics.
func (m *Memoizer) Stats() MemoizerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MemoizerStats{
		Hits:       m.hits,
		Misses:     m.misses,
		Entries:    len(m.entries),
		Evictions:  m.evictions,
		MaxEntries: m.config.MaxEntries,
	}
}

// evictExpired removes all entries whose TTL has passed. Must be called with mu held.
func (m *Memoizer) evictExpired() {
	now := time.Now()
	for k, e := range m.entries {
		if now.After(e.expiresAt) {
			delete(m.entries, k)
			m.evictions++
		}
	}
}

// evictOldest removes the entry with the earliest expiration. Must be called with mu held.
func (m *Memoizer) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, e := range m.entries {
		if first || e.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.expiresAt
			first = false
		}
	}

	if oldestKey != "" {
		delete(m.entries, oldestKey)
		m.evictions++
	}
}
