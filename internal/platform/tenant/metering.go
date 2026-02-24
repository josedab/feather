package tenant

import (
	"fmt"
	"sync"
	"time"
)

// UsageMeter aggregates tenant usage metrics for cost attribution
// and quota enforcement across all resource types.
type UsageMeter struct {
	mu        sync.RWMutex
	records   map[string][]MeterRecord
	summaries map[string]*MeterSummary
	quotas    map[string]map[string]float64 // tenantID -> metric -> limit
}

// MeterRecord captures a single metered usage event.
type MeterRecord struct {
	TenantID  string    `json:"tenant_id"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// MeterSummary provides aggregated usage for a tenant.
type MeterSummary struct {
	TenantID        string             `json:"tenant_id"`
	Period          string             `json:"period"`
	Metrics         map[string]float64 `json:"metrics"`
	CostAttribution map[string]float64 `json:"cost_attribution"`
	LastUpdated     time.Time          `json:"last_updated"`
}

// QuotaExceeded is returned when a tenant exceeds their quota.
type QuotaExceeded struct {
	TenantID string  `json:"tenant_id"`
	Metric   string  `json:"metric"`
	Current  float64 `json:"current"`
	Limit    float64 `json:"limit"`
}

func (e *QuotaExceeded) Error() string {
	return fmt.Sprintf("tenant %s exceeded %s quota: %.0f/%.0f", e.TenantID, e.Metric, e.Current, e.Limit)
}

// CostConfig defines per-metric unit costs for cost attribution.
type CostConfig struct {
	Rates map[string]float64 `json:"rates" yaml:"rates"`
}

// DefaultCostConfig returns default cost rates per metric unit.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		Rates: map[string]float64{
			"requests":      0.0001,  // $0.0001 per request
			"storage_gb":    0.023,   // $0.023 per GB/month
			"entities":      0.00001, // $0.00001 per entity
			"vector_ops":    0.0005,  // $0.0005 per vector op
			"data_transfer": 0.09,    // $0.09 per GB transfer
			"compute_hours": 0.034,   // $0.034 per compute hour
		},
	}
}

// NewUsageMeter creates a new usage meter.
func NewUsageMeter() *UsageMeter {
	return &UsageMeter{
		records:   make(map[string][]MeterRecord),
		summaries: make(map[string]*MeterSummary),
		quotas:    make(map[string]map[string]float64),
	}
}

// SetQuota sets a usage limit for a tenant on a specific metric.
func (m *UsageMeter) SetQuota(tenantID, metric string, limit float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.quotas[tenantID]; !exists {
		m.quotas[tenantID] = make(map[string]float64)
	}
	m.quotas[tenantID][metric] = limit
}

// Record captures a usage event, returning an error if quota is exceeded.
func (m *UsageMeter) Record(tenantID, metric string, value float64) error {
	if value < 0 {
		return fmt.Errorf("usage value must be non-negative, got %f", value)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check period rollover.
	currentPeriod := time.Now().Format("2006-01")
	summary, exists := m.summaries[tenantID]
	if !exists || summary.Period != currentPeriod {
		summary = &MeterSummary{
			TenantID:        tenantID,
			Period:          currentPeriod,
			Metrics:         make(map[string]float64),
			CostAttribution: make(map[string]float64),
		}
		m.summaries[tenantID] = summary
		// Clear records for new period.
		m.records[tenantID] = nil
	}

	// Check quota before recording.
	if quotas, ok := m.quotas[tenantID]; ok {
		if limit, hasLimit := quotas[metric]; hasLimit {
			newUsage := summary.Metrics[metric] + value
			if newUsage > limit {
				return &QuotaExceeded{
					TenantID: tenantID,
					Metric:   metric,
					Current:  newUsage,
					Limit:    limit,
				}
			}
		}
	}

	record := MeterRecord{
		TenantID:  tenantID,
		Metric:    metric,
		Value:     value,
		Timestamp: time.Now(),
	}
	m.records[tenantID] = append(m.records[tenantID], record)
	summary.Metrics[metric] += value
	summary.LastUpdated = time.Now()
	return nil
}

// GetSummary returns usage summary for a tenant.
func (m *UsageMeter) GetSummary(tenantID string) (*MeterSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, exists := m.summaries[tenantID]
	if !exists {
		return nil, fmt.Errorf("no usage data for tenant %s", tenantID)
	}
	return s, nil
}

// GetCostAttribution returns cost breakdown for a tenant.
func (m *UsageMeter) GetCostAttribution(tenantID string, costs CostConfig) (map[string]float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, exists := m.summaries[tenantID]
	if !exists {
		return nil, fmt.Errorf("no usage data for tenant %s", tenantID)
	}
	attribution := make(map[string]float64)
	for metric, usage := range s.Metrics {
		rate, ok := costs.Rates[metric]
		if !ok {
			continue
		}
		attribution[metric] = usage * rate
	}
	return attribution, nil
}

// GetAllSummaries returns usage summaries for all tenants.
func (m *UsageMeter) GetAllSummaries() map[string]*MeterSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*MeterSummary, len(m.summaries))
	for k, v := range m.summaries {
		result[k] = v
	}
	return result
}

// Reset clears all usage data.
func (m *UsageMeter) Reset(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.records, tenantID)
	delete(m.summaries, tenantID)
}
