package finops

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// FeatureCost tracks cost attribution for an individual feature.
type FeatureCost struct {
	FeatureName  string  `json:"feature_name"`
	GroupName    string  `json:"group_name"`
	Team         string  `json:"team"`
	StorageCost  float64 `json:"storage_cost"`
	ComputeCost  float64 `json:"compute_cost"`
	ServingCost  float64 `json:"serving_cost"`
	TotalCost    float64 `json:"total_cost"`
	ReadCount    int64   `json:"read_count"`
	WriteCount   int64   `json:"write_count"`
	LastAccessed time.Time `json:"last_accessed"`
	CostPerRead  float64 `json:"cost_per_read"`
	Period       string  `json:"period"`
}

// PredictionCost links cost to model predictions.
type PredictionCost struct {
	ModelID      string  `json:"model_id"`
	FeatureCount int     `json:"feature_count"`
	CostPerQuery float64 `json:"cost_per_query"`
	TotalQueries int64   `json:"total_queries"`
	TotalCost    float64 `json:"total_cost"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Period       string  `json:"period"`
}

// CostTrend represents cost trend over time.
type CostTrend struct {
	Feature    string  `json:"feature"`
	Date       string  `json:"date"`
	DailyCost  float64 `json:"daily_cost"`
	CumulCost  float64 `json:"cumulative_cost"`
	ReadCount  int64   `json:"read_count"`
	WriteCount int64   `json:"write_count"`
}

// OptimizationSuggestion is an intelligent cost-saving recommendation.
type OptimizationSuggestion struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Feature     string  `json:"feature"`
	Description string  `json:"description"`
	CurrentCost float64 `json:"current_cost"`
	EstSavings  float64 `json:"estimated_savings"`
	SavingsPct  float64 `json:"savings_pct"`
	Priority    string  `json:"priority"`
	Action      string  `json:"action"`
	Reason      string  `json:"reason"`
}

// CostSummary provides an overview of all tracked costs.
type CostSummary struct {
	TotalFeatures    int     `json:"total_features"`
	TotalModels      int     `json:"total_models"`
	TotalStorageCost float64 `json:"total_storage_cost"`
	TotalComputeCost float64 `json:"total_compute_cost"`
	TotalServingCost float64 `json:"total_serving_cost"`
	GrandTotal       float64 `json:"grand_total"`
	PotentialSavings float64 `json:"potential_savings"`
	SuggestionCount  int     `json:"suggestion_count"`
}

// CostAttributor tracks per-feature and per-prediction costs.
type CostAttributor struct {
	mu                sync.RWMutex
	featureCosts      map[string]*FeatureCost
	predictionCosts   map[string]*PredictionCost
	trends            []CostTrend
	storageCostPerMB  float64
	computeCostPerOp  float64
	servingCostPerReq float64
}

// NewCostAttributor creates a new CostAttributor with default rates.
func NewCostAttributor() *CostAttributor {
	return &CostAttributor{
		featureCosts:      make(map[string]*FeatureCost),
		predictionCosts:   make(map[string]*PredictionCost),
		trends:            make([]CostTrend, 0),
		storageCostPerMB:  0.023,
		computeCostPerOp:  0.0001,
		servingCostPerReq: 0.0005,
	}
}

// RecordFeatureRead records a read operation for a feature.
func (ca *CostAttributor) RecordFeatureRead(featureName, groupName, team string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	fc, ok := ca.featureCosts[featureName]
	if !ok {
		fc = &FeatureCost{
			FeatureName: featureName,
			GroupName:   groupName,
			Team:        team,
			Period:      "monthly",
		}
		ca.featureCosts[featureName] = fc
	}

	fc.ReadCount++
	fc.ServingCost += ca.servingCostPerReq
	fc.TotalCost = fc.StorageCost + fc.ComputeCost + fc.ServingCost
	fc.LastAccessed = time.Now()
	if fc.ReadCount > 0 {
		fc.CostPerRead = fc.TotalCost / float64(fc.ReadCount)
	}
}

// RecordFeatureWrite records a write operation for a feature.
func (ca *CostAttributor) RecordFeatureWrite(featureName, groupName, team string) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	fc, ok := ca.featureCosts[featureName]
	if !ok {
		fc = &FeatureCost{
			FeatureName: featureName,
			GroupName:   groupName,
			Team:        team,
			Period:      "monthly",
		}
		ca.featureCosts[featureName] = fc
	}

	fc.WriteCount++
	fc.ComputeCost += ca.computeCostPerOp
	fc.TotalCost = fc.StorageCost + fc.ComputeCost + fc.ServingCost
	fc.LastAccessed = time.Now()
	if fc.ReadCount > 0 {
		fc.CostPerRead = fc.TotalCost / float64(fc.ReadCount)
	}
}

// RecordPrediction records a model prediction event.
func (ca *CostAttributor) RecordPrediction(modelID string, featureCount int, latencyMs float64) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	pc, ok := ca.predictionCosts[modelID]
	if !ok {
		pc = &PredictionCost{
			ModelID: modelID,
			Period:  "monthly",
		}
		ca.predictionCosts[modelID] = pc
	}

	pc.TotalQueries++
	pc.FeatureCount = featureCount
	queryCost := float64(featureCount) * ca.servingCostPerReq
	pc.TotalCost += queryCost
	pc.CostPerQuery = pc.TotalCost / float64(pc.TotalQueries)
	// Running average for latency
	pc.AvgLatencyMs = pc.AvgLatencyMs + (latencyMs-pc.AvgLatencyMs)/float64(pc.TotalQueries)
}

// GetFeatureCost returns cost data for a specific feature.
func (ca *CostAttributor) GetFeatureCost(featureName string) (*FeatureCost, bool) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	fc, ok := ca.featureCosts[featureName]
	return fc, ok
}

// GetAllFeatureCosts returns cost data for all tracked features.
func (ca *CostAttributor) GetAllFeatureCosts() []*FeatureCost {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	result := make([]*FeatureCost, 0, len(ca.featureCosts))
	for _, fc := range ca.featureCosts {
		result = append(result, fc)
	}
	return result
}

// GetPredictionCost returns cost data for a specific model.
func (ca *CostAttributor) GetPredictionCost(modelID string) (*PredictionCost, bool) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	pc, ok := ca.predictionCosts[modelID]
	return pc, ok
}

// GetAllPredictionCosts returns cost data for all tracked models.
func (ca *CostAttributor) GetAllPredictionCosts() []*PredictionCost {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	result := make([]*PredictionCost, 0, len(ca.predictionCosts))
	for _, pc := range ca.predictionCosts {
		result = append(result, pc)
	}
	return result
}

// GenerateOptimizations analyzes feature usage and returns cost-saving suggestions.
func (ca *CostAttributor) GenerateOptimizations() []*OptimizationSuggestion {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	var suggestions []*OptimizationSuggestion
	seq := 0

	for _, fc := range ca.featureCosts {
		// 1. Deprecation: feature not accessed in 30+ days
		if !fc.LastAccessed.IsZero() && time.Since(fc.LastAccessed) > 30*24*time.Hour {
			seq++
			savings := fc.TotalCost * 0.9
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", seq),
				Type:        "deprecation",
				Feature:     fc.FeatureName,
				Description: fmt.Sprintf("Feature %q has not been accessed in 30+ days", fc.FeatureName),
				CurrentCost: fc.TotalCost,
				EstSavings:  savings,
				SavingsPct:  90.0,
				Priority:    "high",
				Action:      "Consider deprecating or archiving this feature",
				Reason:      "No recent access indicates the feature may no longer be needed",
			})
		}

		// 2. Caching: high read count features benefit from caching
		if fc.ReadCount > 1000 {
			seq++
			savings := fc.ServingCost * 0.5
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", seq),
				Type:        "caching",
				Feature:     fc.FeatureName,
				Description: fmt.Sprintf("Feature %q has high read volume (%d reads)", fc.FeatureName, fc.ReadCount),
				CurrentCost: fc.TotalCost,
				EstSavings:  savings,
				SavingsPct:  safePct(savings, fc.TotalCost),
				Priority:    "medium",
				Action:      "Enable aggressive caching for this feature",
				Reason:      "High read volume features benefit significantly from caching",
			})
		}

		// 3. Tier migration: high cost but low reads
		if fc.TotalCost > 10.0 && fc.ReadCount < 100 {
			seq++
			savings := fc.TotalCost * 0.6
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", seq),
				Type:        "tier_migration",
				Feature:     fc.FeatureName,
				Description: fmt.Sprintf("Feature %q costs $%.2f/month but has only %d reads", fc.FeatureName, fc.TotalCost, fc.ReadCount),
				CurrentCost: fc.TotalCost,
				EstSavings:  savings,
				SavingsPct:  60.0,
				Priority:    "high",
				Action:      "Move this feature to warm tier storage",
				Reason:      "Low read count does not justify hot tier costs",
			})
		}
	}

	// Sort by estimated savings descending
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].EstSavings > suggestions[j].EstSavings
	})

	return suggestions
}

// SetRates updates the cost rates used for attribution.
func (ca *CostAttributor) SetRates(storageCostPerMB, computeCostPerOp, servingCostPerReq float64) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	ca.storageCostPerMB = storageCostPerMB
	ca.computeCostPerOp = computeCostPerOp
	ca.servingCostPerReq = servingCostPerReq
}

// GetCostSummary returns an overview of all tracked costs.
func (ca *CostAttributor) GetCostSummary() *CostSummary {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	summary := &CostSummary{
		TotalFeatures: len(ca.featureCosts),
		TotalModels:   len(ca.predictionCosts),
	}

	for _, fc := range ca.featureCosts {
		summary.TotalStorageCost += fc.StorageCost
		summary.TotalComputeCost += fc.ComputeCost
		summary.TotalServingCost += fc.ServingCost
		summary.GrandTotal += fc.TotalCost
	}

	// Calculate potential savings from optimizations (without holding write lock)
	optimizations := ca.generateOptimizationsLocked()
	for _, opt := range optimizations {
		summary.PotentialSavings += opt.EstSavings
	}
	summary.SuggestionCount = len(optimizations)

	return summary
}

// generateOptimizationsLocked is the internal version that assumes the read lock is held.
func (ca *CostAttributor) generateOptimizationsLocked() []*OptimizationSuggestion {
	var suggestions []*OptimizationSuggestion
	seq := 0

	for _, fc := range ca.featureCosts {
		if !fc.LastAccessed.IsZero() && time.Since(fc.LastAccessed) > 30*24*time.Hour {
			seq++
			savings := fc.TotalCost * 0.9
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", seq),
				Type:        "deprecation",
				Feature:     fc.FeatureName,
				CurrentCost: fc.TotalCost,
				EstSavings:  savings,
			})
		}

		if fc.ReadCount > 1000 {
			seq++
			savings := fc.ServingCost * 0.5
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", seq),
				Type:        "caching",
				Feature:     fc.FeatureName,
				CurrentCost: fc.TotalCost,
				EstSavings:  savings,
			})
		}

		if fc.TotalCost > 10.0 && fc.ReadCount < 100 {
			seq++
			savings := fc.TotalCost * 0.6
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", seq),
				Type:        "tier_migration",
				Feature:     fc.FeatureName,
				CurrentCost: fc.TotalCost,
				EstSavings:  savings,
			})
		}
	}

	return suggestions
}

func safePct(savings, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (savings / total) * 100
}
