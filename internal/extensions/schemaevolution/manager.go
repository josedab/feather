package schemaevolution

import (
	"fmt"
	"sync"
	"time"
)

// CompatibilityMode defines how schema changes are validated.
type CompatibilityMode string

const (
	// CompatBackward allows new schema to read old data.
	CompatBackward CompatibilityMode = "backward"
	// CompatForward allows old schema to read new data.
	CompatForward CompatibilityMode = "forward"
	// CompatFull requires both backward and forward compatibility.
	CompatFull CompatibilityMode = "full"
	// CompatNone disables compatibility checking.
	CompatNone CompatibilityMode = "none"
)

// FieldChange represents a change to a schema field.
type FieldChange struct {
	Field      string `json:"field"`
	ChangeType string `json:"change_type"` // "added", "removed", "modified"
	OldType    string `json:"old_type,omitempty"`
	NewType    string `json:"new_type,omitempty"`
	HasDefault bool   `json:"has_default"`
}

// SchemaVersion represents a versioned schema definition.
type SchemaVersion struct {
	Group     string            `json:"group"`
	Version   int               `json:"version"`
	Fields    map[string]string `json:"fields"` // field name -> type
	Defaults  map[string]string `json:"defaults,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	Active    bool              `json:"active"`
}

// MigrationStatus represents the status of a migration.
type MigrationStatus string

const (
	MigrationPending    MigrationStatus = "pending"
	MigrationRunning    MigrationStatus = "running"
	MigrationCompleted  MigrationStatus = "completed"
	MigrationRolledBack MigrationStatus = "rolled_back"
	MigrationFailed     MigrationStatus = "failed"
)

// Migration represents a schema migration.
type Migration struct {
	ID            string            `json:"id"`
	Group         string            `json:"group"`
	FromVersion   int               `json:"from_version"`
	ToVersion     int               `json:"to_version"`
	Changes       []FieldChange     `json:"changes"`
	Status        MigrationStatus   `json:"status"`
	Compatible    bool              `json:"compatible"`
	Compatibility CompatibilityMode `json:"compatibility_mode"`
	CreatedAt     time.Time         `json:"created_at"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	ErrorMessage  string            `json:"error_message,omitempty"`
}

// CompatibilityReport describes compatibility of a schema change.
type CompatibilityReport struct {
	Compatible bool              `json:"compatible"`
	Mode       CompatibilityMode `json:"mode"`
	Changes    []FieldChange     `json:"changes"`
	Warnings   []string          `json:"warnings,omitempty"`
	Errors     []string          `json:"errors,omitempty"`
}

// ManagerConfig configures the schema evolution manager.
type ManagerConfig struct {
	DefaultCompatibility CompatibilityMode `json:"default_compatibility"`
	MaxVersionsPerGroup  int               `json:"max_versions_per_group"`
	MaxMigrations        int               `json:"max_migrations"`
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		DefaultCompatibility: CompatBackward,
		MaxVersionsPerGroup:  100,
		MaxMigrations:        1000,
	}
}

// Manager orchestrates schema migrations.
type Manager struct {
	mu         sync.RWMutex
	config     ManagerConfig
	schemas    map[string][]*SchemaVersion // group -> versions
	migrations []Migration
	migrating  bool
}

// NewManager creates a new schema evolution manager.
func NewManager(config ManagerConfig) *Manager {
	if config.MaxVersionsPerGroup == 0 {
		config = DefaultManagerConfig()
	}
	return &Manager{
		config:     config,
		schemas:    make(map[string][]*SchemaVersion),
		migrations: make([]Migration, 0),
	}
}

// RegisterSchema registers the initial schema version for a group.
func (m *Manager) RegisterSchema(group string, fields map[string]string) (*SchemaVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schemas[group]; exists {
		return nil, fmt.Errorf("schema for group %q already exists, use Evolve", group)
	}

	now := time.Now()
	sv := &SchemaVersion{
		Group:     group,
		Version:   1,
		Fields:    fields,
		Defaults:  make(map[string]string),
		CreatedAt: now,
		Active:    true,
	}

	m.schemas[group] = []*SchemaVersion{sv}
	return sv, nil
}

// Evolve creates a new schema version with compatibility validation.
func (m *Manager) Evolve(group string, newFields map[string]string, defaults map[string]string) (*Migration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.schemas[group]
	if !exists {
		return nil, ErrSchemaNotFound
	}

	if m.migrating {
		return nil, ErrMigrationInProgress
	}

	current := versions[len(versions)-1]

	// Compute changes
	changes := computeChanges(current.Fields, newFields)

	// Check compatibility
	report := checkCompatibility(changes, m.config.DefaultCompatibility, defaults)

	migration := Migration{
		ID:            fmt.Sprintf("%s-v%d-to-v%d", group, current.Version, current.Version+1),
		Group:         group,
		FromVersion:   current.Version,
		ToVersion:     current.Version + 1,
		Changes:       changes,
		Status:        MigrationPending,
		Compatible:    report.Compatible,
		Compatibility: m.config.DefaultCompatibility,
		CreatedAt:     time.Now(),
	}

	if !report.Compatible {
		migration.Status = MigrationFailed
		migration.ErrorMessage = fmt.Sprintf("incompatible: %v", report.Errors)
		m.migrations = append(m.migrations, migration)
		return &migration, fmt.Errorf("%w: %v", ErrIncompatibleSchema, report.Errors)
	}

	// Apply migration
	newVersion := &SchemaVersion{
		Group:     group,
		Version:   current.Version + 1,
		Fields:    newFields,
		Defaults:  defaults,
		CreatedAt: time.Now(),
		Active:    true,
	}
	current.Active = false

	m.schemas[group] = append(versions, newVersion)
	if len(m.schemas[group]) > m.config.MaxVersionsPerGroup {
		m.schemas[group] = m.schemas[group][1:]
	}

	migration.Status = MigrationCompleted
	now := time.Now()
	migration.CompletedAt = &now
	m.migrations = append(m.migrations, migration)

	return &migration, nil
}

// CheckCompatibility validates a proposed schema change without applying it.
func (m *Manager) CheckCompatibility(group string, newFields map[string]string, defaults map[string]string) (*CompatibilityReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.schemas[group]
	if !exists {
		return nil, ErrSchemaNotFound
	}

	current := versions[len(versions)-1]
	changes := computeChanges(current.Fields, newFields)
	report := checkCompatibility(changes, m.config.DefaultCompatibility, defaults)
	return &report, nil
}

// GetSchema returns the current active schema for a group.
func (m *Manager) GetSchema(group string) (*SchemaVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.schemas[group]
	if !exists {
		return nil, ErrSchemaNotFound
	}
	return versions[len(versions)-1], nil
}

// GetSchemaVersion returns a specific version.
func (m *Manager) GetSchemaVersion(group string, version int) (*SchemaVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.schemas[group]
	if !exists {
		return nil, ErrSchemaNotFound
	}
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, ErrSchemaNotFound
}

// ListSchemas returns all groups and their current version.
func (m *Manager) ListSchemas() []SchemaVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SchemaVersion, 0, len(m.schemas))
	for _, versions := range m.schemas {
		if len(versions) > 0 {
			result = append(result, *versions[len(versions)-1])
		}
	}
	return result
}

// ListMigrations returns migration history.
func (m *Manager) ListMigrations(group string) []Migration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Migration
	for _, mig := range m.migrations {
		if group == "" || mig.Group == group {
			result = append(result, mig)
		}
	}
	return result
}

// Rollback reverts the last migration for a group.
func (m *Manager) Rollback(group string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions, exists := m.schemas[group]
	if !exists || len(versions) < 2 {
		return ErrSchemaNotFound
	}

	// Revert to previous version
	versions[len(versions)-1].Active = false
	versions[len(versions)-2].Active = true
	m.schemas[group] = versions[:len(versions)-1]

	return nil
}

// Stats returns aggregate statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats ManagerStats
	stats.TotalGroups = len(m.schemas)
	for _, versions := range m.schemas {
		stats.TotalVersions += len(versions)
	}
	stats.TotalMigrations = len(m.migrations)
	for _, mig := range m.migrations {
		if mig.Status == MigrationCompleted {
			stats.SuccessfulMigrations++
		} else if mig.Status == MigrationFailed {
			stats.FailedMigrations++
		}
	}
	return stats
}

// ManagerStats provides aggregate statistics.
type ManagerStats struct {
	TotalGroups          int `json:"total_groups"`
	TotalVersions        int `json:"total_versions"`
	TotalMigrations      int `json:"total_migrations"`
	SuccessfulMigrations int `json:"successful_migrations"`
	FailedMigrations     int `json:"failed_migrations"`
}

func computeChanges(oldFields, newFields map[string]string) []FieldChange {
	var changes []FieldChange

	for name, oldType := range oldFields {
		if newType, exists := newFields[name]; !exists {
			changes = append(changes, FieldChange{Field: name, ChangeType: "removed", OldType: oldType})
		} else if newType != oldType {
			changes = append(changes, FieldChange{Field: name, ChangeType: "modified", OldType: oldType, NewType: newType})
		}
	}

	for name, newType := range newFields {
		if _, exists := oldFields[name]; !exists {
			changes = append(changes, FieldChange{Field: name, ChangeType: "added", NewType: newType})
		}
	}

	return changes
}

func checkCompatibility(changes []FieldChange, mode CompatibilityMode, defaults map[string]string) CompatibilityReport {
	report := CompatibilityReport{
		Compatible: true,
		Mode:       mode,
		Changes:    changes,
	}

	if mode == CompatNone {
		return report
	}

	for _, change := range changes {
		switch change.ChangeType {
		case "removed":
			if mode == CompatBackward || mode == CompatFull {
				// Removing fields breaks backward compat if readers expect them
				report.Warnings = append(report.Warnings, fmt.Sprintf("field %q removed", change.Field))
			}
			if mode == CompatForward || mode == CompatFull {
				report.Compatible = false
				report.Errors = append(report.Errors, fmt.Sprintf("cannot remove field %q in %s mode", change.Field, mode))
			}

		case "added":
			_, hasDefault := defaults[change.Field]
			change.HasDefault = hasDefault
			if mode == CompatForward || mode == CompatFull {
				// Adding required fields breaks forward compat
				if !hasDefault {
					report.Warnings = append(report.Warnings, fmt.Sprintf("new field %q without default", change.Field))
				}
			}

		case "modified":
			if !isCoercible(change.OldType, change.NewType) {
				report.Compatible = false
				report.Errors = append(report.Errors, fmt.Sprintf("incompatible type change for %q: %s -> %s", change.Field, change.OldType, change.NewType))
			} else {
				report.Warnings = append(report.Warnings, fmt.Sprintf("type coercion for %q: %s -> %s", change.Field, change.OldType, change.NewType))
			}
		}
	}

	return report
}

func isCoercible(from, to string) bool {
	coercions := map[string][]string{
		"int64":   {"float64", "string"},
		"float64": {"string"},
		"int32":   {"int64", "float64", "string"},
		"bool":    {"string", "int64"},
	}

	if targets, exists := coercions[from]; exists {
		for _, t := range targets {
			if t == to {
				return true
			}
		}
	}
	return false
}
