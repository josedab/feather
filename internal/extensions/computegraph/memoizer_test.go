package computegraph

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMemoizer_GetPut(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		value     interface{}
		wantFound bool
	}{
		{
			name:      "store and retrieve string",
			key:       "node_a:hash1",
			value:     "hello",
			wantFound: true,
		},
		{
			name:      "store and retrieve float",
			key:       "node_b:hash2",
			value:     42.5,
			wantFound: true,
		},
		{
			name:      "store and retrieve nil",
			key:       "node_c:hash3",
			value:     nil,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoizer(DefaultMemoizerConfig())
			m.Put(tt.key, tt.value)
			got, ok := m.Get(tt.key)
			if ok != tt.wantFound {
				t.Fatalf("Get(%q) found=%v, want %v", tt.key, ok, tt.wantFound)
			}
			if got != tt.value {
				t.Fatalf("Get(%q) = %v, want %v", tt.key, got, tt.value)
			}
		})
	}
}

func TestMemoizer_MissOnUnknownKey(t *testing.T) {
	m := NewMemoizer(DefaultMemoizerConfig())
	_, ok := m.Get("nonexistent")
	if ok {
		t.Fatal("expected miss on unknown key")
	}
}

func TestMemoizer_TTLExpiration(t *testing.T) {
	cfg := MemoizerConfig{
		MaxEntries: 100,
		DefaultTTL: 50 * time.Millisecond,
		Enabled:    true,
	}
	m := NewMemoizer(cfg)

	m.Put("key1", "value1")

	// Should exist before TTL.
	if _, ok := m.Get("key1"); !ok {
		t.Fatal("expected hit before TTL expiration")
	}

	// Wait for expiration.
	time.Sleep(60 * time.Millisecond)

	if _, ok := m.Get("key1"); ok {
		t.Fatal("expected miss after TTL expiration")
	}
}

func TestMemoizer_Invalidate(t *testing.T) {
	tests := []struct {
		name       string
		putKeys    []string
		invalidate []string
		checkKey   string
		wantFound  bool
	}{
		{
			name:       "invalidate single key",
			putKeys:    []string{"a", "b"},
			invalidate: []string{"a"},
			checkKey:   "a",
			wantFound:  false,
		},
		{
			name:       "other keys unaffected",
			putKeys:    []string{"a", "b"},
			invalidate: []string{"a"},
			checkKey:   "b",
			wantFound:  true,
		},
		{
			name:       "invalidate multiple keys",
			putKeys:    []string{"x", "y", "z"},
			invalidate: []string{"x", "y"},
			checkKey:   "z",
			wantFound:  true,
		},
		{
			name:       "invalidate nonexistent key is safe",
			putKeys:    []string{"a"},
			invalidate: []string{"nope"},
			checkKey:   "a",
			wantFound:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoizer(DefaultMemoizerConfig())
			for _, k := range tt.putKeys {
				m.Put(k, "val_"+k)
			}
			m.Invalidate(tt.invalidate...)
			_, ok := m.Get(tt.checkKey)
			if ok != tt.wantFound {
				t.Fatalf("Get(%q) found=%v, want %v", tt.checkKey, ok, tt.wantFound)
			}
		})
	}
}

func TestMemoizer_Stats(t *testing.T) {
	m := NewMemoizer(DefaultMemoizerConfig())

	m.Put("a", 1)
	m.Put("b", 2)

	// 2 hits
	m.Get("a")
	m.Get("b")

	// 1 miss
	m.Get("c")

	stats := m.Stats()
	if stats.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Entries != 2 {
		t.Fatalf("expected 2 entries, got %d", stats.Entries)
	}
}

func TestMemoizer_MaxEntries(t *testing.T) {
	cfg := MemoizerConfig{
		MaxEntries: 3,
		DefaultTTL: time.Hour,
		Enabled:    true,
	}
	m := NewMemoizer(cfg)

	for i := 0; i < 5; i++ {
		m.Put(fmt.Sprintf("key%d", i), i)
	}

	stats := m.Stats()
	if stats.Entries > cfg.MaxEntries {
		t.Fatalf("entries %d exceeds max %d", stats.Entries, cfg.MaxEntries)
	}
}

func TestMemoizer_Disabled(t *testing.T) {
	cfg := MemoizerConfig{
		MaxEntries: 100,
		DefaultTTL: time.Hour,
		Enabled:    false,
	}
	m := NewMemoizer(cfg)

	m.Put("key", "value")
	_, ok := m.Get("key")
	if ok {
		t.Fatal("expected miss when memoizer is disabled")
	}
}

func TestMemoizer_ConcurrentAccess(t *testing.T) {
	m := NewMemoizer(DefaultMemoizerConfig())
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", n%10)
			m.Put(key, n)
			m.Get(key)
			m.Invalidate(key)
		}(i)
	}

	wg.Wait()

	// Should not panic; stats should be consistent.
	stats := m.Stats()
	if stats.Hits < 0 || stats.Misses < 0 {
		t.Fatal("negative stats after concurrent access")
	}
}

func TestMemoizer_OverwriteKey(t *testing.T) {
	m := NewMemoizer(DefaultMemoizerConfig())

	m.Put("k", "first")
	m.Put("k", "second")

	got, ok := m.Get("k")
	if !ok {
		t.Fatal("expected hit after overwrite")
	}
	if got != "second" {
		t.Fatalf("expected 'second', got %v", got)
	}
}
