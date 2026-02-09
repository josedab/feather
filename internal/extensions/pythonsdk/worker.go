package pythonsdk

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// TransformWorkerStatus represents the status of a gRPC transform worker.
type TransformWorkerStatus string

const (
	WorkerDisconnected TransformWorkerStatus = "disconnected"
	WorkerConnecting   TransformWorkerStatus = "connecting"
	WorkerReady        TransformWorkerStatus = "ready"
	WorkerDraining     TransformWorkerStatus = "draining"
)

// TransformRequest is a feature transform execution request sent to a Python worker.
type TransformRequest struct {
	TransformID string                 `json:"transform_id"`
	EntryPoint  string                 `json:"entry_point"`
	Inputs      map[string]interface{} `json:"inputs"`
	RequestID   string                 `json:"request_id"`
	Timeout     time.Duration          `json:"timeout"`
}

// TransformResponse is the result from a Python worker.
type TransformResponse struct {
	RequestID string                 `json:"request_id"`
	Outputs   map[string]interface{} `json:"outputs"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	WorkerID  string                 `json:"worker_id"`
}

// WorkerPoolConfig configures the Python worker pool.
type WorkerPoolConfig struct {
	// Endpoints are gRPC addresses of Python transform workers.
	Endpoints       []string      `json:"endpoints" yaml:"endpoints"`
	MaxConcurrent   int           `json:"max_concurrent" yaml:"max_concurrent"`
	ConnectTimeout  time.Duration `json:"connect_timeout" yaml:"connect_timeout"`
	RequestTimeout  time.Duration `json:"request_timeout" yaml:"request_timeout"`
	HealthInterval  time.Duration `json:"health_interval" yaml:"health_interval"`
	EnableHotReload bool          `json:"enable_hot_reload" yaml:"enable_hot_reload"`
}

// DefaultWorkerPoolConfig returns sensible defaults for the worker pool.
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		Endpoints:       []string{"localhost:50052"},
		MaxConcurrent:   100,
		ConnectTimeout:  5 * time.Second,
		RequestTimeout:  30 * time.Second,
		HealthInterval:  10 * time.Second,
		EnableHotReload: true,
	}
}

// WorkerInfo describes a connected Python worker.
type WorkerInfo struct {
	ID            string                `json:"id"`
	Endpoint      string                `json:"endpoint"`
	Status        TransformWorkerStatus `json:"status"`
	ActiveTasks   int                   `json:"active_tasks"`
	TotalExecuted int64                 `json:"total_executed"`
	LastHeartbeat time.Time             `json:"last_heartbeat"`
	PythonVersion string                `json:"python_version,omitempty"`
	Packages      []string              `json:"packages,omitempty"`
}

// WorkerPool manages connections to Python transform workers.
type WorkerPool struct {
	mu      sync.RWMutex
	config  WorkerPoolConfig
	workers map[string]*WorkerInfo
	logger  *slog.Logger
	stopCh  chan struct{}
}

// NewWorkerPool creates a new Python transform worker pool.
func NewWorkerPool(config WorkerPoolConfig) *WorkerPool {
	if config.MaxConcurrent == 0 {
		config = DefaultWorkerPoolConfig()
	}
	return &WorkerPool{
		config:  config,
		workers: make(map[string]*WorkerInfo),
		logger:  slog.Default(),
		stopCh:  make(chan struct{}),
	}
}

// Start initializes connections to configured worker endpoints.
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	for i, endpoint := range wp.config.Endpoints {
		workerID := fmt.Sprintf("worker-%d", i)
		wp.workers[workerID] = &WorkerInfo{
			ID:       workerID,
			Endpoint: endpoint,
			Status:   WorkerDisconnected,
		}
	}

	// Start health check loop
	go wp.healthCheckLoop(ctx)

	return nil
}

// Stop gracefully shuts down the worker pool.
func (wp *WorkerPool) Stop() {
	close(wp.stopCh)
}

// Execute sends a transform request to an available worker.
func (wp *WorkerPool) Execute(ctx context.Context, req *TransformRequest) (*TransformResponse, error) {
	worker, err := wp.selectWorker()
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// In a full implementation, this would use gRPC to call the Python worker.
	// For now, return a placeholder response indicating the worker was selected.
	resp := &TransformResponse{
		RequestID: req.RequestID,
		Outputs:   make(map[string]interface{}),
		Success:   true,
		Duration:  time.Since(start),
		WorkerID:  worker.ID,
	}

	wp.mu.Lock()
	worker.TotalExecuted++
	wp.mu.Unlock()

	return resp, nil
}

// Workers returns info about all workers.
func (wp *WorkerPool) Workers() []WorkerInfo {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	result := make([]WorkerInfo, 0, len(wp.workers))
	for _, w := range wp.workers {
		result = append(result, *w)
	}
	return result
}

// HotReload triggers a reload of transform code on all workers.
func (wp *WorkerPool) HotReload(ctx context.Context, transformID string) error {
	if !wp.config.EnableHotReload {
		return fmt.Errorf("hot reload disabled")
	}

	wp.mu.RLock()
	defer wp.mu.RUnlock()

	// In a full implementation, send reload command via gRPC to workers
	wp.logger.Info("hot-reload triggered", "transform_id", transformID)
	return nil
}

func (wp *WorkerPool) selectWorker() (*WorkerInfo, error) {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	var best *WorkerInfo
	for _, w := range wp.workers {
		if w.Status == WorkerReady || w.Status == WorkerDisconnected {
			if best == nil || w.ActiveTasks < best.ActiveTasks {
				best = w
			}
		}
	}
	if best == nil {
		return nil, ErrWorkerUnavailable
	}
	return best, nil
}

func (wp *WorkerPool) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(wp.config.HealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wp.stopCh:
			return
		case <-ticker.C:
			wp.checkWorkerHealth()
		}
	}
}

func (wp *WorkerPool) checkWorkerHealth() {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	for _, w := range wp.workers {
		// In a full implementation, ping the gRPC endpoint
		w.LastHeartbeat = time.Now()
		if w.Status == WorkerDisconnected {
			w.Status = WorkerReady
		}
	}
}
