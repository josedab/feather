package anomalydetect

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// IsolationForestConfig configures the isolation forest.
type IsolationForestConfig struct {
	NumTrees         int     `json:"num_trees" yaml:"num_trees"`
	SampleSize       int     `json:"sample_size" yaml:"sample_size"`
	MaxDepth         int     `json:"max_depth" yaml:"max_depth"`
	AnomalyThreshold float64 `json:"anomaly_threshold" yaml:"anomaly_threshold"`
}

// DefaultIsolationForestConfig returns sensible defaults.
func DefaultIsolationForestConfig() IsolationForestConfig {
	return IsolationForestConfig{
		NumTrees:         100,
		SampleSize:       256,
		MaxDepth:         8,
		AnomalyThreshold: 0.6,
	}
}

// IsolationTree is a single tree in the isolation forest.
type IsolationTree struct {
	Left       *IsolationTree `json:"-"`
	Right      *IsolationTree `json:"-"`
	SplitValue float64        `json:"split_value"`
	Size       int            `json:"size"`
	Depth      int            `json:"depth"`
	IsLeaf     bool           `json:"is_leaf"`
}

// IsolationForest implements the isolation forest anomaly detection algorithm.
type IsolationForest struct {
	mu     sync.RWMutex
	config IsolationForestConfig
	trees  []*IsolationTree
	rng    *rand.Rand
	fitted bool
}

// NewIsolationForest creates a new isolation forest detector.
func NewIsolationForest(config IsolationForestConfig) *IsolationForest {
	if config.NumTrees == 0 {
		config = DefaultIsolationForestConfig()
	}
	return &IsolationForest{
		config: config,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Fit trains the isolation forest on the provided data samples.
func (f *IsolationForest) Fit(data []float64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.trees = make([]*IsolationTree, f.config.NumTrees)
	for i := 0; i < f.config.NumTrees; i++ {
		sample := f.subsample(data)
		f.trees[i] = f.buildTree(sample, 0)
	}
	f.fitted = true
}

// Score returns the anomaly score for a value (higher = more anomalous).
func (f *IsolationForest) Score(value float64) float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if !f.fitted || len(f.trees) == 0 {
		return 0
	}

	var totalPathLength float64
	for _, tree := range f.trees {
		totalPathLength += float64(f.pathLength(tree, value, 0))
	}
	avgPathLength := totalPathLength / float64(len(f.trees))

	// Anomaly score: s(x, n) = 2^(-E(h(x))/c(n))
	n := float64(f.config.SampleSize)
	cn := f.averagePathLength(n)
	if cn == 0 {
		return 0
	}
	return math.Pow(2, -avgPathLength/cn)
}

// IsAnomaly checks if a value is anomalous according to the threshold.
func (f *IsolationForest) IsAnomaly(value float64) bool {
	return f.Score(value) > f.config.AnomalyThreshold
}

func (f *IsolationForest) subsample(data []float64) []float64 {
	n := f.config.SampleSize
	if n > len(data) {
		n = len(data)
	}
	sample := make([]float64, len(data))
	copy(sample, data)
	// Fisher-Yates shuffle
	for i := len(sample) - 1; i > 0; i-- {
		j := f.rng.Intn(i + 1)
		sample[i], sample[j] = sample[j], sample[i]
	}
	return sample[:n]
}

func (f *IsolationForest) buildTree(data []float64, depth int) *IsolationTree {
	if len(data) <= 1 || depth >= f.config.MaxDepth {
		return &IsolationTree{Size: len(data), Depth: depth, IsLeaf: true}
	}

	// Find min/max for split
	minVal, maxVal := data[0], data[0]
	for _, v := range data[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	if minVal == maxVal {
		return &IsolationTree{Size: len(data), Depth: depth, IsLeaf: true}
	}

	splitVal := minVal + f.rng.Float64()*(maxVal-minVal)

	var left, right []float64
	for _, v := range data {
		if v < splitVal {
			left = append(left, v)
		} else {
			right = append(right, v)
		}
	}

	return &IsolationTree{
		SplitValue: splitVal,
		Left:       f.buildTree(left, depth+1),
		Right:      f.buildTree(right, depth+1),
		Size:       len(data),
		Depth:      depth,
	}
}

func (f *IsolationForest) pathLength(tree *IsolationTree, value float64, depth int) int {
	if tree == nil || tree.IsLeaf {
		if tree != nil && tree.Size > 1 {
			return depth + int(f.averagePathLength(float64(tree.Size)))
		}
		return depth
	}
	if value < tree.SplitValue {
		return f.pathLength(tree.Left, value, depth+1)
	}
	return f.pathLength(tree.Right, value, depth+1)
}

// averagePathLength returns the expected path length for n samples (c(n)).
func (f *IsolationForest) averagePathLength(n float64) float64 {
	if n <= 1 {
		return 0
	}
	if n == 2 {
		return 1
	}
	// c(n) = 2*H(n-1) - 2*(n-1)/n, where H(i) = ln(i) + euler_constant
	euler := 0.5772156649
	return 2*(math.Log(n-1)+euler) - 2*(n-1)/n
}

// IsFitted returns whether the forest has been trained.
func (f *IsolationForest) IsFitted() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fitted
}
