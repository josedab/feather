// Package operator provides a lightweight control plane for managing Feather deployments.
package operator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ManagerConfig configures the operator manager.
type ManagerConfig struct {
	// MetricsAddr is the address for the metrics endpoint.
	MetricsAddr string

	// ProbeAddr is the address for health probes.
	ProbeAddr string

	// LeaderElect enables leader election.
	LeaderElect bool

	// LeaderElectLease is the leader election lease duration.
	LeaderElectLease time.Duration

	// Namespace restricts the operator to a specific namespace.
	Namespace string

	// Workers is the number of reconciler workers.
	Workers int

	// ResyncPeriod is the periodic resync interval.
	ResyncPeriod time.Duration
}

// DefaultManagerConfig returns default configuration.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MetricsAddr:      ":8081",
		ProbeAddr:        ":8082",
		LeaderElect:      true,
		LeaderElectLease: 15 * time.Second,
		Namespace:        "",
		Workers:          4,
		ResyncPeriod:     10 * time.Minute,
	}
}

// Manager manages the operator lifecycle.
type Manager struct {
	config     ManagerConfig
	controller *Controller

	mu        sync.RWMutex
	isLeader  bool
	leaderID  string
	started   bool
	startTime time.Time

	metricsServer *http.Server
	probeServer   *http.Server

	stopCh chan struct{}
}

// NewManager creates a new operator manager.
func NewManager(config ManagerConfig) (*Manager, error) {
	if config.Workers < 1 {
		config.Workers = 4
	}

	return &Manager{
		config:     config,
		controller: NewController(config.Workers),
		leaderID:   fmt.Sprintf("feather-operator-%d", time.Now().UnixNano()),
		stopCh:     make(chan struct{}),
	}, nil
}

// Controller returns the manager controller.
func (m *Manager) Controller() *Controller {
	return m.controller
}

// Start starts the manager.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("manager already started")
	}
	m.started = true
	m.startTime = time.Now()
	m.mu.Unlock()

	// Start metrics server
	if m.config.MetricsAddr != "" {
		go m.startMetricsServer()
	}

	// Start probe server
	if m.config.ProbeAddr != "" {
		go m.startProbeServer()
	}

	// Leader election (simplified)
	if m.config.LeaderElect {
		go m.runLeaderElection(ctx)
	} else {
		m.mu.Lock()
		m.isLeader = true
		m.mu.Unlock()
	}

	// Wait for leadership
	for {
		m.mu.RLock()
		isLeader := m.isLeader
		m.mu.RUnlock()

		if isLeader {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Start periodic resync
	go m.runResync(ctx)

	// Start controller
	return m.controller.Start(ctx)
}

// Stop stops the manager.
func (m *Manager) Stop() error {
	close(m.stopCh)

	if m.metricsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.metricsServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutting down metrics server: %w", err)
		}
	}

	if m.probeServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.probeServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutting down probe server: %w", err)
		}
	}

	return nil
}

func (m *Manager) startMetricsServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", m.handleMetrics)

	m.metricsServer = &http.Server{
		Addr:              m.config.MetricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := m.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		m.mu.Lock()
		m.metricsServer = nil
		m.mu.Unlock()
	}
}

func (m *Manager) startProbeServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", m.handleHealthz)
	mux.HandleFunc("/readyz", m.handleReadyz)

	m.probeServer = &http.Server{
		Addr:              m.config.ProbeAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := m.probeServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		m.mu.Lock()
		m.probeServer = nil
		m.mu.Unlock()
	}
}

func (m *Manager) handleMetrics(w http.ResponseWriter, r *http.Request) {
	stats := m.controller.Stats()

	if _, err := fmt.Fprintf(w, "# HELP feather_operator_reconcile_total Total reconciliations\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE feather_operator_reconcile_total counter\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "feather_operator_reconcile_total %d\n", stats.ReconcileCount); err != nil {
		return
	}

	if _, err := fmt.Fprintf(w, "# HELP feather_operator_reconcile_errors_total Total reconciliation errors\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE feather_operator_reconcile_errors_total counter\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "feather_operator_reconcile_errors_total %d\n", stats.ReconcileErrors); err != nil {
		return
	}

	if _, err := fmt.Fprintf(w, "# HELP feather_operator_feature_stores Number of FeatureStore resources\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE feather_operator_feature_stores gauge\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "feather_operator_feature_stores %d\n", stats.FeatureStores); err != nil {
		return
	}

	if _, err := fmt.Fprintf(w, "# HELP feather_operator_feature_groups Number of FeatureGroup resources\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE feather_operator_feature_groups gauge\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "feather_operator_feature_groups %d\n", stats.FeatureGroups); err != nil {
		return
	}

	if _, err := fmt.Fprintf(w, "# HELP feather_operator_feature_views Number of FeatureView resources\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE feather_operator_feature_views gauge\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "feather_operator_feature_views %d\n", stats.FeatureViews); err != nil {
		return
	}

	if _, err := fmt.Fprintf(w, "# HELP feather_operator_reconcile_duration_seconds Average reconcile duration\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE feather_operator_reconcile_duration_seconds gauge\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "feather_operator_reconcile_duration_seconds %f\n", stats.AvgReconcileTime.Seconds()); err != nil {
		return
	}

	m.mu.RLock()
	isLeader := m.isLeader
	m.mu.RUnlock()

	leaderVal := 0
	if isLeader {
		leaderVal = 1
	}
	if _, err := fmt.Fprintf(w, "# HELP feather_operator_is_leader Whether this instance is leader\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE feather_operator_is_leader gauge\n"); err != nil {
		return
	}
	if _, err := fmt.Fprintf(w, "feather_operator_is_leader %d\n", leaderVal); err != nil {
		return
	}
}

func (m *Manager) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (m *Manager) handleReadyz(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	isLeader := m.isLeader
	started := m.started
	m.mu.RUnlock()

	if !started {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not started"))
		return
	}

	if m.config.LeaderElect && !isLeader {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not leader"))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (m *Manager) runLeaderElection(ctx context.Context) {
	// Simplified leader election - in production, use client-go's leaderelection
	ticker := time.NewTicker(m.config.LeaderElectLease / 3)
	defer ticker.Stop()

	// Simulate acquiring leadership
	m.mu.Lock()
	m.isLeader = true
	m.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Renew lease (simplified)
		}
	}
}

func (m *Manager) runResync(ctx context.Context) {
	if m.config.ResyncPeriod <= 0 {
		return
	}

	ticker := time.NewTicker(m.config.ResyncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.resyncAll()
		}
	}
}

func (m *Manager) resyncAll() {
	// Re-enqueue all resources for reconciliation
	for _, store := range m.controller.ListFeatureStores("") {
		m.controller.enqueue("FeatureStore", store.ObjectMeta.Namespace, store.ObjectMeta.Name)
	}
	for _, group := range m.controller.ListFeatureGroups("") {
		m.controller.enqueue("FeatureGroup", group.ObjectMeta.Namespace, group.ObjectMeta.Name)
	}
	for _, view := range m.controller.ListFeatureViews("") {
		m.controller.enqueue("FeatureView", view.ObjectMeta.Namespace, view.ObjectMeta.Name)
	}
}

// IsLeader returns whether this manager is the leader.
func (m *Manager) IsLeader() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isLeader
}

// LeaderID returns the leader ID.
func (m *Manager) LeaderID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.leaderID
}

// Uptime returns the manager uptime.
func (m *Manager) Uptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.startTime.IsZero() {
		return 0
	}
	return time.Since(m.startTime)
}
