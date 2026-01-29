package playground

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

var (
	ErrQueryNotFound   = errors.New("query not found")
	ErrInvalidQuery    = errors.New("invalid query")
	ErrDatasetNotFound = errors.New("dataset not found")
	ErrDatasetExists   = errors.New("dataset already exists")
)

// FeatureSummary provides a statistical overview of a feature.
type FeatureSummary struct {
	Name        string            `json:"name"`
	Group       string            `json:"group"`
	DataType    string            `json:"data_type"`
	Count       int64             `json:"count"`
	NullCount   int64             `json:"null_count"`
	NullRate    float64           `json:"null_rate"`
	Mean        float64           `json:"mean,omitempty"`
	StdDev      float64           `json:"std_dev,omitempty"`
	Min         float64           `json:"min,omitempty"`
	Max         float64           `json:"max,omitempty"`
	P25         float64           `json:"p25,omitempty"`
	P50         float64           `json:"p50,omitempty"`
	P75         float64           `json:"p75,omitempty"`
	TopValues   []ValueCount      `json:"top_values,omitempty"`
	Histogram   []HistogramBucket `json:"histogram,omitempty"`
	LastUpdated time.Time         `json:"last_updated"`
}

// ValueCount tracks a categorical value's frequency.
type ValueCount struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// HistogramBucket represents a bin in a histogram.
type HistogramBucket struct {
	LowerBound float64 `json:"lower_bound"`
	UpperBound float64 `json:"upper_bound"`
	Count      int64   `json:"count"`
}

// SavedQuery stores a user-defined feature query for reuse.
type SavedQuery struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Entities    []string  `json:"entities"`
	Features    []string  `json:"features"`
	Filters     []Filter  `json:"filters,omitempty"`
	AsOf        string    `json:"as_of,omitempty"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Filter defines a predicate on feature values.
type Filter struct {
	Feature  string      `json:"feature"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// DatasetConfig defines training dataset generation parameters.
type DatasetConfig struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Entities []string `json:"entities"`
	Features []string `json:"features"`
	AsOf     string   `json:"as_of,omitempty"`
	Format   string   `json:"format"` // csv, json, parquet
	Limit    int      `json:"limit,omitempty"`
}

// DatasetStatus tracks dataset generation progress.
type DatasetStatus struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // pending, generating, ready, failed
	RowCount  int64     `json:"row_count"`
	SizeBytes int64     `json:"size_bytes"`
	Format    string    `json:"format"`
	CreatedAt time.Time `json:"created_at"`
	ReadyAt   time.Time `json:"ready_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// FeatureProvider retrieves feature data for the playground.
type FeatureProvider interface {
	GetFeature(ctx context.Context, entity, feature string) (interface{}, time.Time, error)
	ListFeatures(ctx context.Context) ([]string, error)
	GetFeatureValues(ctx context.Context, feature string, limit int) ([]interface{}, error)
}

// Service provides the playground API.
type Service struct {
	mu       sync.RWMutex
	queries  map[string]*SavedQuery
	datasets map[string]*DatasetStatus
	provider FeatureProvider
	querySeq int64
}

// NewService creates a new playground service.
func NewService(provider FeatureProvider) *Service {
	return &Service{
		queries:  make(map[string]*SavedQuery),
		datasets: make(map[string]*DatasetStatus),
		provider: provider,
	}
}

// ComputeSummary generates statistical summary for a feature from sample values.
func (s *Service) ComputeSummary(name, group, dataType string, values []float64) *FeatureSummary {
	summary := &FeatureSummary{
		Name:        name,
		Group:       group,
		DataType:    dataType,
		Count:       int64(len(values)),
		LastUpdated: time.Now(),
	}

	if len(values) == 0 {
		return summary
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Basic stats
	var sum float64
	summary.Min = sorted[0]
	summary.Max = sorted[len(sorted)-1]
	for _, v := range sorted {
		sum += v
	}
	summary.Mean = sum / float64(len(sorted))

	var variance float64
	for _, v := range sorted {
		d := v - summary.Mean
		variance += d * d
	}
	summary.StdDev = math.Sqrt(variance / float64(len(sorted)))

	// Percentiles
	summary.P25 = pctile(sorted, 25)
	summary.P50 = pctile(sorted, 50)
	summary.P75 = pctile(sorted, 75)

	// Histogram (10 bins)
	summary.Histogram = computeHistogram(sorted, 10)

	return summary
}

// SaveQuery stores a query for reuse.
func (s *Service) SaveQuery(q *SavedQuery) error {
	if q.Name == "" || len(q.Features) == 0 {
		return ErrInvalidQuery
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.querySeq++
	if q.ID == "" {
		q.ID = fmt.Sprintf("q-%d", s.querySeq)
	}
	q.CreatedAt = time.Now()
	s.queries[q.ID] = q
	return nil
}

// GetQuery retrieves a saved query.
func (s *Service) GetQuery(id string) (*SavedQuery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q, exists := s.queries[id]
	if !exists {
		return nil, ErrQueryNotFound
	}
	return q, nil
}

// ListQueries returns all saved queries.
func (s *Service) ListQueries() []*SavedQuery {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*SavedQuery, 0, len(s.queries))
	for _, q := range s.queries {
		result = append(result, q)
	}
	return result
}

// DeleteQuery removes a saved query.
func (s *Service) DeleteQuery(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.queries[id]; !exists {
		return ErrQueryNotFound
	}
	delete(s.queries, id)
	return nil
}

// CreateDataset starts generation of a training dataset.
func (s *Service) CreateDataset(cfg *DatasetConfig) (*DatasetStatus, error) {
	if cfg.Name == "" || len(cfg.Features) == 0 {
		return nil, ErrInvalidQuery
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.datasets[cfg.ID]; exists {
		return nil, ErrDatasetExists
	}

	status := &DatasetStatus{
		ID:        cfg.ID,
		Name:      cfg.Name,
		Status:    "pending",
		Format:    cfg.Format,
		CreatedAt: time.Now(),
	}
	s.datasets[cfg.ID] = status

	// Simulate generation completing
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.mu.Lock()
		defer s.mu.Unlock()
		if ds, ok := s.datasets[cfg.ID]; ok {
			ds.Status = "ready"
			ds.RowCount = int64(len(cfg.Entities))
			ds.ReadyAt = time.Now()
		}
	}()

	return status, nil
}

// GetDatasetStatus returns the status of a dataset.
func (s *Service) GetDatasetStatus(id string) (*DatasetStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ds, exists := s.datasets[id]
	if !exists {
		return nil, ErrDatasetNotFound
	}
	return ds, nil
}

// ListDatasets returns all datasets.
func (s *Service) ListDatasets() []*DatasetStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*DatasetStatus, 0, len(s.datasets))
	for _, ds := range s.datasets {
		result = append(result, ds)
	}
	return result
}

func pctile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(p) / 100 * float64(len(sorted)-1))
	return sorted[idx]
}

func computeHistogram(sorted []float64, bins int) []HistogramBucket {
	if len(sorted) == 0 || bins <= 0 {
		return nil
	}

	min := sorted[0]
	max := sorted[len(sorted)-1]
	if min == max {
		return []HistogramBucket{{LowerBound: min, UpperBound: max, Count: int64(len(sorted))}}
	}

	width := (max - min) / float64(bins)
	buckets := make([]HistogramBucket, bins)
	for i := 0; i < bins; i++ {
		buckets[i] = HistogramBucket{
			LowerBound: min + float64(i)*width,
			UpperBound: min + float64(i+1)*width,
		}
	}

	for _, v := range sorted {
		idx := int((v - min) / width)
		if idx >= bins {
			idx = bins - 1
		}
		buckets[idx].Count++
	}

	return buckets
}
