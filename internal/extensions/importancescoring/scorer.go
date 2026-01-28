package importancescoring

import (
	"math"
	"sort"
	"sync"
	"time"
)

// FeatureScore holds the computed importance of a single feature.
type FeatureScore struct {
	Name             string
	ImportanceScore  float64
	AccessFrequency  float64
	ValueVariance    float64
	CorrelationScore float64
	LastScored       time.Time
	Recommendation   string
}

// ScorerConfig configures the importance scorer.
type ScorerConfig struct {
	MinSamples             int
	ScoringInterval        time.Duration
	LowImportanceThreshold float64
	MaxTracked             int
}

// DefaultScorerConfig returns sensible defaults.
func DefaultScorerConfig() ScorerConfig {
	return ScorerConfig{
		MinSamples:             100,
		ScoringInterval:        1 * time.Hour,
		LowImportanceThreshold: 0.1,
		MaxTracked:             100000,
	}
}

// featureData tracks raw observations for a single feature.
type featureData struct {
	values      []float64
	accessCount int64
	lastAccess  time.Time
}

// ScorerStats holds aggregate scorer statistics.
type ScorerStats struct {
	TotalFeatures         int
	ScoredFeatures        int
	DeprecationCandidates int
	AvgImportance         float64
}

// Scorer computes feature importance from access patterns and value distributions.
type Scorer struct {
	mu       sync.RWMutex
	config   ScorerConfig
	features map[string]*featureData
	scores   map[string]*FeatureScore
}

// NewScorer creates a new Scorer with the given configuration.
func NewScorer(config ScorerConfig) *Scorer {
	return &Scorer{
		config:   config,
		features: make(map[string]*featureData),
		scores:   make(map[string]*FeatureScore),
	}
}

// RecordAccess records a feature access with its current value.
func (s *Scorer) RecordAccess(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fd, ok := s.features[name]
	if !ok {
		fd = &featureData{}
		s.features[name] = fd
	}
	fd.accessCount++
	fd.lastAccess = time.Now()

	// Ring buffer: keep at most MinSamples values
	if len(fd.values) >= s.config.MinSamples {
		fd.values = fd.values[1:]
	}
	fd.values = append(fd.values, value)
}

// ScoreAll computes importance for all tracked features.
func (s *Scorer) ScoreAll() []FeatureScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.features) == 0 {
		return nil
	}

	// Collect raw metrics
	type rawMetric struct {
		name       string
		accessFreq float64
		variance   float64
	}
	var metrics []rawMetric
	for name, fd := range s.features {
		metrics = append(metrics, rawMetric{
			name:       name,
			accessFreq: float64(fd.accessCount),
			variance:   computeVariance(fd.values),
		})
	}

	// Normalize access frequency and variance to [0, 1]
	maxAccess := 0.0
	maxVar := 0.0
	for _, m := range metrics {
		if m.accessFreq > maxAccess {
			maxAccess = m.accessFreq
		}
		if m.variance > maxVar {
			maxVar = m.variance
		}
	}

	now := time.Now()
	var results []FeatureScore
	for _, m := range metrics {
		normAccess := 0.0
		if maxAccess > 0 {
			normAccess = m.accessFreq / maxAccess
		}
		normVar := 0.0
		if maxVar > 0 {
			normVar = m.variance / maxVar
		}
		// Correlation score is approximated as variance contribution
		normCorr := normVar * 0.5

		importance := 0.4*normAccess + 0.3*normVar + 0.3*normCorr

		rec := "keep"
		if importance < s.config.LowImportanceThreshold {
			rec = "consider deprecation"
		}

		score := FeatureScore{
			Name:             m.name,
			ImportanceScore:  importance,
			AccessFrequency:  normAccess,
			ValueVariance:    normVar,
			CorrelationScore: normCorr,
			LastScored:       now,
			Recommendation:   rec,
		}
		s.scores[m.name] = &score
		results = append(results, score)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ImportanceScore > results[j].ImportanceScore
	})
	return results
}

// GetScore returns the score for a single feature.
func (s *Scorer) GetScore(name string) (*FeatureScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	score, ok := s.scores[name]
	if !ok {
		return nil, ErrFeatureNotScored
	}
	cp := *score
	return &cp, nil
}

// GetTopK returns the top K features by importance score.
func (s *Scorer) GetTopK(k int) []FeatureScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.sortedScoresDesc()
	if k > len(all) {
		k = len(all)
	}
	return all[:k]
}

// GetBottomK returns the bottom K features by importance score.
func (s *Scorer) GetBottomK(k int) []FeatureScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.sortedScoresDesc()
	sort.Slice(all, func(i, j int) bool {
		return all[i].ImportanceScore < all[j].ImportanceScore
	})
	if k > len(all) {
		k = len(all)
	}
	return all[:k]
}

// GetDeprecationCandidates returns features below the low importance threshold.
func (s *Scorer) GetDeprecationCandidates() []FeatureScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var candidates []FeatureScore
	for _, score := range s.scores {
		if score.ImportanceScore < s.config.LowImportanceThreshold {
			candidates = append(candidates, *score)
		}
	}
	return candidates
}

// Stats returns aggregate scorer statistics.
func (s *Scorer) Stats() ScorerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	depCandidates := 0
	totalImportance := 0.0
	for _, score := range s.scores {
		totalImportance += score.ImportanceScore
		if score.ImportanceScore < s.config.LowImportanceThreshold {
			depCandidates++
		}
	}

	avg := 0.0
	if len(s.scores) > 0 {
		avg = totalImportance / float64(len(s.scores))
	}

	return ScorerStats{
		TotalFeatures:         len(s.features),
		ScoredFeatures:        len(s.scores),
		DeprecationCandidates: depCandidates,
		AvgImportance:         avg,
	}
}

// sortedScoresDesc returns all scores sorted by importance descending.
func (s *Scorer) sortedScoresDesc() []FeatureScore {
	out := make([]FeatureScore, 0, len(s.scores))
	for _, score := range s.scores {
		out = append(out, *score)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ImportanceScore > out[j].ImportanceScore
	})
	return out
}

// computeVariance returns the population variance of a slice of float64.
func computeVariance(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	sumSq := 0.0
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(values)))
}
