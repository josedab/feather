package sla

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSLASpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    Spec
		wantErr bool
	}{
		{
			name: "valid latency SLA",
			spec: Spec{
				Name:    "api-latency",
				Type:    TypeLatency,
				Target:  100,
				Window:  5 * time.Minute,
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "valid availability SLA",
			spec: Spec{
				Name:    "api-availability",
				Type:    TypeAvailability,
				Target:  99.9,
				Window:  24 * time.Hour,
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			spec: Spec{
				Type:   TypeLatency,
				Target: 100,
				Window: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "missing type",
			spec: Spec{
				Name:   "test",
				Target: 100,
				Window: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid target",
			spec: Spec{
				Name:   "test",
				Type:   TypeLatency,
				Target: 0,
				Window: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid window",
			spec: Spec{
				Name:   "test",
				Type:   TypeLatency,
				Target: 100,
				Window: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid availability target",
			spec: Spec{
				Name:   "test",
				Type:   TypeAvailability,
				Target: 150, // > 100%
				Window: 5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "unknown type",
			spec: Spec{
				Name:   "test",
				Type:   "unknown",
				Target: 100,
				Window: 5 * time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPriority_String(t *testing.T) {
	tests := []struct {
		priority Priority
		want     string
	}{
		{PriorityLow, "low"},
		{PriorityMedium, "medium"},
		{PriorityHigh, "high"},
		{PriorityCritical, "critical"},
		{Priority(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.priority.String(); got != tt.want {
				t.Errorf("Priority.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	config := DefaultManagerConfig()
	manager := NewManager(nil, config)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.checkInterval != config.CheckInterval {
		t.Errorf("checkInterval = %v, want %v", manager.checkInterval, config.CheckInterval)
	}

	if manager.maxBreaches != config.MaxBreachHistory {
		t.Errorf("maxBreaches = %v, want %v", manager.maxBreaches, config.MaxBreachHistory)
	}
}

func TestManager_RegisterSLA(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	spec := &Spec{
		Name:    "test-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	}

	err := manager.RegisterSLA(spec)
	if err != nil {
		t.Fatalf("RegisterSLA failed: %v", err)
	}

	// Verify it's registered
	got, err := manager.GetSLA("test-sla")
	if err != nil {
		t.Fatalf("GetSLA failed: %v", err)
	}

	if got.Name != spec.Name {
		t.Errorf("SLA name = %v, want %v", got.Name, spec.Name)
	}
}

func TestManager_RegisterSLA_Invalid(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	spec := &Spec{
		Name: "", // Invalid - missing name
		Type: TypeLatency,
	}

	err := manager.RegisterSLA(spec)
	if err == nil {
		t.Error("RegisterSLA should fail with invalid spec")
	}
}

func TestManager_UnregisterSLA(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	spec := &Spec{
		Name:    "test-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	err := manager.UnregisterSLA("test-sla")
	if err != nil {
		t.Fatalf("UnregisterSLA failed: %v", err)
	}

	// Verify it's removed
	_, err = manager.GetSLA("test-sla")
	if err == nil {
		t.Error("GetSLA should fail after unregister")
	}
}

func TestManager_UnregisterSLA_NotFound(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	err := manager.UnregisterSLA("nonexistent")
	if err == nil {
		t.Error("UnregisterSLA should fail for nonexistent SLA")
	}
}

func TestManager_ListSLAs(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	specs := []*Spec{
		{Name: "sla-1", Type: TypeLatency, Target: 100, Window: time.Minute, Enabled: true},
		{Name: "sla-2", Type: TypeAvailability, Target: 99.9, Window: time.Hour, Enabled: true},
	}

	for _, spec := range specs {
		_ = manager.RegisterSLA(spec)
	}

	list := manager.ListSLAs()
	if len(list) != 2 {
		t.Errorf("ListSLAs returned %d SLAs, want 2", len(list))
	}
}

func TestManager_GetStatus(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	spec := &Spec{
		Name:    "test-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	status, err := manager.GetStatus("test-sla")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Spec.Name != spec.Name {
		t.Errorf("Status spec name = %v, want %v", status.Spec.Name, spec.Name)
	}

	if status.CompliancePercentage != 100.0 {
		t.Errorf("Initial compliance = %v, want 100.0", status.CompliancePercentage)
	}
}

func TestManager_GetStatus_NotFound(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	_, err := manager.GetStatus("nonexistent")
	if err == nil {
		t.Error("GetStatus should fail for nonexistent SLA")
	}
}

func TestManager_GetAllStatuses(t *testing.T) {
	manager := NewManager(nil, DefaultManagerConfig())

	specs := []*Spec{
		{Name: "sla-1", Type: TypeLatency, Target: 100, Window: time.Minute, Enabled: true},
		{Name: "sla-2", Type: TypeAvailability, Target: 99.9, Window: time.Hour, Enabled: true},
	}

	for _, spec := range specs {
		_ = manager.RegisterSLA(spec)
	}

	statuses := manager.GetAllStatuses()
	if len(statuses) != 2 {
		t.Errorf("GetAllStatuses returned %d statuses, want 2", len(statuses))
	}
}

// mockMetricsProvider implements MetricsProvider for testing.
type mockMetricsProvider struct {
	latency      time.Duration
	freshness    time.Duration
	availability float64
	throughput   float64
}

func (m *mockMetricsProvider) GetLatencyP99(_ context.Context, _ string, _ time.Duration) (time.Duration, error) {
	return m.latency, nil
}

func (m *mockMetricsProvider) GetFreshness(_ context.Context, _ string) (time.Duration, error) {
	return m.freshness, nil
}

func (m *mockMetricsProvider) GetAvailability(_ context.Context, _ string, _ time.Duration) (float64, error) {
	return m.availability, nil
}

func (m *mockMetricsProvider) GetThroughput(_ context.Context, _ string, _ time.Duration) (float64, error) {
	return m.throughput, nil
}

// mockAlertHandler implements AlertHandler for testing.
type mockAlertHandler struct {
	warningCount  int32
	breachCount   int32
	recoveryCount int32
}

func (m *mockAlertHandler) OnWarning(_ context.Context, _ *Status) error {
	atomic.AddInt32(&m.warningCount, 1)
	return nil
}

func (m *mockAlertHandler) OnBreach(_ context.Context, _ *Breach) error {
	atomic.AddInt32(&m.breachCount, 1)
	return nil
}

func (m *mockAlertHandler) OnRecovery(_ context.Context, _ string) error {
	atomic.AddInt32(&m.recoveryCount, 1)
	return nil
}

func TestManager_EvaluateSLA_Latency(t *testing.T) {
	metrics := &mockMetricsProvider{
		latency: 150 * time.Millisecond, // Exceeds 100ms target
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	spec := &Spec{
		Name:    "latency-sla",
		Type:    TypeLatency,
		Target:  100, // 100ms
		Window:  5 * time.Minute,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	ctx := context.Background()
	manager.EvaluateNow(ctx)

	status, _ := manager.GetStatus("latency-sla")
	if !status.IsBreached {
		t.Error("SLA should be breached (150ms > 100ms)")
	}

	if status.CurrentValue != 150 {
		t.Errorf("CurrentValue = %v, want 150", status.CurrentValue)
	}
}

func TestManager_EvaluateSLA_Availability(t *testing.T) {
	metrics := &mockMetricsProvider{
		availability: 99.5, // Below 99.9 target
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	spec := &Spec{
		Name:    "availability-sla",
		Type:    TypeAvailability,
		Target:  99.9,
		Window:  24 * time.Hour,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	ctx := context.Background()
	manager.EvaluateNow(ctx)

	status, _ := manager.GetStatus("availability-sla")
	if !status.IsBreached {
		t.Error("SLA should be breached (99.5 < 99.9)")
	}
}

func TestManager_EvaluateSLA_Healthy(t *testing.T) {
	metrics := &mockMetricsProvider{
		latency: 50 * time.Millisecond, // Below 100ms target
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	spec := &Spec{
		Name:    "healthy-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	ctx := context.Background()
	manager.EvaluateNow(ctx)

	status, _ := manager.GetStatus("healthy-sla")
	if status.IsBreached {
		t.Error("SLA should not be breached (50ms < 100ms)")
	}
}

func TestManager_AlertHandler_Breach(t *testing.T) {
	metrics := &mockMetricsProvider{
		latency: 150 * time.Millisecond,
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	handler := &mockAlertHandler{}
	manager.AddAlertHandler(handler)

	spec := &Spec{
		Name:    "alert-test-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	ctx := context.Background()
	manager.EvaluateNow(ctx)

	if atomic.LoadInt32(&handler.breachCount) != 1 {
		t.Errorf("OnBreach called %d times, want 1", handler.breachCount)
	}
}

func TestManager_AlertHandler_Recovery(t *testing.T) {
	metrics := &mockMetricsProvider{
		latency: 150 * time.Millisecond, // Start breached
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	handler := &mockAlertHandler{}
	manager.AddAlertHandler(handler)

	spec := &Spec{
		Name:    "recovery-test-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	ctx := context.Background()

	// First evaluation - breach
	manager.EvaluateNow(ctx)

	// Update metrics to healthy
	metrics.latency = 50 * time.Millisecond

	// Second evaluation - recovery
	manager.EvaluateNow(ctx)

	if atomic.LoadInt32(&handler.recoveryCount) != 1 {
		t.Errorf("OnRecovery called %d times, want 1", handler.recoveryCount)
	}
}

func TestManager_GetBreaches(t *testing.T) {
	metrics := &mockMetricsProvider{
		latency: 150 * time.Millisecond,
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	spec := &Spec{
		Name:    "breach-history-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: true,
	}

	_ = manager.RegisterSLA(spec)

	ctx := context.Background()
	manager.EvaluateNow(ctx)

	breaches := manager.GetBreaches(time.Now().Add(-1 * time.Hour))
	if len(breaches) != 1 {
		t.Errorf("GetBreaches returned %d breaches, want 1", len(breaches))
	}

	if breaches[0].SLAName != "breach-history-sla" {
		t.Errorf("Breach SLA name = %v, want breach-history-sla", breaches[0].SLAName)
	}
}

func TestManager_GetComplianceSummary(t *testing.T) {
	metrics := &mockMetricsProvider{
		latency:      50 * time.Millisecond,
		availability: 99.99,
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	specs := []*Spec{
		{Name: "sla-1", Type: TypeLatency, Target: 100, Window: time.Minute, Enabled: true},
		{Name: "sla-2", Type: TypeAvailability, Target: 99.9, Window: time.Hour, Enabled: true},
	}

	for _, spec := range specs {
		_ = manager.RegisterSLA(spec)
	}

	ctx := context.Background()
	manager.EvaluateNow(ctx)

	summary := manager.GetComplianceSummary()

	if summary["totalSLAs"].(int) != 2 {
		t.Errorf("totalSLAs = %v, want 2", summary["totalSLAs"])
	}

	if summary["healthy"].(int) != 2 {
		t.Errorf("healthy = %v, want 2", summary["healthy"])
	}

	if summary["compliancePercent"].(float64) != 100.0 {
		t.Errorf("compliancePercent = %v, want 100.0", summary["compliancePercent"])
	}
}

func TestManager_DisabledSLA(t *testing.T) {
	metrics := &mockMetricsProvider{
		latency: 150 * time.Millisecond,
	}
	manager := NewManager(metrics, DefaultManagerConfig())

	spec := &Spec{
		Name:    "disabled-sla",
		Type:    TypeLatency,
		Target:  100,
		Window:  5 * time.Minute,
		Enabled: false, // Disabled
	}

	_ = manager.RegisterSLA(spec)

	ctx := context.Background()
	manager.EvaluateNow(ctx)

	status, _ := manager.GetStatus("disabled-sla")
	// Status should not be updated for disabled SLA
	if status.IsBreached {
		t.Error("Disabled SLA should not be evaluated")
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.CheckInterval != 30*time.Second {
		t.Errorf("CheckInterval = %v, want 30s", config.CheckInterval)
	}

	if config.MaxBreachHistory != 1000 {
		t.Errorf("MaxBreachHistory = %v, want 1000", config.MaxBreachHistory)
	}
}
