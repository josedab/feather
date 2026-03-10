package feastcompat

import (
	"context"
	"fmt"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// FeatureStoreBackend is the interface needed from the core store.
type FeatureStoreBackend interface {
	Get(ctx context.Context, entityKey string, features []string) (map[string]*domain.FeatureValue, error)
	GetAsOf(ctx context.Context, entityKey string, features []string, asOf time.Time) (map[string]*domain.FeatureValue, error)
	Put(ctx context.Context, entityKey string, features map[string]*domain.FeatureValue) error
}

// StoreLookupAdapter bridges the Feast adapter to the real Feather storage tiers.
type StoreLookupAdapter struct {
	store FeatureStoreBackend
}

// NewStoreLookupAdapter creates a lookup adapter backed by real storage.
func NewStoreLookupAdapter(store FeatureStoreBackend) *StoreLookupAdapter {
	return &StoreLookupAdapter{store: store}
}

// LookupFunc returns a FeatureLookupFunc that reads from real hot/warm tiers.
func (s *StoreLookupAdapter) LookupFunc() FeatureLookupFunc {
	return func(entityID string, features []string) (map[string]interface{}, error) {
		ctx := context.Background()
		vals, err := s.store.Get(ctx, entityID, features)
		if err != nil {
			return nil, fmt.Errorf("looking up features for %s: %w", entityID, err)
		}
		result := make(map[string]interface{}, len(vals))
		for k, v := range vals {
			if v != nil {
				result[k] = v.Value
			}
		}
		return result, nil
	}
}

// PushToStore writes feature values from a Feast push request into the real store.
func (s *StoreLookupAdapter) PushToStore(pushSourceName string, rows []map[string]interface{}) (int, error) {
	ctx := context.Background()
	ingested := 0
	for _, row := range rows {
		entityID := ""
		features := make(map[string]*domain.FeatureValue)
		now := time.Now().UnixNano()
		for k, v := range row {
			if k == "entity_id" || k == "id" || k == "entity_key" {
				entityID = fmt.Sprintf("%v", v)
				continue
			}
			features[k] = &domain.FeatureValue{
				Value:     v,
				Timestamp: now,
				Version:   1,
			}
		}
		if entityID == "" {
			entityID = fmt.Sprintf("%s_%d", pushSourceName, ingested)
		}
		if err := s.store.Put(ctx, entityID, features); err != nil {
			return ingested, fmt.Errorf("pushing features for entity %s: %w", entityID, err)
		}
		ingested++
	}
	return ingested, nil
}

// MaterializeFromStore reads features from the warm tier at a point-in-time
// and promotes them to the hot tier for online serving.
func (s *StoreLookupAdapter) MaterializeFromStore(entityKeys []string, features []string, endDate time.Time) (int64, error) {
	ctx := context.Background()
	var rowsWritten int64
	for _, entityKey := range entityKeys {
		vals, err := s.store.GetAsOf(ctx, entityKey, features, endDate)
		if err != nil {
			continue
		}
		if len(vals) > 0 {
			if err := s.store.Put(ctx, entityKey, vals); err != nil {
				continue
			}
			rowsWritten += int64(len(vals))
		}
	}
	return rowsWritten, nil
}
