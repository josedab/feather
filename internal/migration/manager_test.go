package migration

import (
	"context"
	"testing"
	"time"

	"github.com/feather-store/feather/internal/domain"
)

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(config)

	if manager == nil {
		t.Fatal("Expected manager to be non-nil")
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.SchemaConfig.DefaultTTL != 5*time.Minute {
		t.Errorf("Expected default TTL 5m, got %v", config.SchemaConfig.DefaultTTL)
	}
	if config.DataConfig.BatchSize != 1000 {
		t.Errorf("Expected batch size 1000, got %d", config.DataConfig.BatchSize)
	}
}

func TestManager_AnalyzeProject(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	ttl := 10 * time.Minute
	project := &FeastProject{
		Name:     "test_project",
		Provider: "local",
		Entities: []FeastEntity{
			{Name: "user_id", ValueType: FeastTypeString},
		},
		FeatureViews: []FeastFeatureView{
			{
				Name:     "user_features",
				Entities: []string{"user_id"},
				Features: []FeastFeature{
					{Name: "age", ValueType: FeastTypeInt64},
				},
				TTL:    &ttl,
				Online: true,
			},
		},
	}

	report, err := manager.AnalyzeProject(project)
	if err != nil {
		t.Fatalf("AnalyzeProject failed: %v", err)
	}

	if report == nil {
		t.Fatal("Expected report to be non-nil")
	}

	if report.ProjectName != "test_project" {
		t.Errorf("Expected project name 'test_project', got '%s'", report.ProjectName)
	}

	if report.CompatibilityScore <= 0 {
		t.Error("Expected positive compatibility score")
	}
}

func TestManager_AnalyzeProject_Invalid(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	// Empty project name
	project := &FeastProject{
		Name: "",
	}

	_, err := manager.AnalyzeProject(project)
	if err == nil {
		t.Error("Expected error for invalid project")
	}
}

func TestManager_ConvertSchema(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	project := &FeastProject{
		Name: "test",
		FeatureViews: []FeastFeatureView{
			{
				Name: "features",
				Features: []FeastFeature{
					{Name: "value", ValueType: FeastTypeDouble},
				},
			},
		},
	}

	result, err := manager.ConvertSchema(project)
	if err != nil {
		t.Fatalf("ConvertSchema failed: %v", err)
	}

	if len(result.FeatureGroups) != 1 {
		t.Errorf("Expected 1 feature group, got %d", len(result.FeatureGroups))
	}
}

func TestManager_ConvertConfig(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	project := &FeastProject{
		Name: "test",
	}

	result, err := manager.ConvertConfig(project)
	if err != nil {
		t.Fatalf("ConvertConfig failed: %v", err)
	}

	if result.Config == nil {
		t.Error("Expected config to be non-nil")
	}
}

func TestManager_PlanCRUD(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	// Create plan
	plan := &MigrationPlan{
		ID:         "plan-1",
		Name:       "Test Plan",
		SourceType: "parquet",
	}

	err := manager.CreatePlan(plan)
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	// Get plan
	retrieved, err := manager.GetPlan("plan-1")
	if err != nil {
		t.Fatalf("GetPlan failed: %v", err)
	}
	if retrieved.Name != "Test Plan" {
		t.Errorf("Expected name 'Test Plan', got '%s'", retrieved.Name)
	}
	if retrieved.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", retrieved.Status)
	}

	// List plans
	plans := manager.ListPlans()
	if len(plans) != 1 {
		t.Errorf("Expected 1 plan, got %d", len(plans))
	}

	// Delete plan
	err = manager.DeletePlan("plan-1")
	if err != nil {
		t.Fatalf("DeletePlan failed: %v", err)
	}

	// Verify deletion
	_, err = manager.GetPlan("plan-1")
	if err == nil {
		t.Error("Expected error after deletion")
	}
}

func TestManager_CreatePlan_EmptyID(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	plan := &MigrationPlan{
		ID:   "",
		Name: "Test Plan",
	}

	err := manager.CreatePlan(plan)
	if err == nil {
		t.Error("Expected error for empty ID")
	}
}

func TestManager_CreatePlan_Duplicate(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	plan := &MigrationPlan{
		ID:   "plan-1",
		Name: "Test Plan",
	}

	_ = manager.CreatePlan(plan)
	err := manager.CreatePlan(plan)
	if err == nil {
		t.Error("Expected error for duplicate plan")
	}
}

func TestManager_GetPlan_NotFound(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	_, err := manager.GetPlan("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent plan")
	}
}

func TestManager_DeletePlan_NotFound(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	err := manager.DeletePlan("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent plan")
	}
}

func TestManager_ListJobs(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	jobs := manager.ListJobs()
	if jobs == nil {
		t.Error("Expected jobs list to be non-nil")
	}
	if len(jobs) != 0 {
		t.Errorf("Expected 0 jobs, got %d", len(jobs))
	}
}

func TestManager_GetJob_NotFound(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	_, err := manager.GetJob("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent job")
	}
}

func TestManager_RunFullMigration(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	project := &FeastProject{
		Name: "test_project",
		Entities: []FeastEntity{
			{Name: "user_id", ValueType: FeastTypeString},
		},
		FeatureViews: []FeastFeatureView{
			{
				Name:     "user_features",
				Entities: []string{"user_id"},
				Features: []FeastFeature{
					{Name: "age", ValueType: FeastTypeInt64},
				},
			},
		},
	}

	migration, err := manager.RunFullMigration(project)
	if err != nil {
		t.Fatalf("RunFullMigration failed: %v", err)
	}

	if migration.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", migration.Status)
	}

	if migration.SchemaResult == nil {
		t.Error("Expected SchemaResult to be non-nil")
	}

	if migration.ConfigResult == nil {
		t.Error("Expected ConfigResult to be non-nil")
	}

	if migration.Report == nil {
		t.Error("Expected Report to be non-nil")
	}
}

func TestManager_ExportMigration(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	migration := &FullMigration{
		Project: &FeastProject{Name: "test"},
		Status:  "completed",
	}

	data, err := manager.ExportMigration(migration)
	if err != nil {
		t.Fatalf("ExportMigration failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty export data")
	}
}

func TestManager_Stats(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	// Add some plans
	_ = manager.CreatePlan(&MigrationPlan{ID: "plan-1", Name: "Plan 1"})
	_ = manager.CreatePlan(&MigrationPlan{ID: "plan-2", Name: "Plan 2"})

	stats := manager.Stats()

	if stats.TotalPlans != 2 {
		t.Errorf("Expected 2 plans, got %d", stats.TotalPlans)
	}
	if stats.TotalJobs != 0 {
		t.Errorf("Expected 0 jobs, got %d", stats.TotalJobs)
	}
}

func TestManager_ExecutePlan(t *testing.T) {
	store := NewMockFeatureStore()
	config := DefaultManagerConfig()
	config.DataConfig.TargetStore = store
	config.DataConfig.Workers = 1
	manager := NewManager(config)

	// Create plan
	plan := &MigrationPlan{
		ID:           "plan-1",
		Name:         "Test Plan",
		FieldMapping: NewFieldMapping(),
	}
	_ = manager.CreatePlan(plan)

	// Create source
	source := NewMockDataSource([][]map[string]interface{}{
		{
			{"entity_id": "user_1", "event_timestamp": "2024-01-15T10:00:00Z", "age": 25},
		},
	})

	// Execute plan
	job, err := manager.ExecutePlan(context.Background(), "plan-1", source)
	if err != nil {
		t.Fatalf("ExecutePlan failed: %v", err)
	}

	if job == nil {
		t.Fatal("Expected job to be non-nil")
	}

	// Wait for job to complete
	time.Sleep(100 * time.Millisecond)

	// Check job status
	job, _ = manager.GetJob(job.ID)
	if job.Status != "completed" && job.Status != "running" {
		t.Errorf("Expected job status 'completed' or 'running', got '%s'", job.Status)
	}
}

func TestManager_ExecutePlan_NotFound(t *testing.T) {
	manager := NewManager(DefaultManagerConfig())

	_, err := manager.ExecutePlan(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("Expected error for nonexistent plan")
	}
}

func TestFullMigration_Structure(t *testing.T) {
	migration := &FullMigration{
		Project: &FeastProject{Name: "test"},
		SchemaResult: &ConvertResult{
			FeatureGroups: make([]domain.FeatureGroup, 1),
		},
		ConfigResult: &ConfigConvertResult{
			Config: &FeatherConfig{},
		},
		Report: &MigrationReport{
			ProjectName:        "test",
			CompatibilityScore: 95.0,
		},
		StartedAt:   time.Now().Add(-1 * time.Minute),
		CompletedAt: time.Now(),
		Status:      "completed",
	}

	if migration.Project.Name != "test" {
		t.Errorf("Expected project name 'test', got '%s'", migration.Project.Name)
	}
	if migration.Report.CompatibilityScore != 95.0 {
		t.Errorf("Expected compatibility score 95.0, got %f", migration.Report.CompatibilityScore)
	}
}

func TestManagerStats_Structure(t *testing.T) {
	stats := ManagerStats{
		TotalPlans:    5,
		TotalJobs:     10,
		RunningJobs:   2,
		CompletedJobs: 7,
		FailedJobs:    1,
	}

	if stats.TotalPlans != 5 {
		t.Errorf("Expected 5 total plans, got %d", stats.TotalPlans)
	}
	if stats.RunningJobs+stats.CompletedJobs+stats.FailedJobs != stats.TotalJobs {
		t.Error("Job counts don't add up")
	}
}
