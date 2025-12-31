package cost

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Tracker tracks usage and calculates costs.
type Tracker struct {
	mu           sync.RWMutex
	rates        map[CostCategory]map[CostUnit]*CostRate
	usageRecords []UsageRecord
	costEntries  []CostEntry
	currency     string
	defaultRates map[string]*CostRate
	maxRecords   int
}

// NewTracker creates a new cost tracker.
func NewTracker(currency string) *Tracker {
	t := &Tracker{
		rates:        make(map[CostCategory]map[CostUnit]*CostRate),
		currency:     currency,
		defaultRates: make(map[string]*CostRate),
		maxRecords:   1000000, // Default max records
	}
	t.initializeDefaultRates()
	return t
}

// initializeDefaultRates sets up default pricing.
func (t *Tracker) initializeDefaultRates() {
	defaultRates := []*CostRate{
		{Category: CostCategoryStorage, Unit: CostUnitBytes, PricePerUnit: 0.000000023, Description: "Storage per byte per month"},
		{Category: CostCategoryAPI, Unit: CostUnitRequests, PricePerUnit: 0.0001, Description: "API requests"},
		{Category: CostCategoryCompute, Unit: CostUnitCPUSeconds, PricePerUnit: 0.00005, Description: "CPU seconds"},
		{Category: CostCategoryCompute, Unit: CostUnitGPUSeconds, PricePerUnit: 0.001, Description: "GPU seconds"},
		{Category: CostCategoryNetwork, Unit: CostUnitBytes, PricePerUnit: 0.00000009, Description: "Network egress per byte"},
		{Category: CostCategoryML, Unit: CostUnitTokens, PricePerUnit: 0.00001, Description: "ML inference tokens"},
		{Category: CostCategoryML, Unit: CostUnitEmbeddings, PricePerUnit: 0.0001, Description: "Embedding generations"},
		{Category: CostCategoryVector, Unit: CostUnitRequests, PricePerUnit: 0.00005, Description: "Vector search requests"},
	}

	for _, rate := range defaultRates {
		t.SetRate(rate)
	}
}

// SetRate sets a cost rate for a category/unit combination.
func (t *Tracker) SetRate(rate *CostRate) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.rates[rate.Category] == nil {
		t.rates[rate.Category] = make(map[CostUnit]*CostRate)
	}
	t.rates[rate.Category][rate.Unit] = rate
}

// GetRate returns the cost rate for a category/unit combination.
func (t *Tracker) GetRate(category CostCategory, unit CostUnit) (*CostRate, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.rates[category] == nil {
		return nil, false
	}
	rate, exists := t.rates[category][unit]
	return rate, exists
}

// ListRates returns all configured cost rates.
func (t *Tracker) ListRates() []*CostRate {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var rates []*CostRate
	for _, units := range t.rates {
		for _, rate := range units {
			rates = append(rates, rate)
		}
	}
	return rates
}

// RecordUsage records a usage event and calculates cost.
func (t *Tracker) RecordUsage(record UsageRecord) (*CostEntry, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Generate ID if not provided
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// Get rate
	rate, exists := t.rates[record.Category][record.Unit]
	if !exists {
		return nil, fmt.Errorf("no rate configured for category %s unit %s", record.Category, record.Unit)
	}

	// Calculate cost
	quantity := record.Quantity
	if rate.FreeAllowance > 0 {
		quantity = quantity - rate.FreeAllowance
		if quantity < 0 {
			quantity = 0
		}
	}

	cost := quantity * rate.PricePerUnit
	if rate.MinCharge > 0 && cost < rate.MinCharge && cost > 0 {
		cost = rate.MinCharge
	}

	// Create cost entry
	entry := CostEntry{
		ID:           uuid.New().String(),
		TenantID:     record.TenantID,
		FeatureGroup: record.FeatureGroup,
		Feature:      record.Feature,
		Category:     record.Category,
		Unit:         record.Unit,
		Quantity:     record.Quantity,
		Rate:         rate.PricePerUnit,
		Cost:         cost,
		Currency:     t.currency,
		Timestamp:    record.Timestamp,
	}

	// Store records
	t.usageRecords = append(t.usageRecords, record)
	t.costEntries = append(t.costEntries, entry)

	// Trim if over limit
	if len(t.usageRecords) > t.maxRecords {
		t.usageRecords = t.usageRecords[len(t.usageRecords)-t.maxRecords:]
		t.costEntries = t.costEntries[len(t.costEntries)-t.maxRecords:]
	}

	return &entry, nil
}

// GetUsage returns usage records for a time range.
func (t *Tracker) GetUsage(tenantID string, start, end time.Time) []UsageRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var records []UsageRecord
	for _, r := range t.usageRecords {
		if tenantID != "" && r.TenantID != tenantID {
			continue
		}
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}
		records = append(records, r)
	}
	return records
}

// GetCosts returns cost entries for a time range.
func (t *Tracker) GetCosts(tenantID string, start, end time.Time) []CostEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var entries []CostEntry
	for _, e := range t.costEntries {
		if tenantID != "" && e.TenantID != tenantID {
			continue
		}
		if e.Timestamp.Before(start) || e.Timestamp.After(end) {
			continue
		}
		entries = append(entries, e)
	}
	return entries
}

// GetCostSummary returns an aggregated cost summary.
func (t *Tracker) GetCostSummary(tenantID string, start, end time.Time) *CostSummary {
	entries := t.GetCosts(tenantID, start, end)

	summary := &CostSummary{
		TenantID:    tenantID,
		PeriodStart: start,
		PeriodEnd:   end,
		Currency:    t.currency,
		ByCategory:  make(map[CostCategory]float64),
		ByFeature:   make(map[string]float64),
		ByTenant:    make(map[string]float64),
	}

	for _, e := range entries {
		summary.TotalCost += e.Cost
		summary.ByCategory[e.Category] += e.Cost
		if e.FeatureGroup != "" {
			summary.ByFeature[e.FeatureGroup] += e.Cost
		}
		if e.TenantID != "" {
			summary.ByTenant[e.TenantID] += e.Cost
		}
	}

	return summary
}

// GetFeatureCosts returns costs grouped by feature.
func (t *Tracker) GetFeatureCosts(tenantID string, start, end time.Time) map[string]*CostSummary {
	entries := t.GetCosts(tenantID, start, end)

	features := make(map[string]*CostSummary)
	for _, e := range entries {
		key := e.FeatureGroup
		if key == "" {
			key = "_unattributed"
		}

		if features[key] == nil {
			features[key] = &CostSummary{
				TenantID:     tenantID,
				FeatureGroup: key,
				PeriodStart:  start,
				PeriodEnd:    end,
				Currency:     t.currency,
				ByCategory:   make(map[CostCategory]float64),
			}
		}

		features[key].TotalCost += e.Cost
		features[key].ByCategory[e.Category] += e.Cost
	}

	return features
}

// GetTenantCosts returns costs grouped by tenant.
func (t *Tracker) GetTenantCosts(start, end time.Time) map[string]*CostSummary {
	entries := t.GetCosts("", start, end)

	tenants := make(map[string]*CostSummary)
	for _, e := range entries {
		key := e.TenantID
		if key == "" {
			key = "_shared"
		}

		if tenants[key] == nil {
			tenants[key] = &CostSummary{
				TenantID:    key,
				PeriodStart: start,
				PeriodEnd:   end,
				Currency:    t.currency,
				ByCategory:  make(map[CostCategory]float64),
				ByFeature:   make(map[string]float64),
			}
		}

		tenants[key].TotalCost += e.Cost
		tenants[key].ByCategory[e.Category] += e.Cost
		if e.FeatureGroup != "" {
			tenants[key].ByFeature[e.FeatureGroup] += e.Cost
		}
	}

	return tenants
}

// GetTimeSeries returns costs over time.
func (t *Tracker) GetTimeSeries(tenantID string, start, end time.Time, granularity string) []TimeSeriesPoint {
	entries := t.GetCosts(tenantID, start, end)

	// Determine interval
	var interval time.Duration
	switch granularity {
	case "hourly":
		interval = time.Hour
	case "daily":
		interval = 24 * time.Hour
	case "weekly":
		interval = 7 * 24 * time.Hour
	default:
		interval = 24 * time.Hour
	}

	// Group by interval
	buckets := make(map[int64]*TimeSeriesPoint)
	for _, e := range entries {
		bucketTime := e.Timestamp.Truncate(interval)
		bucketKey := bucketTime.Unix()

		if buckets[bucketKey] == nil {
			buckets[bucketKey] = &TimeSeriesPoint{
				Timestamp:  bucketTime,
				ByCategory: make(map[CostCategory]float64),
			}
		}
		buckets[bucketKey].Cost += e.Cost
		buckets[bucketKey].ByCategory[e.Category] += e.Cost
	}

	// Convert to sorted slice
	var points []TimeSeriesPoint
	for _, p := range buckets {
		points = append(points, *p)
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	return points
}

// CalculateTrends calculates cost trends.
func (t *Tracker) CalculateTrends(tenantID string, start, end time.Time) *CostTrends {
	points := t.GetTimeSeries(tenantID, start, end, "daily")

	if len(points) == 0 {
		return nil
	}

	trends := &CostTrends{}

	// Calculate totals and find highs/lows
	var totalCost float64
	trends.HighestDayCost = points[0].Cost
	trends.HighestDay = points[0].Timestamp
	trends.LowestDayCost = points[0].Cost
	trends.LowestDay = points[0].Timestamp

	for _, p := range points {
		totalCost += p.Cost
		if p.Cost > trends.HighestDayCost {
			trends.HighestDayCost = p.Cost
			trends.HighestDay = p.Timestamp
		}
		if p.Cost < trends.LowestDayCost {
			trends.LowestDayCost = p.Cost
			trends.LowestDay = p.Timestamp
		}
	}

	// Calculate averages
	days := float64(len(points))
	trends.AverageDailyCost = totalCost / days
	trends.ProjectedMonthly = trends.AverageDailyCost * 30

	// Calculate period over period change
	if len(points) >= 2 {
		mid := len(points) / 2
		var firstHalf, secondHalf float64
		for i, p := range points {
			if i < mid {
				firstHalf += p.Cost
			} else {
				secondHalf += p.Cost
			}
		}
		if firstHalf > 0 {
			trends.PeriodOverPeriodChange = ((secondHalf - firstHalf) / firstHalf) * 100
		}
	}

	return trends
}

// RecordCount returns the number of stored records.
func (t *Tracker) RecordCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.usageRecords)
}

// SetMaxRecords sets the maximum number of records to retain.
func (t *Tracker) SetMaxRecords(max int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maxRecords = max
}
