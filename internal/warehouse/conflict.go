package warehouse

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

// ConflictResolver handles data conflicts during synchronization.
type ConflictResolver struct {
	mu            sync.RWMutex
	strategy      ConflictResolution
	customHandler CustomConflictHandler
	logger        *slog.Logger

	// Stats
	conflictsResolved int64
	conflictsByType   map[string]int64
}

// CustomConflictHandler is a function that handles custom conflict resolution.
type CustomConflictHandler func(ctx context.Context, conflict *Conflict) (*Resolution, error)

// Conflict represents a data conflict between source and target.
type Conflict struct {
	// EntityID is the entity key that has a conflict.
	EntityID string `json:"entity_id"`

	// FeatureName is the feature that has a conflict.
	FeatureName string `json:"feature_name"`

	// SourceValue is the value from the source system.
	SourceValue *FeatureConflictValue `json:"source_value"`

	// TargetValue is the value from the target system.
	TargetValue *FeatureConflictValue `json:"target_value"`

	// ConflictType describes the nature of the conflict.
	ConflictType ConflictType `json:"conflict_type"`

	// DetectedAt is when the conflict was detected.
	DetectedAt time.Time `json:"detected_at"`
}

// FeatureConflictValue holds a feature value with metadata.
type FeatureConflictValue struct {
	Value     interface{} `json:"value"`
	Timestamp time.Time   `json:"timestamp"`
	Version   int64       `json:"version"`
	Checksum  string      `json:"checksum,omitempty"`
	Source    string      `json:"source"`
}

// ConflictType describes the type of conflict.
type ConflictType string

const (
	// ConflictTypeValueMismatch indicates different values at same timestamp.
	ConflictTypeValueMismatch ConflictType = "value_mismatch"

	// ConflictTypeVersionConflict indicates both sides updated the same version.
	ConflictTypeVersionConflict ConflictType = "version_conflict"

	// ConflictTypeConcurrentUpdate indicates updates happened at nearly the same time.
	ConflictTypeConcurrentUpdate ConflictType = "concurrent_update"

	// ConflictTypeDeleteUpdate indicates one side deleted while other updated.
	ConflictTypeDeleteUpdate ConflictType = "delete_update"

	// ConflictTypeSchemaMismatch indicates type/schema incompatibility.
	ConflictTypeSchemaMismatch ConflictType = "schema_mismatch"
)

// Resolution represents how a conflict was resolved.
type Resolution struct {
	// ResolvedValue is the final value to use.
	ResolvedValue interface{} `json:"resolved_value"`

	// Timestamp is the timestamp to use.
	Timestamp time.Time `json:"timestamp"`

	// Version is the version to use.
	Version int64 `json:"version"`

	// Strategy is the strategy used to resolve.
	Strategy ConflictResolution `json:"strategy"`

	// Winner indicates which side won ("source", "target", "merged", "custom").
	Winner string `json:"winner"`

	// Reason explains why this resolution was chosen.
	Reason string `json:"reason,omitempty"`
}

// NewConflictResolver creates a new conflict resolver.
func NewConflictResolver(strategy ConflictResolution, logger *slog.Logger) *ConflictResolver {
	if logger == nil {
		logger = slog.Default()
	}

	return &ConflictResolver{
		strategy:        strategy,
		logger:          logger,
		conflictsByType: make(map[string]int64),
	}
}

// SetStrategy changes the conflict resolution strategy.
func (r *ConflictResolver) SetStrategy(strategy ConflictResolution) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = strategy
}

// GetStrategy returns the current strategy.
func (r *ConflictResolver) GetStrategy() ConflictResolution {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.strategy
}

// SetCustomHandler sets a custom conflict handler.
func (r *ConflictResolver) SetCustomHandler(handler CustomConflictHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customHandler = handler
}

// Resolve resolves a conflict using the configured strategy.
func (r *ConflictResolver) Resolve(ctx context.Context, conflict *Conflict) (*Resolution, error) {
	r.mu.RLock()
	strategy := r.strategy
	customHandler := r.customHandler
	r.mu.RUnlock()

	var resolution *Resolution
	var err error

	switch strategy {
	case ConflictResolutionLatest:
		resolution = r.resolveLatest(conflict)
	case ConflictResolutionSource:
		resolution = r.resolveSource(conflict)
	case ConflictResolutionTarget:
		resolution = r.resolveTarget(conflict)
	case ConflictResolutionHigherVer:
		resolution = r.resolveHigherVersion(conflict)
	case ConflictResolutionMerge:
		resolution, err = r.resolveMerge(conflict)
	case ConflictResolutionCustom:
		if customHandler != nil {
			resolution, err = customHandler(ctx, conflict)
		} else {
			err = fmt.Errorf("no custom handler registered")
		}
	default:
		err = fmt.Errorf("unknown conflict resolution strategy: %s", strategy)
	}

	if err != nil {
		r.logger.Error("conflict resolution failed",
			"entity", conflict.EntityID,
			"feature", conflict.FeatureName,
			"type", conflict.ConflictType,
			"error", err,
		)
		return nil, err
	}

	// Update stats
	r.mu.Lock()
	r.conflictsResolved++
	r.conflictsByType[string(conflict.ConflictType)]++
	r.mu.Unlock()

	r.logger.Debug("conflict resolved",
		"entity", conflict.EntityID,
		"feature", conflict.FeatureName,
		"type", conflict.ConflictType,
		"winner", resolution.Winner,
		"strategy", resolution.Strategy,
	)

	return resolution, nil
}

// resolveLatest picks the value with the latest timestamp.
func (r *ConflictResolver) resolveLatest(conflict *Conflict) *Resolution {
	if conflict.SourceValue.Timestamp.After(conflict.TargetValue.Timestamp) {
		return &Resolution{
			ResolvedValue: conflict.SourceValue.Value,
			Timestamp:     conflict.SourceValue.Timestamp,
			Version:       conflict.SourceValue.Version,
			Strategy:      ConflictResolutionLatest,
			Winner:        "source",
			Reason:        "source has later timestamp",
		}
	}

	return &Resolution{
		ResolvedValue: conflict.TargetValue.Value,
		Timestamp:     conflict.TargetValue.Timestamp,
		Version:       conflict.TargetValue.Version,
		Strategy:      ConflictResolutionLatest,
		Winner:        "target",
		Reason:        "target has later timestamp",
	}
}

// resolveSource always picks the source value.
func (r *ConflictResolver) resolveSource(conflict *Conflict) *Resolution {
	return &Resolution{
		ResolvedValue: conflict.SourceValue.Value,
		Timestamp:     conflict.SourceValue.Timestamp,
		Version:       conflict.SourceValue.Version,
		Strategy:      ConflictResolutionSource,
		Winner:        "source",
		Reason:        "source wins by policy",
	}
}

// resolveTarget always picks the target value.
func (r *ConflictResolver) resolveTarget(conflict *Conflict) *Resolution {
	return &Resolution{
		ResolvedValue: conflict.TargetValue.Value,
		Timestamp:     conflict.TargetValue.Timestamp,
		Version:       conflict.TargetValue.Version,
		Strategy:      ConflictResolutionTarget,
		Winner:        "target",
		Reason:        "target wins by policy",
	}
}

// resolveHigherVersion picks the value with the higher version.
func (r *ConflictResolver) resolveHigherVersion(conflict *Conflict) *Resolution {
	if conflict.SourceValue.Version > conflict.TargetValue.Version {
		return &Resolution{
			ResolvedValue: conflict.SourceValue.Value,
			Timestamp:     conflict.SourceValue.Timestamp,
			Version:       conflict.SourceValue.Version,
			Strategy:      ConflictResolutionHigherVer,
			Winner:        "source",
			Reason:        fmt.Sprintf("source version %d > target version %d", conflict.SourceValue.Version, conflict.TargetValue.Version),
		}
	}

	if conflict.TargetValue.Version > conflict.SourceValue.Version {
		return &Resolution{
			ResolvedValue: conflict.TargetValue.Value,
			Timestamp:     conflict.TargetValue.Timestamp,
			Version:       conflict.TargetValue.Version,
			Strategy:      ConflictResolutionHigherVer,
			Winner:        "target",
			Reason:        fmt.Sprintf("target version %d > source version %d", conflict.TargetValue.Version, conflict.SourceValue.Version),
		}
	}

	// Equal versions - fall back to latest timestamp
	return r.resolveLatest(conflict)
}

// resolveMerge attempts to merge values where possible.
func (r *ConflictResolver) resolveMerge(conflict *Conflict) (*Resolution, error) {
	sourceVal := conflict.SourceValue.Value
	targetVal := conflict.TargetValue.Value

	// Try to merge based on type
	merged, canMerge := r.tryMerge(sourceVal, targetVal)
	if canMerge {
		// Use the later timestamp and incremented version
		ts := conflict.SourceValue.Timestamp
		if conflict.TargetValue.Timestamp.After(ts) {
			ts = conflict.TargetValue.Timestamp
		}

		version := conflict.SourceValue.Version
		if conflict.TargetValue.Version > version {
			version = conflict.TargetValue.Version
		}
		version++ // Increment for the merge

		return &Resolution{
			ResolvedValue: merged,
			Timestamp:     ts,
			Version:       version,
			Strategy:      ConflictResolutionMerge,
			Winner:        "merged",
			Reason:        "values merged successfully",
		}, nil
	}

	// Cannot merge - fall back to latest
	r.logger.Debug("cannot merge values, falling back to latest",
		"entity", conflict.EntityID,
		"feature", conflict.FeatureName,
	)
	return r.resolveLatest(conflict), nil
}

// tryMerge attempts to merge two values.
func (r *ConflictResolver) tryMerge(source, target interface{}) (interface{}, bool) {
	// Merge maps
	sourceMap, sourceIsMap := source.(map[string]interface{})
	targetMap, targetIsMap := target.(map[string]interface{})
	if sourceIsMap && targetIsMap {
		merged := make(map[string]interface{})
		// Start with target
		for k, v := range targetMap {
			merged[k] = v
		}
		// Overlay source (source wins on key conflicts)
		for k, v := range sourceMap {
			merged[k] = v
		}
		return merged, true
	}

	// Merge slices by concatenation
	sourceSlice, sourceIsSlice := source.([]interface{})
	targetSlice, targetIsSlice := target.([]interface{})
	if sourceIsSlice && targetIsSlice {
		merged := append(targetSlice, sourceSlice...)
		return merged, true
	}

	// Numeric values - could use max, sum, avg, etc.
	// For now, we don't merge numeric values
	return nil, false
}

// DetectConflict detects if there's a conflict between two feature values.
func (r *ConflictResolver) DetectConflict(
	entityID, featureName string,
	sourceVal, targetVal *FeatureConflictValue,
	tolerance time.Duration,
) *Conflict {
	// No conflict if values are equal
	if r.valuesEqual(sourceVal.Value, targetVal.Value) {
		return nil
	}

	conflict := &Conflict{
		EntityID:    entityID,
		FeatureName: featureName,
		SourceValue: sourceVal,
		TargetValue: targetVal,
		DetectedAt:  time.Now(),
	}

	// Determine conflict type
	timeDiff := sourceVal.Timestamp.Sub(targetVal.Timestamp)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	// Check for concurrent updates (within tolerance)
	if timeDiff <= tolerance {
		conflict.ConflictType = ConflictTypeConcurrentUpdate
		return conflict
	}

	// Check for version conflict
	if sourceVal.Version == targetVal.Version && sourceVal.Version > 0 {
		conflict.ConflictType = ConflictTypeVersionConflict
		return conflict
	}

	// Default to value mismatch
	conflict.ConflictType = ConflictTypeValueMismatch
	return conflict
}

// valuesEqual checks if two values are equal.
func (r *ConflictResolver) valuesEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Direct comparison for basic types
	switch av := a.(type) {
	case int64:
		if bv, ok := b.(int64); ok {
			return av == bv
		}
	case float64:
		if bv, ok := b.(float64); ok {
			return av == bv
		}
	case string:
		if bv, ok := b.(string); ok {
			return av == bv
		}
	case bool:
		if bv, ok := b.(bool); ok {
			return av == bv
		}
	}

	// For complex types, use string comparison as fallback
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// Stats returns conflict resolution statistics.
func (r *ConflictResolver) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	byType := make(map[string]int64)
	for k, v := range r.conflictsByType {
		byType[k] = v
	}

	return map[string]interface{}{
		"conflicts_resolved": r.conflictsResolved,
		"conflicts_by_type":  byType,
		"current_strategy":   r.strategy,
	}
}

// ResetStats resets the statistics counters.
func (r *ConflictResolver) ResetStats() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.conflictsResolved = 0
	r.conflictsByType = make(map[string]int64)
}

// Additional conflict resolution strategies

// ConflictResolutionMerge merges compatible values.
const ConflictResolutionMerge ConflictResolution = "merge"

// ConflictResolutionCustom uses a custom handler.
const ConflictResolutionCustom ConflictResolution = "custom"

// BatchConflictResolver handles multiple conflicts efficiently.
type BatchConflictResolver struct {
	resolver   *ConflictResolver
	conflicts  []*Conflict
	resolutions []*Resolution
	mu         sync.Mutex
}

// NewBatchConflictResolver creates a batch resolver.
func NewBatchConflictResolver(resolver *ConflictResolver) *BatchConflictResolver {
	return &BatchConflictResolver{
		resolver:   resolver,
		conflicts:  make([]*Conflict, 0),
		resolutions: make([]*Resolution, 0),
	}
}

// AddConflict adds a conflict to the batch.
func (b *BatchConflictResolver) AddConflict(conflict *Conflict) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.conflicts = append(b.conflicts, conflict)
}

// ResolveAll resolves all conflicts in the batch.
func (b *BatchConflictResolver) ResolveAll(ctx context.Context) ([]*Resolution, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.resolutions = make([]*Resolution, 0, len(b.conflicts))

	for _, conflict := range b.conflicts {
		resolution, err := b.resolver.Resolve(ctx, conflict)
		if err != nil {
			// Store nil resolution but continue
			b.resolutions = append(b.resolutions, nil)
			continue
		}
		b.resolutions = append(b.resolutions, resolution)
	}

	return b.resolutions, nil
}

// Clear clears the batch.
func (b *BatchConflictResolver) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.conflicts = make([]*Conflict, 0)
	b.resolutions = make([]*Resolution, 0)
}

// GetConflicts returns all conflicts in the batch.
func (b *BatchConflictResolver) GetConflicts() []*Conflict {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]*Conflict, len(b.conflicts))
	copy(result, b.conflicts)
	return result
}

// ConflictLog records conflicts for auditing.
type ConflictLog struct {
	mu       sync.Mutex
	entries  []ConflictLogEntry
	maxSize  int
}

// ConflictLogEntry is a single conflict log entry.
type ConflictLogEntry struct {
	Conflict   *Conflict   `json:"conflict"`
	Resolution *Resolution `json:"resolution"`
	Timestamp  time.Time   `json:"timestamp"`
}

// NewConflictLog creates a new conflict log.
func NewConflictLog(maxSize int) *ConflictLog {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &ConflictLog{
		entries: make([]ConflictLogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Log records a conflict and its resolution.
func (l *ConflictLog) Log(conflict *Conflict, resolution *Resolution) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := ConflictLogEntry{
		Conflict:   conflict,
		Resolution: resolution,
		Timestamp:  time.Now(),
	}

	// Rotate if at max size
	if len(l.entries) >= l.maxSize {
		l.entries = l.entries[1:]
	}

	l.entries = append(l.entries, entry)
}

// GetEntries returns log entries, optionally filtered by time.
func (l *ConflictLog) GetEntries(since time.Time) []ConflictLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if since.IsZero() {
		result := make([]ConflictLogEntry, len(l.entries))
		copy(result, l.entries)
		return result
	}

	result := make([]ConflictLogEntry, 0)
	for _, entry := range l.entries {
		if entry.Timestamp.After(since) {
			result = append(result, entry)
		}
	}
	return result
}

// Clear clears the log.
func (l *ConflictLog) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]ConflictLogEntry, 0, l.maxSize)
}

// FeatureValueToConflictValue converts a domain.FeatureValue to FeatureConflictValue.
func FeatureValueToConflictValue(fv *domain.FeatureValue, source string) *FeatureConflictValue {
	if fv == nil {
		return nil
	}

	return &FeatureConflictValue{
		Value:     fv.Value,
		Timestamp: time.Unix(0, fv.Timestamp), // Convert from nanoseconds
		Version:   fv.Version,
		Source:    source,
	}
}
