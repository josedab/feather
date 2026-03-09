package offlinestore

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// PITJoinConfig configures point-in-time join behavior.
type PITJoinConfig struct {
	MaxLookback time.Duration `json:"max_lookback" yaml:"max_lookback"`
	MaxEntities int           `json:"max_entities" yaml:"max_entities"`
	TTL         time.Duration `json:"ttl" yaml:"ttl"`
}

// DefaultPITJoinConfig returns defaults.
func DefaultPITJoinConfig() PITJoinConfig {
	return PITJoinConfig{
		MaxLookback: 30 * 24 * time.Hour,
		MaxEntities: 1000000,
		TTL:         0,
	}
}

// EntityTimestamp pairs an entity with a point-in-time.
type EntityTimestamp struct {
	EntityID  string    `json:"entity_id"`
	Timestamp time.Time `json:"timestamp"`
}

// TrainingRow is one row of a training dataset.
type TrainingRow struct {
	EntityID  string                 `json:"entity_id"`
	EventTime time.Time              `json:"event_time"`
	Features  map[string]interface{} `json:"features"`
}

// TrainingDataset is the result of a point-in-time join.
type TrainingDataset struct {
	Rows      []TrainingRow `json:"rows"`
	Schema    []string      `json:"schema"`
	RowCount  int           `json:"row_count"`
	JoinStats PITJoinStats  `json:"join_stats"`
}

// PITJoinStats tracks PIT join performance.
type PITJoinStats struct {
	EntitiesProcessed int           `json:"entities_processed"`
	FeaturesJoined    int           `json:"features_joined"`
	RowsMatched       int           `json:"rows_matched"`
	RowsUnmatched     int           `json:"rows_unmatched"`
	Duration          time.Duration `json:"duration_ns"`
}

// PITJoinEngine performs point-in-time joins for training dataset generation.
type PITJoinEngine struct {
	config PITJoinConfig
	store  *Store
}

// NewPITJoinEngine creates a new point-in-time join engine.
func NewPITJoinEngine(config PITJoinConfig, store *Store) *PITJoinEngine {
	return &PITJoinEngine{config: config, store: store}
}

// Join performs a point-in-time join: for each (entity, timestamp) pair,
// find the latest feature values at or before that timestamp.
func (e *PITJoinEngine) Join(ctx context.Context, entityTimestamps []EntityTimestamp, featureNames []string, datasetName string) (*TrainingDataset, error) {
	if len(entityTimestamps) == 0 {
		return nil, fmt.Errorf("entity_timestamps must not be empty")
	}
	if len(featureNames) == 0 {
		return nil, fmt.Errorf("feature_names must not be empty")
	}
	if len(entityTimestamps) > e.config.MaxEntities {
		return nil, fmt.Errorf("too many entities (%d), max %d", len(entityTimestamps), e.config.MaxEntities)
	}

	start := time.Now()

	// Get all rows from the dataset
	allRows, err := e.store.GetRows(datasetName, int(e.store.config.MaxRowsPerDataset), 0)
	if err != nil {
		return nil, fmt.Errorf("reading dataset %s: %w", datasetName, err)
	}

	// Index rows by entity
	rowsByEntity := make(map[string][]FeatureRow)
	for _, row := range allRows {
		rowsByEntity[row.EntityID] = append(rowsByEntity[row.EntityID], row)
	}
	// Sort each entity's rows by timestamp
	for entityID := range rowsByEntity {
		sort.Slice(rowsByEntity[entityID], func(i, j int) bool {
			return rowsByEntity[entityID][i].Timestamp.Before(rowsByEntity[entityID][j].Timestamp)
		})
	}

	featureSet := make(map[string]bool, len(featureNames))
	for _, f := range featureNames {
		featureSet[f] = true
	}

	var result []TrainingRow
	matched, unmatched := 0, 0

	for _, et := range entityTimestamps {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		rows, ok := rowsByEntity[et.EntityID]
		if !ok {
			unmatched++
			continue
		}

		// Binary search for latest row <= timestamp
		idx := sort.Search(len(rows), func(i int) bool {
			return rows[i].Timestamp.After(et.Timestamp)
		}) - 1

		if idx < 0 {
			unmatched++
			continue
		}

		row := rows[idx]

		// Check lookback
		if e.config.MaxLookback > 0 && et.Timestamp.Sub(row.Timestamp) > e.config.MaxLookback {
			unmatched++
			continue
		}

		// Check TTL
		if e.config.TTL > 0 && et.Timestamp.Sub(row.Timestamp) > e.config.TTL {
			unmatched++
			continue
		}

		// Project only requested features
		features := make(map[string]interface{})
		for k, v := range row.Features {
			if featureSet[k] {
				features[k] = v
			}
		}

		result = append(result, TrainingRow{
			EntityID:  et.EntityID,
			EventTime: et.Timestamp,
			Features:  features,
		})
		matched++
	}

	return &TrainingDataset{
		Rows:     result,
		Schema:   featureNames,
		RowCount: len(result),
		JoinStats: PITJoinStats{
			EntitiesProcessed: len(entityTimestamps),
			FeaturesJoined:    len(featureNames),
			RowsMatched:       matched,
			RowsUnmatched:     unmatched,
			Duration:          time.Since(start),
		},
	}, nil
}

// GenerateTrainingSet is a convenience method that creates a full training
// dataset from a dataset by joining all entities at their latest timestamps.
func (e *PITJoinEngine) GenerateTrainingSet(ctx context.Context, datasetName string, featureNames []string) (*TrainingDataset, error) {
	allRows, err := e.store.GetRows(datasetName, int(e.store.config.MaxRowsPerDataset), 0)
	if err != nil {
		return nil, fmt.Errorf("reading dataset %s: %w", datasetName, err)
	}

	// Find latest timestamp per entity
	latestTS := make(map[string]time.Time)
	for _, row := range allRows {
		if existing, ok := latestTS[row.EntityID]; !ok || row.Timestamp.After(existing) {
			latestTS[row.EntityID] = row.Timestamp
		}
	}

	entityTimestamps := make([]EntityTimestamp, 0, len(latestTS))
	for entityID, ts := range latestTS {
		entityTimestamps = append(entityTimestamps, EntityTimestamp{
			EntityID:  entityID,
			Timestamp: ts,
		})
	}

	return e.Join(ctx, entityTimestamps, featureNames, datasetName)
}
