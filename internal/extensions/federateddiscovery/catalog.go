package federateddiscovery

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// CatalogEntry represents a published feature in the catalog.
type CatalogEntry struct {
	ID           string
	Name         string
	Description  string
	Owner        string
	FeatureGroup string
	Schema       map[string]string
	Tags         []string
	Quality      float64
	Freshness    string
	PublishedAt  time.Time
	Source       string
}

// Subscription tracks a subscriber to a catalog entry.
type Subscription struct {
	ID             string
	CatalogEntryID string
	Subscriber     string
	CreatedAt      time.Time
}

// SearchQuery specifies search criteria for catalog entries.
type SearchQuery struct {
	Text       string
	Tags       []string
	Owner      string
	MinQuality float64
}

// CatalogConfig configures the catalog.
type CatalogConfig struct {
	MaxEntries       int
	MaxSubscriptions int
}

// DefaultCatalogConfig returns sensible defaults.
func DefaultCatalogConfig() CatalogConfig {
	return CatalogConfig{
		MaxEntries:       100000,
		MaxSubscriptions: 500000,
	}
}

// CatalogStats holds aggregate statistics.
type CatalogStats struct {
	TotalEntries       int
	TotalSubscriptions int
	UniqueOwners       int
	UniqueSubscribers  int
	AvgQuality         float64
}

// Catalog manages feature catalog entries and subscriptions.
type Catalog struct {
	mu            sync.RWMutex
	config        CatalogConfig
	entries       map[string]*CatalogEntry
	subscriptions map[string][]Subscription
}

// NewCatalog creates a new catalog.
func NewCatalog(config CatalogConfig) *Catalog {
	if config.MaxEntries == 0 {
		config = DefaultCatalogConfig()
	}
	return &Catalog{
		config:        config,
		entries:       make(map[string]*CatalogEntry),
		subscriptions: make(map[string][]Subscription),
	}
}

// Publish adds or updates a catalog entry.
func (c *Catalog) Publish(entry CatalogEntry) error {
	if entry.ID == "" {
		return fmt.Errorf("entry ID is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[entry.ID]; !exists {
		if len(c.entries) >= c.config.MaxEntries {
			return fmt.Errorf("max entries (%d) reached", c.config.MaxEntries)
		}
	}

	if entry.PublishedAt.IsZero() {
		entry.PublishedAt = time.Now()
	}
	copy := entry
	c.entries[entry.ID] = &copy
	return nil
}

// Unpublish removes a catalog entry.
func (c *Catalog) Unpublish(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[id]; !exists {
		return ErrCatalogNotFound
	}
	delete(c.entries, id)
	delete(c.subscriptions, id)
	return nil
}

// Get returns a catalog entry by ID.
func (c *Catalog) Get(id string) (*CatalogEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[id]
	if !exists {
		return nil, ErrCatalogNotFound
	}
	copy := *entry
	return &copy, nil
}

// Search finds catalog entries matching the query.
func (c *Catalog) Search(query SearchQuery) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []CatalogEntry
	for _, entry := range c.entries {
		if !matchesQuery(entry, query) {
			continue
		}
		results = append(results, *entry)
	}
	return results
}

// ListAll returns all catalog entries.
func (c *Catalog) ListAll() []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]CatalogEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		out = append(out, *entry)
	}
	return out
}

// Subscribe creates a subscription to a catalog entry.
func (c *Catalog) Subscribe(entryID, subscriber string) (*Subscription, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[entryID]; !exists {
		return nil, ErrCatalogNotFound
	}

	// Check for existing subscription
	for _, sub := range c.subscriptions[entryID] {
		if sub.Subscriber == subscriber {
			return nil, ErrSubscriptionExists
		}
	}

	totalSubs := 0
	for _, subs := range c.subscriptions {
		totalSubs += len(subs)
	}
	if totalSubs >= c.config.MaxSubscriptions {
		return nil, fmt.Errorf("max subscriptions (%d) reached", c.config.MaxSubscriptions)
	}

	sub := Subscription{
		ID:             fmt.Sprintf("%s-%s", entryID, subscriber),
		CatalogEntryID: entryID,
		Subscriber:     subscriber,
		CreatedAt:      time.Now(),
	}
	c.subscriptions[entryID] = append(c.subscriptions[entryID], sub)

	return &sub, nil
}

// Unsubscribe removes a subscription.
func (c *Catalog) Unsubscribe(entryID, subscriber string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	subs, exists := c.subscriptions[entryID]
	if !exists {
		return ErrCatalogNotFound
	}

	for i, sub := range subs {
		if sub.Subscriber == subscriber {
			c.subscriptions[entryID] = append(subs[:i], subs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("subscription not found")
}

// GetSubscribers returns all subscribers for a catalog entry.
func (c *Catalog) GetSubscribers(entryID string) []Subscription {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := c.subscriptions[entryID]
	out := make([]Subscription, len(subs))
	copy(out, subs)
	return out
}

// GetSubscriptions returns all entries a subscriber follows.
func (c *Catalog) GetSubscriptions(subscriber string) []CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []CatalogEntry
	for entryID, subs := range c.subscriptions {
		for _, sub := range subs {
			if sub.Subscriber == subscriber {
				if entry, exists := c.entries[entryID]; exists {
					results = append(results, *entry)
				}
				break
			}
		}
	}
	return results
}

// Stats returns aggregate statistics.
func (c *Catalog) Stats() CatalogStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	owners := make(map[string]bool)
	subscribers := make(map[string]bool)
	var totalQuality float64

	for _, entry := range c.entries {
		owners[entry.Owner] = true
		totalQuality += entry.Quality
	}

	totalSubs := 0
	for _, subs := range c.subscriptions {
		totalSubs += len(subs)
		for _, sub := range subs {
			subscribers[sub.Subscriber] = true
		}
	}

	var avgQuality float64
	if len(c.entries) > 0 {
		avgQuality = totalQuality / float64(len(c.entries))
	}

	return CatalogStats{
		TotalEntries:       len(c.entries),
		TotalSubscriptions: totalSubs,
		UniqueOwners:       len(owners),
		UniqueSubscribers:  len(subscribers),
		AvgQuality:         avgQuality,
	}
}

// matchesQuery checks if an entry matches the search query.
func matchesQuery(entry *CatalogEntry, query SearchQuery) bool {
	// Text search on name and description
	if query.Text != "" {
		text := strings.ToLower(query.Text)
		if !strings.Contains(strings.ToLower(entry.Name), text) &&
			!strings.Contains(strings.ToLower(entry.Description), text) {
			return false
		}
	}

	// Tag filter
	if len(query.Tags) > 0 {
		matched := false
		for _, qt := range query.Tags {
			for _, et := range entry.Tags {
				if strings.EqualFold(qt, et) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Owner filter
	if query.Owner != "" && !strings.EqualFold(entry.Owner, query.Owner) {
		return false
	}

	// Quality filter
	if query.MinQuality > 0 && entry.Quality < query.MinQuality {
		return false
	}

	return true
}
