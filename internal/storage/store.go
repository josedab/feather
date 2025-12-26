package storage

import (
	"context"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// Store provides feature storage with tiered architecture.
type Store struct {
	hot     *HotTier
	warm    *WarmTier
	schema  SchemaRegistry
	metrics *StoreMetrics

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// SchemaRegistry provides access to feature schemas.
type SchemaRegistry interface {
	GetGroup(name string) (*domain.FeatureGroup, error)
	GetFeatureSpec(featureName string) (*domain.FeatureSpec, error)
	ListGroups() []*domain.FeatureGroup
}

// StoreMetrics tracks store performance.
type StoreMetrics struct {
	HotHits    int64
	HotMisses  int64
	WarmHits   int64
	WarmMisses int64
	Writes     int64
}

// StoreOptions configures the store.
type StoreOptions struct {
	HotMaxSize       int64
	WarmPath         string
	WarmSyncInterval time.Duration
	WarmInMemory     bool // For testing
	TTLCheckInterval time.Duration
}

// NewStore creates a new feature store.
func NewStore(opts StoreOptions, schema SchemaRegistry) (*Store, error) {
	hot := NewHotTier(opts.HotMaxSize)

	warm, err := NewWarmTier(WarmTierOptions{
		Path:         opts.WarmPath,
		SyncInterval: opts.WarmSyncInterval,
		InMemory:     opts.WarmInMemory,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Store{
		hot:     hot,
		warm:    warm,
		schema:  schema,
		metrics: &StoreMetrics{},
		cancel:  cancel,
	}

	// Start background tasks
	ttlInterval := opts.TTLCheckInterval
	if ttlInterval == 0 {
		ttlInterval = time.Minute
	}
	s.wg.Add(1)
	go s.ttlLoop(ctx, ttlInterval)

	return s, nil
}

// Get retrieves features for an entity.
func (s *Store) Get(entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	// Try hot tier first
	result, err := s.hot.Get(entityKey, features)
	if err != nil && err != domain.ErrEntityNotFound {
		return nil, err
	}

	// Get missing slice from pool to reduce allocations
	missingPtr := featureNameSlicePool.Get().(*[]string)
	missing := (*missingPtr)[:0]
	defer func() {
		*missingPtr = missing[:0]
		featureNameSlicePool.Put(missingPtr)
	}()

	if result == nil {
		result = make(map[string]*domain.FeatureValue)
		missing = append(missing, features...)
	} else {
		for _, f := range features {
			if _, ok := result[f]; !ok {
				missing = append(missing, f)
			}
		}
	}

	// Check warm tier for missing features
	if len(missing) > 0 {
		warmResult, err := s.warm.Get(entityKey, missing)
		if err != nil {
			return nil, err
		}
		for k, v := range warmResult {
			result[k] = v
		}
	}

	return result, nil
}

// GetAsOf retrieves features as of a specific timestamp.
func (s *Store) GetAsOf(entityKey string, features []string, asOf time.Time) (map[string]*domain.FeatureValue, error) {
	return s.warm.GetAsOf(entityKey, features, asOf)
}

// Put stores features for an entity in both tiers.
func (s *Store) Put(entityKey string, features map[string]*domain.FeatureValue) error {
	// Write to hot tier first
	if err := s.hot.Put(entityKey, features); err != nil {
		return err
	}

	// Write to warm tier asynchronously with tracking
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		_ = s.warm.Put(entityKey, features)
	}()

	return nil
}

// Delete removes an entity from both tiers.
func (s *Store) Delete(entityKey string) error {
	if err := s.hot.Delete(entityKey); err != nil {
		return err
	}
	// Note: We don't delete from warm tier to preserve history
	return nil
}

// Hot returns the hot tier for direct access.
func (s *Store) Hot() *HotTier {
	return s.hot
}

// Warm returns the warm tier for direct access.
func (s *Store) Warm() *WarmTier {
	return s.warm
}

// Metrics returns current metrics.
func (s *Store) Metrics() StoreMetrics {
	hotMetrics := s.hot.Metrics()
	return StoreMetrics{
		HotHits:   hotMetrics.Hits,
		HotMisses: hotMetrics.Misses,
	}
}

// ttlLoop periodically checks for and removes expired features.
func (s *Store) ttlLoop(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.schema != nil {
				for _, group := range s.schema.ListGroups() {
					if group.TTL > 0 {
						s.hot.ExpireOlderThan(group.TTL)
					}
				}
			}
		}
	}
}

// Close shuts down the store.
func (s *Store) Close() error {
	s.cancel()
	s.wg.Wait()
	return s.warm.Close()
}
