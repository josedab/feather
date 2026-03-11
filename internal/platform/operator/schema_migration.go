package operator

import (
	"fmt"
	"sync"
	"time"
)

// MigrationOperation defines what kind of schema change is needed.
type MigrationOperation string

const (
	MigrationAddField    MigrationOperation = "add_field"
	MigrationRemoveField MigrationOperation = "remove_field"
	MigrationChangeType  MigrationOperation = "change_type"
	MigrationAddGroup    MigrationOperation = "add_group"
	MigrationRemoveGroup MigrationOperation = "remove_group"
)

// SchemaMigration represents a generated schema migration.
type SchemaMigration struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	Operations  []MigrationStep `json:"operations"`
	CreatedAt   time.Time       `json:"created_at"`
	Applied     bool            `json:"applied"`
	AppliedAt   *time.Time      `json:"applied_at,omitempty"`
	DryRun      bool            `json:"dry_run"`
}

// MigrationStep is a single operation in a migration.
type MigrationStep struct {
	Operation  MigrationOperation `json:"operation"`
	Group      string             `json:"group"`
	Field      string             `json:"field,omitempty"`
	OldType    string             `json:"old_type,omitempty"`
	NewType    string             `json:"new_type,omitempty"`
	Reversible bool               `json:"reversible"`
}

// MigrationGenerator generates schema migrations.
type MigrationGenerator struct {
	mu         sync.RWMutex
	migrations []*SchemaMigration
	nextID     int
}

// NewMigrationGenerator creates a new migration generator.
func NewMigrationGenerator() *MigrationGenerator {
	return &MigrationGenerator{}
}

// GenerateMigration creates a migration based on old and new FeatureGroup specs.
func (g *MigrationGenerator) GenerateMigration(oldGroup, newGroup *FeatureGroup) (*SchemaMigration, error) {
	if oldGroup == nil && newGroup == nil {
		return nil, fmt.Errorf("at least one group must be provided")
	}

	g.mu.Lock()
	g.nextID++
	migrationID := fmt.Sprintf("migration-%d", g.nextID)
	g.mu.Unlock()

	migration := &SchemaMigration{
		ID:        migrationID,
		CreatedAt: time.Now(),
	}

	if oldGroup == nil && newGroup != nil {
		migration.Description = fmt.Sprintf("Create feature group %s", newGroup.ObjectMeta.Name)
		migration.Operations = append(migration.Operations, MigrationStep{
			Operation:  MigrationAddGroup,
			Group:      newGroup.ObjectMeta.Name,
			Reversible: true,
		})
		for _, feat := range newGroup.Spec.Features {
			migration.Operations = append(migration.Operations, MigrationStep{
				Operation:  MigrationAddField,
				Group:      newGroup.ObjectMeta.Name,
				Field:      feat.Name,
				NewType:    feat.Type,
				Reversible: true,
			})
		}
	} else if oldGroup != nil && newGroup == nil {
		migration.Description = fmt.Sprintf("Remove feature group %s", oldGroup.ObjectMeta.Name)
		migration.Operations = append(migration.Operations, MigrationStep{
			Operation:  MigrationRemoveGroup,
			Group:      oldGroup.ObjectMeta.Name,
			Reversible: true,
		})
	} else {
		migration.Description = fmt.Sprintf("Update feature group %s", newGroup.ObjectMeta.Name)
		oldFeatures := make(map[string]string)
		for _, f := range oldGroup.Spec.Features {
			oldFeatures[f.Name] = f.Type
		}
		newFeatures := make(map[string]string)
		for _, f := range newGroup.Spec.Features {
			newFeatures[f.Name] = f.Type
		}

		for name, newType := range newFeatures {
			if oldType, exists := oldFeatures[name]; !exists {
				migration.Operations = append(migration.Operations, MigrationStep{
					Operation:  MigrationAddField,
					Group:      newGroup.ObjectMeta.Name,
					Field:      name,
					NewType:    newType,
					Reversible: true,
				})
			} else if oldType != newType {
				migration.Operations = append(migration.Operations, MigrationStep{
					Operation:  MigrationChangeType,
					Group:      newGroup.ObjectMeta.Name,
					Field:      name,
					OldType:    oldType,
					NewType:    newType,
					Reversible: false,
				})
			}
		}
		for name := range oldFeatures {
			if _, exists := newFeatures[name]; !exists {
				migration.Operations = append(migration.Operations, MigrationStep{
					Operation:  MigrationRemoveField,
					Group:      oldGroup.ObjectMeta.Name,
					Field:      name,
					OldType:    oldFeatures[name],
					Reversible: true,
				})
			}
		}
	}

	g.mu.Lock()
	g.migrations = append(g.migrations, migration)
	g.mu.Unlock()

	return migration, nil
}

// DryRun generates a migration plan without applying it.
func (g *MigrationGenerator) DryRun(oldGroup, newGroup *FeatureGroup) (*SchemaMigration, error) {
	m, err := g.GenerateMigration(oldGroup, newGroup)
	if err != nil {
		return nil, err
	}
	m.DryRun = true
	return m, nil
}

// Apply marks a migration as applied.
func (g *MigrationGenerator) Apply(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, m := range g.migrations {
		if m.ID == id {
			m.Applied = true
			now := time.Now()
			m.AppliedAt = &now
			return nil
		}
	}
	return fmt.Errorf("migration %s not found", id)
}

// ListMigrations returns all migrations.
func (g *MigrationGenerator) ListMigrations() []SchemaMigration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]SchemaMigration, len(g.migrations))
	for i, m := range g.migrations {
		result[i] = *m
	}
	return result
}
