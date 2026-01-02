package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Manager coordinates the migration process from Feast to Feather.
type Manager struct {
	schemaConverter *SchemaConverter
	configMigrator  *ConfigMigrator
	dataMigrator    *DataMigrator

	plans map[string]*MigrationPlan
	jobs  map[string]*MigrationJob
	mu    sync.RWMutex
}

// ManagerConfig configures the migration manager.
type ManagerConfig struct {
	SchemaConfig SchemaConverterConfig
	ConfigConfig ConfigMigratorConfig
	DataConfig   DataMigratorConfig
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		SchemaConfig: DefaultSchemaConverterConfig(),
		ConfigConfig: DefaultConfigMigratorConfig(),
		DataConfig:   DefaultDataMigratorConfig(),
	}
}

// NewManager creates a new migration manager.
func NewManager(config ManagerConfig) *Manager {
	return &Manager{
		schemaConverter: NewSchemaConverter(config.SchemaConfig),
		configMigrator:  NewConfigMigrator(config.ConfigConfig),
		dataMigrator:    NewDataMigrator(config.DataConfig),
		plans:           make(map[string]*MigrationPlan),
		jobs:            make(map[string]*MigrationJob),
	}
}

// AnalyzeProject analyzes a Feast project and returns a migration report.
func (m *Manager) AnalyzeProject(project *FeastProject) (*MigrationReport, error) {
	// Validate project
	if errors := ValidateFeastProject(project); len(errors) > 0 {
		return nil, fmt.Errorf("validation errors: %v", errors)
	}

	// Convert schema
	schemaResult, err := m.schemaConverter.ConvertProject(project)
	if err != nil {
		return nil, fmt.Errorf("schema conversion failed: %w", err)
	}

	// Convert config
	configResult, err := m.configMigrator.ConvertConfig(project)
	if err != nil {
		return nil, fmt.Errorf("config conversion failed: %w", err)
	}

	// Generate report
	report := GenerateMigrationReport(project, schemaResult, configResult)

	return report, nil
}

// ConvertSchema converts a Feast project schema to Feather format.
func (m *Manager) ConvertSchema(project *FeastProject) (*ConvertResult, error) {
	return m.schemaConverter.ConvertProject(project)
}

// ConvertConfig converts Feast configuration to Feather format.
func (m *Manager) ConvertConfig(project *FeastProject) (*ConfigConvertResult, error) {
	return m.configMigrator.ConvertConfig(project)
}

// CreatePlan creates a migration plan.
func (m *Manager) CreatePlan(plan *MigrationPlan) error {
	if plan.ID == "" {
		return fmt.Errorf("plan ID is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plans[plan.ID]; exists {
		return fmt.Errorf("plan '%s' already exists", plan.ID)
	}

	plan.CreatedAt = time.Now()
	plan.Status = "pending"
	m.plans[plan.ID] = plan

	return nil
}

// GetPlan retrieves a migration plan.
func (m *Manager) GetPlan(id string) (*MigrationPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, exists := m.plans[id]
	if !exists {
		return nil, fmt.Errorf("plan '%s' not found", id)
	}

	return plan, nil
}

// ListPlans returns all migration plans.
func (m *Manager) ListPlans() []*MigrationPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MigrationPlan, 0, len(m.plans))
	for _, plan := range m.plans {
		result = append(result, plan)
	}
	return result
}

// DeletePlan removes a migration plan.
func (m *Manager) DeletePlan(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plans[id]; !exists {
		return fmt.Errorf("plan '%s' not found", id)
	}

	delete(m.plans, id)
	return nil
}

// ExecutePlan executes a migration plan.
func (m *Manager) ExecutePlan(ctx context.Context, planID string, source DataSource) (*MigrationJob, error) {
	m.mu.Lock()
	plan, exists := m.plans[planID]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("plan '%s' not found", planID)
	}

	jobID := fmt.Sprintf("%s_%d", planID, time.Now().Unix())
	job := &MigrationJob{
		ID:        jobID,
		PlanID:    planID,
		StartedAt: time.Now(),
		Status:    "running",
	}
	m.jobs[jobID] = job
	plan.Status = "running"
	m.mu.Unlock()

	// Execute migration
	go func() {
		stats, err := m.dataMigrator.MigrateFromSource(ctx, source, plan.FieldMapping)

		m.mu.Lock()
		defer m.mu.Unlock()

		job.Stats = stats
		job.CompletedAt = time.Now()

		if err != nil {
			job.Status = "failed"
			plan.Status = "failed"
		} else {
			job.Status = "completed"
			plan.Status = "completed"
		}
	}()

	return job, nil
}

// GetJob retrieves a migration job.
func (m *Manager) GetJob(id string) (*MigrationJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job '%s' not found", id)
	}

	return job, nil
}

// ListJobs returns all migration jobs.
func (m *Manager) ListJobs() []*MigrationJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MigrationJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		result = append(result, job)
	}
	return result
}

// FullMigration performs a complete migration from Feast to Feather.
type FullMigration struct {
	Project      *FeastProject        `json:"project"`
	SchemaResult *ConvertResult       `json:"schema_result"`
	ConfigResult *ConfigConvertResult `json:"config_result"`
	DataStats    *MigrationStats      `json:"data_stats,omitempty"`
	Report       *MigrationReport     `json:"report"`
	StartedAt    time.Time            `json:"started_at"`
	CompletedAt  time.Time            `json:"completed_at,omitempty"`
	Status       string               `json:"status"`
}

// RunFullMigration runs a complete migration without data.
func (m *Manager) RunFullMigration(project *FeastProject) (*FullMigration, error) {
	migration := &FullMigration{
		Project:   project,
		StartedAt: time.Now(),
		Status:    "running",
	}

	// Convert schema
	schemaResult, err := m.ConvertSchema(project)
	if err != nil {
		migration.Status = "failed"
		return migration, fmt.Errorf("schema conversion: %w", err)
	}
	migration.SchemaResult = schemaResult

	// Convert config
	configResult, err := m.ConvertConfig(project)
	if err != nil {
		migration.Status = "failed"
		return migration, fmt.Errorf("config conversion: %w", err)
	}
	migration.ConfigResult = configResult

	// Generate report
	migration.Report = GenerateMigrationReport(project, schemaResult, configResult)

	migration.CompletedAt = time.Now()
	migration.Status = "completed"

	return migration, nil
}

// ExportMigration exports migration results as JSON.
func (m *Manager) ExportMigration(migration *FullMigration) ([]byte, error) {
	return json.MarshalIndent(migration, "", "  ")
}

// ManagerStats contains manager statistics.
type ManagerStats struct {
	TotalPlans    int `json:"total_plans"`
	TotalJobs     int `json:"total_jobs"`
	RunningJobs   int `json:"running_jobs"`
	CompletedJobs int `json:"completed_jobs"`
	FailedJobs    int `json:"failed_jobs"`
}

// Stats returns manager statistics.
func (m *Manager) Stats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ManagerStats{
		TotalPlans: len(m.plans),
		TotalJobs:  len(m.jobs),
	}

	for _, job := range m.jobs {
		switch job.Status {
		case "running":
			stats.RunningJobs++
		case "completed":
			stats.CompletedJobs++
		case "failed":
			stats.FailedJobs++
		}
	}

	return stats
}
