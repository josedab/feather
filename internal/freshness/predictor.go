package freshness

import (
	"sync"
	"time"
)

// Prediction represents a freshness TTL prediction for a feature.
type Prediction struct {
	FeatureName     string        `json:"feature_name"`
	RecommendedTTL  time.Duration `json:"recommended_ttl"`
	Confidence      float64       `json:"confidence"`       // 0-1 confidence score
	Reason          string        `json:"reason"`           // Human-readable explanation
	AccessScore     float64       `json:"access_score"`     // Contribution from access patterns
	VolatilityScore float64       `json:"volatility_score"` // Contribution from volatility
	DriftScore      float64       `json:"drift_score"`      // Contribution from drift
	PredictedAt     time.Time     `json:"predicted_at"`
}

// PredictorConfig configures the freshness predictor.
type PredictorConfig struct {
	MinTTL           time.Duration // Minimum allowed TTL
	MaxTTL           time.Duration // Maximum allowed TTL
	DefaultTTL       time.Duration // Default TTL when no data available
	AccessWeight     float64       // Weight for access pattern score (0-1)
	VolatilityWeight float64       // Weight for volatility score (0-1)
	DriftWeight      float64       // Weight for drift score (0-1)
	UpdateInterval   time.Duration // How often to update predictions
}

// DefaultPredictorConfig returns sensible defaults.
func DefaultPredictorConfig() PredictorConfig {
	return PredictorConfig{
		MinTTL:           1 * time.Second,
		MaxTTL:           24 * time.Hour,
		DefaultTTL:       5 * time.Minute,
		AccessWeight:     0.3,
		VolatilityWeight: 0.4,
		DriftWeight:      0.3,
		UpdateInterval:   1 * time.Minute,
	}
}

// Predictor predicts optimal TTL values based on feature metrics.
type Predictor struct {
	config      PredictorConfig
	monitor     *Monitor
	predictions map[string]*Prediction
	mu          sync.RWMutex
	stopCh      chan struct{}
}

// NewPredictor creates a new freshness predictor.
func NewPredictor(config PredictorConfig, monitor *Monitor) *Predictor {
	p := &Predictor{
		config:      config,
		monitor:     monitor,
		predictions: make(map[string]*Prediction),
		stopCh:      make(chan struct{}),
	}

	// Start prediction update loop
	go p.updateLoop()

	return p
}

// Predict returns the predicted optimal TTL for a feature.
func (p *Predictor) Predict(featureName string) *Prediction {
	p.mu.RLock()
	prediction, exists := p.predictions[featureName]
	p.mu.RUnlock()

	if exists {
		return prediction
	}

	// Generate new prediction
	return p.generatePrediction(featureName)
}

// GetAllPredictions returns predictions for all tracked features.
func (p *Predictor) GetAllPredictions() []*Prediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*Prediction, 0, len(p.predictions))
	for _, pred := range p.predictions {
		result = append(result, pred)
	}
	return result
}

func (p *Predictor) generatePrediction(featureName string) *Prediction {
	accessMetrics, hasAccess := p.monitor.GetAccessMetrics(featureName)
	changeMetrics, hasChange := p.monitor.GetChangeMetrics(featureName)

	// Calculate component scores
	accessScore := p.calculateAccessScore(accessMetrics, hasAccess)
	volatilityScore := p.calculateVolatilityScore(changeMetrics, hasChange)
	driftScore := p.calculateDriftScore(changeMetrics, hasChange)

	// Weighted combination
	totalWeight := p.config.AccessWeight + p.config.VolatilityWeight + p.config.DriftWeight
	normalizedAccess := p.config.AccessWeight / totalWeight
	normalizedVolatility := p.config.VolatilityWeight / totalWeight
	normalizedDrift := p.config.DriftWeight / totalWeight

	// Combined score (0 = need short TTL, 1 = can have long TTL)
	combinedScore := accessScore*normalizedAccess +
		(1-volatilityScore)*normalizedVolatility +
		(1-driftScore)*normalizedDrift

	// Map score to TTL range
	ttl := p.scoreToTTL(combinedScore)

	// Calculate confidence based on data availability
	confidence := p.calculateConfidence(hasAccess, hasChange, accessMetrics, changeMetrics)

	reason := p.generateReason(accessScore, volatilityScore, driftScore, ttl)

	prediction := &Prediction{
		FeatureName:     featureName,
		RecommendedTTL:  ttl,
		Confidence:      confidence,
		Reason:          reason,
		AccessScore:     accessScore,
		VolatilityScore: volatilityScore,
		DriftScore:      driftScore,
		PredictedAt:     time.Now(),
	}

	// Store prediction
	p.mu.Lock()
	p.predictions[featureName] = prediction
	p.mu.Unlock()

	return prediction
}

func (p *Predictor) calculateAccessScore(metrics *AccessMetrics, hasMetrics bool) float64 {
	if !hasMetrics || metrics == nil {
		return 0.5 // Neutral score when no data
	}

	// High access rate + high cache hit rate = can tolerate longer TTL
	// Low cache hit rate = might need shorter TTL for fresher data

	// Access rate score (normalize to 0-1)
	// High access rate (>100/s) gets high score
	rateScore := minFloat64(metrics.AccessRate/100.0, 1.0)

	// Cache hit rate directly contributes
	hitScore := metrics.CacheHitRate

	// Stale serves penalty
	staleRatio := 0.0
	if metrics.TotalAccesses > 0 {
		staleRatio = float64(metrics.StaleServes) / float64(metrics.TotalAccesses)
	}
	stalePenalty := staleRatio * 0.5 // Max 0.5 penalty

	return (rateScore*0.3 + hitScore*0.7) * (1 - stalePenalty)
}

func (p *Predictor) calculateVolatilityScore(metrics *ChangeMetrics, hasMetrics bool) float64 {
	if !hasMetrics || metrics == nil {
		return 0.0 // Low volatility assumed when no data
	}

	// High volatility = need shorter TTL
	// Normalize volatility to 0-1 scale

	// Update rate contribution
	// High update rate (>1/s) = high volatility
	updateScore := minFloat64(metrics.UpdateRate, 1.0)

	// Change magnitude contribution
	// Normalize avg change magnitude (assume 100 is "high")
	magnitudeScore := minFloat64(metrics.AvgChangeMagnitude/100.0, 1.0)

	// Raw volatility contribution
	volatilityScore := minFloat64(metrics.Volatility/50.0, 1.0)

	return (updateScore*0.4 + magnitudeScore*0.3 + volatilityScore*0.3)
}

func (p *Predictor) calculateDriftScore(metrics *ChangeMetrics, hasMetrics bool) float64 {
	if !hasMetrics || metrics == nil {
		return 0.0 // No drift assumed when no data
	}

	// Drift score is already 0-1
	return minFloat64(metrics.DriftScore, 1.0)
}

func (p *Predictor) scoreToTTL(score float64) time.Duration {
	// Score 0 = min TTL, Score 1 = max TTL
	score = maxFloat64(0, minFloat64(score, 1))

	minNs := float64(p.config.MinTTL.Nanoseconds())
	maxNs := float64(p.config.MaxTTL.Nanoseconds())

	// Logarithmic scaling for better distribution
	// Use exponential interpolation
	logMin := 1.0
	logMax := maxNs / minNs
	logTTL := logMin + score*(logMax-logMin)

	ttlNs := minNs * logTTL
	return time.Duration(ttlNs)
}

func (p *Predictor) calculateConfidence(hasAccess, hasChange bool, accessMetrics *AccessMetrics, changeMetrics *ChangeMetrics) float64 {
	confidence := 0.0

	// Data availability contributes to confidence
	if hasAccess {
		confidence += 0.3
		// More data = more confidence
		if accessMetrics != nil && accessMetrics.TotalAccesses > 100 {
			confidence += 0.2
		} else if accessMetrics != nil && accessMetrics.TotalAccesses > 10 {
			confidence += 0.1
		}
	}

	if hasChange {
		confidence += 0.3
		// More change data = more confidence
		if changeMetrics != nil && changeMetrics.TotalUpdates > 50 {
			confidence += 0.2
		} else if changeMetrics != nil && changeMetrics.TotalUpdates > 5 {
			confidence += 0.1
		}
	}

	// No data = low confidence, use default
	if !hasAccess && !hasChange {
		confidence = 0.1
	}

	return minFloat64(confidence, 1.0)
}

func (p *Predictor) generateReason(accessScore, volatilityScore, driftScore float64, ttl time.Duration) string {
	// Determine primary factors
	reasons := []string{}

	if volatilityScore > 0.7 {
		reasons = append(reasons, "high volatility")
	} else if volatilityScore < 0.3 {
		reasons = append(reasons, "stable values")
	}

	if driftScore > 0.7 {
		reasons = append(reasons, "significant drift detected")
	} else if driftScore < 0.2 {
		reasons = append(reasons, "no drift")
	}

	if accessScore > 0.7 {
		reasons = append(reasons, "high cache hit rate")
	} else if accessScore < 0.3 {
		reasons = append(reasons, "low cache efficiency")
	}

	if len(reasons) == 0 {
		return "balanced factors"
	}

	result := reasons[0]
	for i := 1; i < len(reasons); i++ {
		result += ", " + reasons[i]
	}

	return result
}

func (p *Predictor) updateLoop() {
	ticker := time.NewTicker(p.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.updateAllPredictions()
		}
	}
}

func (p *Predictor) updateAllPredictions() {
	// Get all tracked features
	accessMetrics := p.monitor.GetAllAccessMetrics()

	for _, metrics := range accessMetrics {
		p.generatePrediction(metrics.FeatureName)
	}
}

// Stop stops the predictor.
func (p *Predictor) Stop() {
	close(p.stopCh)
}

// PredictorStats returns statistics about the predictor.
type PredictorStats struct {
	TotalPredictions int           `json:"total_predictions"`
	AvgConfidence    float64       `json:"avg_confidence"`
	MinTTL           time.Duration `json:"min_ttl"`
	MaxTTL           time.Duration `json:"max_ttl"`
	AvgTTL           time.Duration `json:"avg_ttl"`
}

// Stats returns predictor statistics.
func (p *Predictor) Stats() PredictorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := PredictorStats{
		TotalPredictions: len(p.predictions),
	}

	if len(p.predictions) == 0 {
		stats.MinTTL = p.config.DefaultTTL
		stats.MaxTTL = p.config.DefaultTTL
		stats.AvgTTL = p.config.DefaultTTL
		return stats
	}

	var minTTL, maxTTL time.Duration
	var totalTTL time.Duration
	var totalConfidence float64
	first := true

	for _, pred := range p.predictions {
		totalConfidence += pred.Confidence
		totalTTL += pred.RecommendedTTL

		if first {
			minTTL = pred.RecommendedTTL
			maxTTL = pred.RecommendedTTL
			first = false
		} else {
			if pred.RecommendedTTL < minTTL {
				minTTL = pred.RecommendedTTL
			}
			if pred.RecommendedTTL > maxTTL {
				maxTTL = pred.RecommendedTTL
			}
		}
	}

	stats.AvgConfidence = totalConfidence / float64(len(p.predictions))
	stats.MinTTL = minTTL
	stats.MaxTTL = maxTTL
	stats.AvgTTL = totalTTL / time.Duration(len(p.predictions))

	return stats
}

// Helper functions
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
