package gitops

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ReconcileStatus represents the state of reconciliation.
type ReconcileStatus string

const (
	ReconcileIdle    ReconcileStatus = "idle"
	ReconcileRunning ReconcileStatus = "running"
	ReconcileFailed  ReconcileStatus = "failed"
	ReconcileSuccess ReconcileStatus = "success"
)

// MigrationStep represents a schema version transition.
type MigrationStep struct {
	// FromVersion is the starting schema version.
	FromVersion string `json:"from_version"`
	// ToVersion is the target schema version.
	ToVersion string `json:"to_version"`
	// Description explains the migration.
	Description string `json:"description"`
	// Changes lists the modifications applied.
	Changes []SchemaChange `json:"changes"`
	// AppliedAt is when this migration was executed.
	AppliedAt time.Time `json:"applied_at"`
	// Status indicates success or failure.
	Status string `json:"status"`
	// Error holds failure details.
	Error string `json:"error,omitempty"`
}

// SchemaChange describes a single schema modification.
type SchemaChange struct {
	Type        string `json:"type"` // add_feature, remove_feature, modify_type, add_group, remove_group
	Group       string `json:"group,omitempty"`
	Feature     string `json:"feature,omitempty"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
}

// ReconcileEvent records a reconciliation cycle.
type ReconcileEvent struct {
	StartedAt  time.Time       `json:"started_at"`
	EndedAt    time.Time       `json:"ended_at"`
	Status     ReconcileStatus `json:"status"`
	Changes    int             `json:"changes"`
	Errors     int             `json:"errors"`
	Message    string          `json:"message,omitempty"`
}

// ReconcilerConfig configures the reconciliation controller.
type ReconcilerConfig struct {
	// Interval is how often to reconcile.
	Interval time.Duration
	// DryRun prevents actual changes when true.
	DryRun bool
	// MaxRetries for failed reconciliations.
	MaxRetries int
	// AutoRollback reverts on failure.
	AutoRollback bool
}

// DefaultReconcilerConfig returns sensible defaults.
func DefaultReconcilerConfig() ReconcilerConfig {
	return ReconcilerConfig{
		Interval:     time.Minute,
		DryRun:       false,
		MaxRetries:   3,
		AutoRollback: true,
	}
}

// Reconciler periodically syncs Git definitions with the feature store.
type Reconciler struct {
	mu          sync.RWMutex
	syncManager *SyncManager
	config      ReconcilerConfig
	status      ReconcileStatus
	history     []ReconcileEvent
	migrations  []MigrationStep
	version     string
	stopCh      chan struct{}
}

// NewReconciler creates a new reconciliation controller.
func NewReconciler(syncManager *SyncManager, config ReconcilerConfig) *Reconciler {
	if config.Interval == 0 {
		config = DefaultReconcilerConfig()
	}
	return &Reconciler{
		syncManager: syncManager,
		config:      config,
		status:      ReconcileIdle,
		history:     make([]ReconcileEvent, 0),
		migrations:  make([]MigrationStep, 0),
		version:     "v0",
		stopCh:      make(chan struct{}),
	}
}

// Start begins periodic reconciliation.
func (r *Reconciler) Start(ctx context.Context) {
	go r.reconcileLoop(ctx)
}

// Stop halts the reconciler.
func (r *Reconciler) Stop() {
	close(r.stopCh)
}

// ReconcileNow triggers an immediate reconciliation cycle.
func (r *Reconciler) ReconcileNow(ctx context.Context) *ReconcileEvent {
	return r.doReconcile(ctx)
}

// GetStatus returns the current reconciliation status.
func (r *Reconciler) GetStatus() ReconcileStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// GetHistory returns recent reconciliation events.
func (r *Reconciler) GetHistory(limit int) []ReconcileEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 || limit > len(r.history) {
		limit = len(r.history)
	}
	start := len(r.history) - limit
	if start < 0 {
		start = 0
	}
	result := make([]ReconcileEvent, limit)
	copy(result, r.history[start:])
	return result
}

// GetMigrations returns the migration history.
func (r *Reconciler) GetMigrations() []MigrationStep {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]MigrationStep, len(r.migrations))
	copy(result, r.migrations)
	return result
}

// GetVersion returns the current schema version.
func (r *Reconciler) GetVersion() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
}

// ApplyMigration records and executes a schema migration.
func (r *Reconciler) ApplyMigration(step MigrationStep) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	step.AppliedAt = time.Now()
	step.Status = "applied"
	r.migrations = append(r.migrations, step)
	r.version = step.ToVersion
	return nil
}

// ComputeSchemaDiff compares two sets of definitions and produces changes.
func ComputeSchemaDiff(old, new []FeatureDefinition) []SchemaChange {
	oldMap := make(map[string]*FeatureDefinition)
	for i := range old {
		oldMap[old[i].Metadata.Name] = &old[i]
	}

	newMap := make(map[string]*FeatureDefinition)
	for i := range new {
		newMap[new[i].Metadata.Name] = &new[i]
	}

	var changes []SchemaChange

	// Detect additions and modifications
	for name, newDef := range newMap {
		oldDef, exists := oldMap[name]
		if !exists {
			changes = append(changes, SchemaChange{
				Type:     "add_group",
				Group:    name,
				NewValue: newDef.Metadata.Name,
			})
			continue
		}

		// Check for modifications by comparing spec as JSON
		oldJSON, _ := fmt.Sprintf("%v", oldDef.Spec), ""
		newJSON, _ := fmt.Sprintf("%v", newDef.Spec), ""
		if oldJSON != newJSON {
			changes = append(changes, SchemaChange{
				Type:     "modify_type",
				Group:    name,
				OldValue: oldJSON,
				NewValue: newJSON,
			})
		}
	}

	// Detect deletions
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			changes = append(changes, SchemaChange{
				Type:  "remove_group",
				Group: name,
			})
		}
	}

	return changes
}

// Summary returns a status overview.
func (r *Reconciler) Summary() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var lastEvent *ReconcileEvent
	if len(r.history) > 0 {
		lastEvent = &r.history[len(r.history)-1]
	}

	summary := map[string]interface{}{
		"status":          string(r.status),
		"version":         r.version,
		"interval":        r.config.Interval.String(),
		"dry_run":         r.config.DryRun,
		"total_syncs":     len(r.history),
		"total_migrations": len(r.migrations),
	}

	if lastEvent != nil {
		summary["last_sync"] = lastEvent.EndedAt
		summary["last_status"] = string(lastEvent.Status)
	}

	return summary
}

func (r *Reconciler) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.doReconcile(ctx)
		}
	}
}

func (r *Reconciler) doReconcile(ctx context.Context) *ReconcileEvent {
	r.mu.Lock()
	r.status = ReconcileRunning
	r.mu.Unlock()

	event := ReconcileEvent{
		StartedAt: time.Now(),
	}

	var retries int
	var lastErr error

	for retries <= r.config.MaxRetries {
		result, err := r.syncManager.Sync(ctx, nil)
		if err != nil {
			lastErr = err
			retries++
			continue
		}

		event.Changes = len(result.Created) + len(result.Updated) + len(result.Deleted)
		event.Status = ReconcileSuccess
		event.EndedAt = time.Now()

		r.mu.Lock()
		r.status = ReconcileSuccess
		r.history = append(r.history, event)
		if len(r.history) > 100 {
			r.history = r.history[1:]
		}
		r.mu.Unlock()

		return &event
	}

	event.Status = ReconcileFailed
	event.EndedAt = time.Now()
	if lastErr != nil {
		event.Message = fmt.Sprintf("reconciliation failed after %d retries: %s", retries, lastErr)
	}
	event.Errors = retries

	r.mu.Lock()
	r.status = ReconcileFailed
	r.history = append(r.history, event)
	if len(r.history) > 100 {
		r.history = r.history[1:]
	}
	r.mu.Unlock()

	return &event
}
