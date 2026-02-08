// Package marketplace provides a feature marketplace for cross-team
// feature publishing, discovery, subscription, and SLA management.
package marketplace

import (
	"fmt"
	"sync"
	"time"
)

// FeatureStatus represents the lifecycle state of a published feature.
type FeatureStatus string

const (
	// FeatureStatusDraft is a feature not yet published.
	FeatureStatusDraft FeatureStatus = "draft"
	// FeatureStatusPublished is an actively available feature.
	FeatureStatusPublished FeatureStatus = "published"
	// FeatureStatusDeprecated is a feature scheduled for removal.
	FeatureStatusDeprecated FeatureStatus = "deprecated"
	// FeatureStatusArchived is a feature that has been retired.
	FeatureStatusArchived FeatureStatus = "archived"
)

// QualityTier indicates the quality level of a published feature.
type QualityTier string

const (
	// QualityTierBronze is the lowest quality tier.
	QualityTierBronze QualityTier = "bronze"
	// QualityTierSilver is a mid-level quality tier.
	QualityTierSilver QualityTier = "silver"
	// QualityTierGold is the highest quality tier.
	QualityTierGold QualityTier = "gold"
)

// PublishedFeature represents a feature listed in the marketplace.
type PublishedFeature struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Owner        string            `json:"owner"`
	Team         string            `json:"team"`
	DataType     string            `json:"data_type"`
	EntityType   string            `json:"entity_type"`
	Tags         []string          `json:"tags"`
	Status       FeatureStatus     `json:"status"`
	Quality      QualityTier       `json:"quality"`
	SLA          *FeatureSLA       `json:"sla,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Version      string            `json:"version"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	PublishedAt  time.Time         `json:"published_at,omitempty"`
	DeprecatedAt time.Time         `json:"deprecated_at,omitempty"`
	Downloads    int64             `json:"downloads"`
}

// FeatureSLA defines the service level agreement for a published feature.
type FeatureSLA struct {
	MaxLatencyMs    int     `json:"max_latency_ms"`
	MinFreshnessSec int     `json:"min_freshness_sec"`
	AvailabilityPct float64 `json:"availability_pct"`
	MaxErrorRate    float64 `json:"max_error_rate"`
}

// Subscription represents a team's subscription to a published feature.
type Subscription struct {
	ID           string    `json:"id"`
	FeatureID    string    `json:"feature_id"`
	SubscriberID string    `json:"subscriber_id"`
	Team         string    `json:"team"`
	CreatedAt    time.Time `json:"created_at"`
	Active       bool      `json:"active"`
}

// SearchFilter defines criteria for marketplace searches.
type SearchFilter struct {
	Query      string        `json:"query"`
	Tags       []string      `json:"tags"`
	Owner      string        `json:"owner"`
	Team       string        `json:"team"`
	Status     FeatureStatus `json:"status"`
	Quality    QualityTier   `json:"quality"`
	EntityType string        `json:"entity_type"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
}

// Catalog manages the feature marketplace.
type Catalog struct {
	features      map[string]*PublishedFeature
	subscriptions map[string][]*Subscription // featureID -> subscriptions
	mu            sync.RWMutex
}

// NewCatalog creates a new marketplace catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		features:      make(map[string]*PublishedFeature),
		subscriptions: make(map[string][]*Subscription),
	}
}

// Publish adds or updates a feature in the marketplace.
func (c *Catalog) Publish(feat *PublishedFeature) error {
	if feat.ID == "" {
		return fmt.Errorf("feature ID is required")
	}
	if feat.Name == "" {
		return fmt.Errorf("feature name is required")
	}
	if feat.Owner == "" {
		return fmt.Errorf("feature owner is required")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if existing, ok := c.features[feat.ID]; ok {
		feat.CreatedAt = existing.CreatedAt
		feat.Downloads = existing.Downloads
	} else {
		feat.CreatedAt = now
	}
	feat.UpdatedAt = now
	if feat.Status == FeatureStatusPublished && feat.PublishedAt.IsZero() {
		feat.PublishedAt = now
	}
	c.features[feat.ID] = feat
	return nil
}

// Get retrieves a feature by ID.
func (c *Catalog) Get(id string) (*PublishedFeature, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	feat, ok := c.features[id]
	if !ok {
		return nil, fmt.Errorf("feature %q not found", id)
	}
	return feat, nil
}

// Search finds features matching the given filter.
func (c *Catalog) Search(filter SearchFilter) []*PublishedFeature {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []*PublishedFeature
	for _, feat := range c.features {
		if !c.matchesFilter(feat, filter) {
			continue
		}
		results = append(results, feat)
	}

	if filter.Limit > 0 && len(results) > filter.Limit {
		start := filter.Offset
		if start >= len(results) {
			return nil
		}
		end := start + filter.Limit
		if end > len(results) {
			end = len(results)
		}
		results = results[start:end]
	}
	return results
}

// List returns all published features.
func (c *Catalog) List() []*PublishedFeature {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*PublishedFeature, 0, len(c.features))
	for _, f := range c.features {
		result = append(result, f)
	}
	return result
}

// Deprecate marks a feature as deprecated.
func (c *Catalog) Deprecate(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	feat, ok := c.features[id]
	if !ok {
		return fmt.Errorf("feature %q not found", id)
	}
	feat.Status = FeatureStatusDeprecated
	feat.DeprecatedAt = time.Now()
	feat.UpdatedAt = time.Now()
	return nil
}

// Subscribe creates a subscription to a feature.
func (c *Catalog) Subscribe(featureID, subscriberID, team string) (*Subscription, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	feat, ok := c.features[featureID]
	if !ok {
		return nil, fmt.Errorf("feature %q not found", featureID)
	}
	if feat.Status != FeatureStatusPublished {
		return nil, fmt.Errorf("feature %q is not published (status: %s)", featureID, feat.Status)
	}

	// Check for existing subscription
	for _, sub := range c.subscriptions[featureID] {
		if sub.SubscriberID == subscriberID && sub.Active {
			return sub, nil
		}
	}

	sub := &Subscription{
		ID:           fmt.Sprintf("sub-%s-%s", featureID, subscriberID),
		FeatureID:    featureID,
		SubscriberID: subscriberID,
		Team:         team,
		CreatedAt:    time.Now(),
		Active:       true,
	}
	c.subscriptions[featureID] = append(c.subscriptions[featureID], sub)
	feat.Downloads++
	return sub, nil
}

// Unsubscribe deactivates a subscription.
func (c *Catalog) Unsubscribe(featureID, subscriberID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	subs, ok := c.subscriptions[featureID]
	if !ok {
		return fmt.Errorf("no subscriptions for feature %q", featureID)
	}

	for _, sub := range subs {
		if sub.SubscriberID == subscriberID {
			sub.Active = false
			return nil
		}
	}
	return fmt.Errorf("subscription not found for subscriber %q", subscriberID)
}

// GetSubscribers returns all active subscribers for a feature.
func (c *Catalog) GetSubscribers(featureID string) []*Subscription {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var active []*Subscription
	for _, sub := range c.subscriptions[featureID] {
		if sub.Active {
			active = append(active, sub)
		}
	}
	return active
}

// GetSubscriptions returns all subscriptions for a subscriber.
func (c *Catalog) GetSubscriptions(subscriberID string) []*Subscription {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Subscription
	for _, subs := range c.subscriptions {
		for _, sub := range subs {
			if sub.SubscriberID == subscriberID && sub.Active {
				result = append(result, sub)
			}
		}
	}
	return result
}

// Stats returns marketplace statistics.
func (c *Catalog) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	published := 0
	deprecated := 0
	totalSubs := 0
	for _, f := range c.features {
		switch f.Status {
		case FeatureStatusPublished:
			published++
		case FeatureStatusDeprecated:
			deprecated++
		}
	}
	for _, subs := range c.subscriptions {
		for _, sub := range subs {
			if sub.Active {
				totalSubs++
			}
		}
	}

	return map[string]interface{}{
		"total_features":     len(c.features),
		"published":          published,
		"deprecated":         deprecated,
		"total_subscriptions": totalSubs,
	}
}

func (c *Catalog) matchesFilter(feat *PublishedFeature, filter SearchFilter) bool {
	if filter.Status != "" && feat.Status != filter.Status {
		return false
	}
	if filter.Quality != "" && feat.Quality != filter.Quality {
		return false
	}
	if filter.Owner != "" && feat.Owner != filter.Owner {
		return false
	}
	if filter.Team != "" && feat.Team != filter.Team {
		return false
	}
	if filter.EntityType != "" && feat.EntityType != filter.EntityType {
		return false
	}
	if filter.Query != "" {
		if !containsIgnoreCase(feat.Name, filter.Query) &&
			!containsIgnoreCase(feat.Description, filter.Query) {
			return false
		}
	}
	if len(filter.Tags) > 0 {
		if !hasAnyTag(feat.Tags, filter.Tags) {
			return false
		}
	}
	return true
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 ||
			findIgnoreCase(s, substr))
}

func findIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			fc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 'a' - 'A'
			}
			if fc >= 'A' && fc <= 'Z' {
				fc += 'a' - 'A'
			}
			if sc != fc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func hasAnyTag(tags, filter []string) bool {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	for _, f := range filter {
		if tagSet[f] {
			return true
		}
	}
	return false
}
