package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"

	"github.com/feather-store/feather/internal/domain"
)

// Object pools for reducing allocations in warm tier operations.
var (
	// Pool for key byte slices
	keyBytesPool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 0, 128)
			return &b
		},
	}
)

// WarmTier provides disk-based storage using BadgerDB.
type WarmTier struct {
	db           *badger.DB
	syncInterval time.Duration
}

// WarmTierOptions configures the warm tier.
type WarmTierOptions struct {
	Path         string
	SyncInterval time.Duration
	InMemory     bool // For testing
}

// NewWarmTier creates a new warm tier.
func NewWarmTier(opts WarmTierOptions) (*WarmTier, error) {
	badgerOpts := badger.DefaultOptions(opts.Path)
	if opts.InMemory {
		badgerOpts = badger.DefaultOptions("").WithInMemory(true)
	}
	badgerOpts = badgerOpts.WithLogger(nil) // Disable default logging

	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("opening badger: %w", err)
	}

	syncInterval := opts.SyncInterval
	if syncInterval == 0 {
		syncInterval = time.Second
	}

	return &WarmTier{
		db:           db,
		syncInterval: syncInterval,
	}, nil
}

// featureKey generates the key for a feature.
// Note: Returns a new byte slice; caller should not retain after use in badger txn.
func featureKey(entityKey, featureName string) []byte {
	return []byte(fmt.Sprintf("f:%s:%s", entityKey, featureName))
}

// featureKeyPooled generates the key for a feature using a pooled buffer.
// Returns a buffer that must be returned to pool after use.
func featureKeyPooled(entityKey, featureName string) ([]byte, func()) {
	bufPtr, ok := keyBytesPool.Get().(*[]byte)
	if !ok {
		b := make([]byte, 0, 128)
		bufPtr = &b
	}
	buf := (*bufPtr)[:0]
	buf = append(buf, "f:"...)
	buf = append(buf, entityKey...)
	buf = append(buf, ":"...)
	buf = append(buf, featureName...)
	return buf, func() {
		*bufPtr = buf[:0]
		keyBytesPool.Put(bufPtr)
	}
}

// historyKey generates the key for historical feature data.
// Use inverted timestamp for time-ordered iteration
func historyKey(entityKey, featureName string, timestamp int64) []byte {
	return []byte(fmt.Sprintf("h:%s:%s:%020d", entityKey, featureName, timestamp))
}

// Get retrieves features for an entity.
func (w *WarmTier) Get(entityKey string, features []string) (map[string]*domain.FeatureValue, error) {
	result := make(map[string]*domain.FeatureValue, len(features))

	err := w.db.View(func(txn *badger.Txn) error {
		for _, feature := range features {
			key, release := featureKeyPooled(entityKey, feature)
			item, err := txn.Get(key)
			release() // Return key buffer to pool
			if errors.Is(err, badger.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return err
			}

			val, err := decodeFeatureValue(item)
			if err != nil {
				return err
			}
			result[feature] = val
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("getting features: %w", err)
	}

	return result, nil
}

// Put stores features for an entity.
func (w *WarmTier) Put(entityKey string, features map[string]*domain.FeatureValue) error {
	return w.db.Update(func(txn *badger.Txn) error {
		for name, val := range features {
			key := featureKey(entityKey, name)
			data, err := encodeFeatureValue(val)
			if err != nil {
				return err
			}

			if err := txn.Set(key, data); err != nil {
				return err
			}

			// Also store in history for point-in-time retrieval
			histKey := historyKey(entityKey, name, val.Timestamp)
			if err := txn.Set(histKey, data); err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete removes features for an entity.
func (w *WarmTier) Delete(entityKey string, features []string) error {
	return w.db.Update(func(txn *badger.Txn) error {
		for _, feature := range features {
			key := featureKey(entityKey, feature)
			if err := txn.Delete(key); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
				return err
			}
		}
		return nil
	})
}

// GetAsOf retrieves features as of a specific timestamp.
func (w *WarmTier) GetAsOf(entityKey string, features []string, asOf time.Time) (map[string]*domain.FeatureValue, error) {
	result := make(map[string]*domain.FeatureValue, len(features))
	asOfNano := asOf.UnixNano()

	err := w.db.View(func(txn *badger.Txn) error {
		for _, feature := range features {
			prefix := []byte(fmt.Sprintf("h:%s:%s:", entityKey, feature))
			seekKey := historyKey(entityKey, feature, asOfNano)

			opts := badger.DefaultIteratorOptions
			opts.Reverse = true
			opts.PrefetchSize = 1
			it := txn.NewIterator(opts)
			defer it.Close()

			it.Seek(seekKey)
			if it.ValidForPrefix(prefix) {
				item := it.Item()
				val, err := decodeFeatureValue(item)
				if err != nil {
					return err
				}
				result[feature] = val
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("getting features as of: %w", err)
	}

	return result, nil
}

// ExpireOlderThan removes historical data older than the given duration.
func (w *WarmTier) ExpireOlderThan(retention time.Duration) (int, error) {
	cutoff := time.Now().Add(-retention).UnixNano()
	deleted := 0

	err := w.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("h:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

			// Parse timestamp from key
			var timestamp int64
			_, err := fmt.Sscanf(string(key), "h:%*[^:]:%*[^:]:%d", &timestamp)
			if err != nil {
				continue
			}

			if timestamp < cutoff {
				if err := txn.Delete(key); err != nil {
					return err
				}
				deleted++
			}
		}
		return nil
	})

	return deleted, err
}

// Close closes the warm tier.
func (w *WarmTier) Close() error {
	return w.db.Close()
}

// RunGC runs garbage collection.
func (w *WarmTier) RunGC() error {
	return w.db.RunValueLogGC(0.5)
}

func encodeFeatureValue(val *domain.FeatureValue) ([]byte, error) {
	return json.Marshal(val)
}

func decodeFeatureValue(item *badger.Item) (*domain.FeatureValue, error) {
	var val domain.FeatureValue
	err := item.Value(func(data []byte) error {
		return json.Unmarshal(data, &val)
	})
	if err != nil {
		return nil, err
	}
	return &val, nil
}
