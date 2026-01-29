package finops

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrTeamNotFound = errors.New("team not found")
	ErrTeamExists   = errors.New("team already exists")
	ErrNoUsageData  = errors.New("no usage data available")
)

// CostCategory identifies the type of resource cost.
type CostCategory string

const (
	CostStorage CostCategory = "storage"
	CostCompute CostCategory = "compute"
	CostNetwork CostCategory = "network"
	CostAPI     CostCategory = "api"
	CostML      CostCategory = "ml"
	CostVector  CostCategory = "vector"
)

// UsageRecord tracks a single usage event.
type UsageRecord struct {
	Timestamp    time.Time    `json:"timestamp"`
	Team         string       `json:"team"`
	FeatureGroup string       `json:"feature_group"`
	Category     CostCategory `json:"category"`
	Quantity     float64      `json:"quantity"`
	Unit         string       `json:"unit"`
	Model        string       `json:"model,omitempty"`
}

// CostRate defines the price per unit for a cost category.
type CostRate struct {
	Category CostCategory `json:"category"`
	Unit     string       `json:"unit"`
	Rate     float64      `json:"rate_per_unit"`
	Currency string       `json:"currency"`
}

// TeamCost aggregates cost for a team over a period.
type TeamCost struct {
	Team       string                   `json:"team"`
	Period     string                   `json:"period"`
	Total      float64                  `json:"total"`
	Currency   string                   `json:"currency"`
	ByCategory map[CostCategory]float64 `json:"by_category"`
	ByGroup    map[string]float64       `json:"by_group"`
}

// FeatureGroupCost aggregates cost for a feature group.
type FeatureGroupCost struct {
	FeatureGroup string                   `json:"feature_group"`
	Total        float64                  `json:"total"`
	Currency     string                   `json:"currency"`
	ByCategory   map[CostCategory]float64 `json:"by_category"`
	ByTeam       map[string]float64       `json:"by_team"`
}

// CostPrediction forecasts a team's future cost.
type CostPrediction struct {
	Team          string  `json:"team"`
	Period        string  `json:"period"`
	EstimatedCost float64 `json:"estimated_cost"`
	Confidence    float64 `json:"confidence"`
	Currency      string  `json:"currency"`
}

// Recommendation suggests a cost optimization action.
type Recommendation struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Description  string  `json:"description"`
	FeatureGroup string  `json:"feature_group,omitempty"`
	Team         string  `json:"team,omitempty"`
	CurrentCost  float64 `json:"current_cost"`
	EstSavings   float64 `json:"estimated_savings"`
	Priority     string  `json:"priority"`
}

// Team represents a cost attribution team.
type Team struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Groups   []string `json:"feature_groups"`
	Budget   float64  `json:"budget,omitempty"`
	Currency string   `json:"currency"`
}

// ManagerConfig configures the FinOps manager.
type ManagerConfig struct {
	DefaultCurrency string
	DefaultRates    map[CostCategory]*CostRate
	RetentionDays   int
	MaxRecords      int
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		DefaultCurrency: "USD",
		RetentionDays:   90,
		MaxRecords:      1000000,
		DefaultRates: map[CostCategory]*CostRate{
			CostStorage: {Category: CostStorage, Unit: "GB-month", Rate: 0.023, Currency: "USD"},
			CostCompute: {Category: CostCompute, Unit: "CPU-hour", Rate: 0.0464, Currency: "USD"},
			CostNetwork: {Category: CostNetwork, Unit: "GB", Rate: 0.09, Currency: "USD"},
			CostAPI:     {Category: CostAPI, Unit: "1K-requests", Rate: 0.004, Currency: "USD"},
			CostML:      {Category: CostML, Unit: "1K-inferences", Rate: 0.01, Currency: "USD"},
			CostVector:  {Category: CostVector, Unit: "1K-searches", Rate: 0.005, Currency: "USD"},
		},
	}
}

// Manager tracks costs, attribution, and generates recommendations.
type Manager struct {
	mu      sync.RWMutex
	teams   map[string]*Team
	records []UsageRecord
	rates   map[CostCategory]*CostRate
	config  ManagerConfig
}

// NewManager creates a new FinOps manager.
func NewManager(config ManagerConfig) *Manager {
	if config.DefaultCurrency == "" {
		config = DefaultManagerConfig()
	}
	return &Manager{
		teams:   make(map[string]*Team),
		records: make([]UsageRecord, 0),
		rates:   config.DefaultRates,
		config:  config,
	}
}

// RegisterTeam adds a team for cost tracking.
func (m *Manager) RegisterTeam(team *Team) error {
	if team.ID == "" {
		return fmt.Errorf("team ID is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.teams[team.ID]; exists {
		return ErrTeamExists
	}

	if team.Currency == "" {
		team.Currency = m.config.DefaultCurrency
	}
	m.teams[team.ID] = team
	return nil
}

// GetTeam retrieves a team by ID.
func (m *Manager) GetTeam(id string) (*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, ok := m.teams[id]
	if !ok {
		return nil, ErrTeamNotFound
	}
	return team, nil
}

// ListTeams returns all registered teams.
func (m *Manager) ListTeams() []*Team {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Team, 0, len(m.teams))
	for _, t := range m.teams {
		result = append(result, t)
	}
	return result
}

// RecordUsage tracks a usage event.
func (m *Manager) RecordUsage(record UsageRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	m.records = append(m.records, record)

	if len(m.records) > m.config.MaxRecords {
		m.records = m.records[len(m.records)-m.config.MaxRecords:]
	}
}

// GetTeamCost calculates cost for a team over a time period.
func (m *Manager) GetTeamCost(teamID string, since, until time.Time) (*TeamCost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.teams[teamID]; !ok {
		return nil, ErrTeamNotFound
	}

	tc := &TeamCost{
		Team:       teamID,
		Period:     fmt.Sprintf("%s to %s", since.Format("2006-01-02"), until.Format("2006-01-02")),
		Currency:   m.config.DefaultCurrency,
		ByCategory: make(map[CostCategory]float64),
		ByGroup:    make(map[string]float64),
	}

	for _, r := range m.records {
		if r.Team != teamID || r.Timestamp.Before(since) || r.Timestamp.After(until) {
			continue
		}

		cost := m.calculateCost(r)
		tc.Total += cost
		tc.ByCategory[r.Category] += cost
		if r.FeatureGroup != "" {
			tc.ByGroup[r.FeatureGroup] += cost
		}
	}

	return tc, nil
}

// GetFeatureGroupCost calculates cost for a feature group.
func (m *Manager) GetFeatureGroupCost(group string, since, until time.Time) *FeatureGroupCost {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fgc := &FeatureGroupCost{
		FeatureGroup: group,
		Currency:     m.config.DefaultCurrency,
		ByCategory:   make(map[CostCategory]float64),
		ByTeam:       make(map[string]float64),
	}

	for _, r := range m.records {
		if r.FeatureGroup != group || r.Timestamp.Before(since) || r.Timestamp.After(until) {
			continue
		}

		cost := m.calculateCost(r)
		fgc.Total += cost
		fgc.ByCategory[r.Category] += cost
		if r.Team != "" {
			fgc.ByTeam[r.Team] += cost
		}
	}

	return fgc
}

// GetRecommendations analyzes usage and suggests cost optimizations.
func (m *Manager) GetRecommendations() []Recommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var recs []Recommendation
	recSeq := 0

	// Analyze per-group costs
	groupCosts := make(map[string]float64)
	groupLastUsed := make(map[string]time.Time)

	for _, r := range m.records {
		cost := m.calculateCost(r)
		groupCosts[r.FeatureGroup] += cost
		if r.Timestamp.After(groupLastUsed[r.FeatureGroup]) {
			groupLastUsed[r.FeatureGroup] = r.Timestamp
		}
	}

	// Recommend archiving unused feature groups
	threshold := time.Now().Add(-7 * 24 * time.Hour)
	for group, lastUsed := range groupLastUsed {
		if lastUsed.Before(threshold) && groupCosts[group] > 0 {
			recSeq++
			recs = append(recs, Recommendation{
				ID:           fmt.Sprintf("rec-%d", recSeq),
				Type:         "archive_unused",
				Description:  fmt.Sprintf("Feature group %q hasn't been used in 7+ days. Consider archiving.", group),
				FeatureGroup: group,
				CurrentCost:  groupCosts[group],
				EstSavings:   groupCosts[group] * 0.8,
				Priority:     "medium",
			})
		}
	}

	// Recommend tier migration for high-cost storage groups
	for group, cost := range groupCosts {
		if cost > 100 {
			recSeq++
			recs = append(recs, Recommendation{
				ID:           fmt.Sprintf("rec-%d", recSeq),
				Type:         "tier_optimization",
				Description:  fmt.Sprintf("Feature group %q has high costs ($%.2f). Consider moving cold features to warm tier.", group, cost),
				FeatureGroup: group,
				CurrentCost:  cost,
				EstSavings:   cost * 0.3,
				Priority:     "high",
			})
		}
	}

	// Budget warnings
	for _, team := range m.teams {
		if team.Budget > 0 {
			var teamTotal float64
			for _, r := range m.records {
				if r.Team == team.ID {
					teamTotal += m.calculateCost(r)
				}
			}
			if teamTotal > team.Budget*0.8 {
				recSeq++
				recs = append(recs, Recommendation{
					ID:          fmt.Sprintf("rec-%d", recSeq),
					Type:        "budget_warning",
					Description: fmt.Sprintf("Team %q has used %.0f%% of budget ($%.2f/$%.2f)", team.Name, teamTotal/team.Budget*100, teamTotal, team.Budget),
					Team:        team.ID,
					CurrentCost: teamTotal,
					Priority:    "critical",
				})
			}
		}
	}

	return recs
}

// PredictCost estimates future cost for a team based on recent trends.
func (m *Manager) PredictCost(teamID string, days int) (*CostPrediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.teams[teamID]; !ok {
		return nil, ErrTeamNotFound
	}

	// Calculate daily average cost from last 30 days
	since := time.Now().Add(-30 * 24 * time.Hour)
	var total float64
	var recordDays int

	dailyCosts := make(map[string]float64)
	for _, r := range m.records {
		if r.Team != teamID || r.Timestamp.Before(since) {
			continue
		}
		day := r.Timestamp.Format("2006-01-02")
		dailyCosts[day] += m.calculateCost(r)
	}

	for _, cost := range dailyCosts {
		total += cost
		recordDays++
	}

	if recordDays == 0 {
		return nil, ErrNoUsageData
	}

	dailyAvg := total / float64(recordDays)

	return &CostPrediction{
		Team:          teamID,
		Period:        fmt.Sprintf("next %d days", days),
		EstimatedCost: dailyAvg * float64(days),
		Confidence:    0.75,
		Currency:      m.config.DefaultCurrency,
	}, nil
}

// Summary returns overall cost dashboard data.
func (m *Manager) Summary(since time.Time) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var total float64
	byCategory := make(map[CostCategory]float64)
	byTeam := make(map[string]float64)

	for _, r := range m.records {
		if r.Timestamp.Before(since) {
			continue
		}
		cost := m.calculateCost(r)
		total += cost
		byCategory[r.Category] += cost
		byTeam[r.Team] += cost
	}

	return map[string]interface{}{
		"total_cost":    total,
		"currency":      m.config.DefaultCurrency,
		"by_category":   byCategory,
		"by_team":       byTeam,
		"total_teams":   len(m.teams),
		"total_records": len(m.records),
	}
}

// SetRate updates the cost rate for a category.
func (m *Manager) SetRate(rate *CostRate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rates[rate.Category] = rate
}

// GetRates returns all configured rates.
func (m *Manager) GetRates() map[CostCategory]*CostRate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[CostCategory]*CostRate)
	for k, v := range m.rates {
		result[k] = v
	}
	return result
}

func (m *Manager) calculateCost(r UsageRecord) float64 {
	rate, ok := m.rates[r.Category]
	if !ok {
		return 0
	}
	return r.Quantity * rate.Rate
}
