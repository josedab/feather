package operator

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Controller manages the reconciliation of Feather custom resources.
type Controller struct {
	mu sync.RWMutex

	// Resource caches
	featureStores map[string]*FeatureStore
	featureGroups map[string]*FeatureGroup
	featureViews  map[string]*FeatureView

	// Reconciler state
	reconcileQueue chan reconcileRequest
	workers        int
	stopCh         chan struct{}

	// Callbacks
	onFeatureStoreChange func(*FeatureStore) error
	onFeatureGroupChange func(*FeatureGroup) error
	onFeatureViewChange  func(*FeatureView) error

	// Metrics
	stats controllerStats
}

type reconcileRequest struct {
	kind      string
	namespace string
	name      string
}

type controllerStats struct {
	reconcileCount   int64
	reconcileErrors  int64
	featureStores    int64
	featureGroups    int64
	featureViews     int64
	lastReconcile    time.Time
	avgReconcileTime time.Duration
}

// NewController creates a new controller.
func NewController(workers int) *Controller {
	return &Controller{
		featureStores:  make(map[string]*FeatureStore),
		featureGroups:  make(map[string]*FeatureGroup),
		featureViews:   make(map[string]*FeatureView),
		reconcileQueue: make(chan reconcileRequest, 1000),
		workers:        workers,
		stopCh:         make(chan struct{}),
	}
}

// OnFeatureStoreChange sets the callback for FeatureStore changes.
func (c *Controller) OnFeatureStoreChange(fn func(*FeatureStore) error) {
	c.onFeatureStoreChange = fn
}

// OnFeatureGroupChange sets the callback for FeatureGroup changes.
func (c *Controller) OnFeatureGroupChange(fn func(*FeatureGroup) error) {
	c.onFeatureGroupChange = fn
}

// OnFeatureViewChange sets the callback for FeatureView changes.
func (c *Controller) OnFeatureViewChange(fn func(*FeatureView) error) {
	c.onFeatureViewChange = fn
}

// Start starts the controller workers.
func (c *Controller) Start(ctx context.Context) error {
	for i := 0; i < c.workers; i++ {
		go c.worker(ctx, i)
	}

	<-ctx.Done()
	close(c.stopCh)
	return ctx.Err()
}

func (c *Controller) worker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case req := <-c.reconcileQueue:
			start := time.Now()
			if err := c.reconcile(ctx, req); err != nil {
				c.mu.Lock()
				c.stats.reconcileErrors++
				c.mu.Unlock()
			}
			c.mu.Lock()
			c.stats.reconcileCount++
			c.stats.lastReconcile = time.Now()
			c.stats.avgReconcileTime = time.Since(start)
			c.mu.Unlock()
		}
	}
}

func (c *Controller) reconcile(ctx context.Context, req reconcileRequest) error {
	key := req.namespace + "/" + req.name

	switch req.kind {
	case "FeatureStore":
		return c.reconcileFeatureStore(ctx, key)
	case "FeatureGroup":
		return c.reconcileFeatureGroup(ctx, key)
	case "FeatureView":
		return c.reconcileFeatureView(ctx, key)
	default:
		return fmt.Errorf("unknown kind: %s", req.kind)
	}
}

func (c *Controller) reconcileFeatureStore(ctx context.Context, key string) error {
	c.mu.RLock()
	store, ok := c.featureStores[key]
	c.mu.RUnlock()

	if !ok {
		return nil // Deleted
	}

	// Update status
	store.Status.ObservedGeneration = store.ObjectMeta.Generation
	store.Status.LastUpdateTime = time.Now()

	// Check if creating or updating
	if store.Status.Phase == "" || store.Status.Phase == PhasePending {
		store.Status.Phase = PhaseCreating
	}

	// Validate spec
	if err := c.validateFeatureStoreSpec(&store.Spec); err != nil {
		store.Status.Phase = PhaseFailed
		c.setCondition(&store.Status.Conditions, "Ready", "False", "ValidationFailed", err.Error())
		return err
	}

	// Simulate deployment reconciliation
	store.Status.ReadyReplicas = store.Spec.Replicas
	store.Status.AvailableReplicas = store.Spec.Replicas
	store.Status.Phase = PhaseRunning

	// Set endpoints
	store.Status.Endpoints = EndpointStatus{
		HTTP:    fmt.Sprintf("%s:%d", store.ObjectMeta.Name, store.Spec.Config.HTTPPort),
		GRPC:    fmt.Sprintf("%s:%d", store.ObjectMeta.Name, store.Spec.Config.GRPCPort),
		Metrics: fmt.Sprintf("%s:%d", store.ObjectMeta.Name, store.Spec.Config.MetricsPort),
	}

	c.setCondition(&store.Status.Conditions, "Ready", "True", "ReconcileSuccess", "FeatureStore is ready")

	// Callback
	if c.onFeatureStoreChange != nil {
		return c.onFeatureStoreChange(store)
	}

	return nil
}

func (c *Controller) reconcileFeatureGroup(ctx context.Context, key string) error {
	c.mu.RLock()
	group, ok := c.featureGroups[key]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	group.Status.ObservedGeneration = group.ObjectMeta.Generation
	group.Status.FeatureCount = len(group.Spec.Features)
	group.Status.LastSyncTime = time.Now()
	group.Status.Phase = "Ready"

	c.setCondition(&group.Status.Conditions, "Ready", "True", "Synced", "FeatureGroup is synced")

	if c.onFeatureGroupChange != nil {
		return c.onFeatureGroupChange(group)
	}

	return nil
}

func (c *Controller) reconcileFeatureView(ctx context.Context, key string) error {
	c.mu.RLock()
	view, ok := c.featureViews[key]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	view.Status.ObservedGeneration = view.ObjectMeta.Generation
	view.Status.LastMaterializationTime = time.Now()
	view.Status.Phase = "Ready"

	c.setCondition(&view.Status.Conditions, "Ready", "True", "Materialized", "FeatureView is materialized")

	if c.onFeatureViewChange != nil {
		return c.onFeatureViewChange(view)
	}

	return nil
}

func (c *Controller) validateFeatureStoreSpec(spec *FeatureStoreSpec) error {
	if spec.Replicas < 1 {
		return fmt.Errorf("replicas must be at least 1")
	}
	if spec.Config.HTTPPort == 0 {
		spec.Config.HTTPPort = 8080
	}
	if spec.Config.GRPCPort == 0 {
		spec.Config.GRPCPort = 50051
	}
	if spec.Config.MetricsPort == 0 {
		spec.Config.MetricsPort = 9090
	}
	return nil
}

func (c *Controller) setCondition(conditions *[]Condition, condType, status, reason, message string) {
	now := time.Now()
	for i, cond := range *conditions {
		if cond.Type == condType {
			if cond.Status != status {
				(*conditions)[i].LastTransitionTime = now
			}
			(*conditions)[i].Status = status
			(*conditions)[i].Reason = reason
			(*conditions)[i].Message = message
			return
		}
	}
	*conditions = append(*conditions, Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// CreateFeatureStore creates or updates a FeatureStore.
func (c *Controller) CreateFeatureStore(store *FeatureStore) error {
	key := store.ObjectMeta.Namespace + "/" + store.ObjectMeta.Name

	c.mu.Lock()
	store.ObjectMeta.Generation++
	store.ObjectMeta.CreationTimestamp = time.Now()
	store.Status.Phase = PhasePending
	c.featureStores[key] = store
	c.stats.featureStores = int64(len(c.featureStores))
	c.mu.Unlock()

	c.enqueue("FeatureStore", store.ObjectMeta.Namespace, store.ObjectMeta.Name)
	return nil
}

// GetFeatureStore gets a FeatureStore by name.
func (c *Controller) GetFeatureStore(namespace, name string) (*FeatureStore, error) {
	key := namespace + "/" + name

	c.mu.RLock()
	defer c.mu.RUnlock()

	store, ok := c.featureStores[key]
	if !ok {
		return nil, fmt.Errorf("feature store not found: %s", key)
	}
	return store, nil
}

// ListFeatureStores lists all FeatureStores.
func (c *Controller) ListFeatureStores(namespace string) []*FeatureStore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*FeatureStore, 0)
	for key, store := range c.featureStores {
		if namespace == "" || store.ObjectMeta.Namespace == namespace {
			_ = key
			result = append(result, store)
		}
	}
	return result
}

// DeleteFeatureStore deletes a FeatureStore.
func (c *Controller) DeleteFeatureStore(namespace, name string) error {
	key := namespace + "/" + name

	c.mu.Lock()
	defer c.mu.Unlock()

	if store, ok := c.featureStores[key]; ok {
		now := time.Now()
		store.ObjectMeta.DeletionTimestamp = &now
		store.Status.Phase = PhaseTerminating
	}

	delete(c.featureStores, key)
	c.stats.featureStores = int64(len(c.featureStores))
	return nil
}

// CreateFeatureGroup creates a FeatureGroup.
func (c *Controller) CreateFeatureGroup(group *FeatureGroup) error {
	key := group.ObjectMeta.Namespace + "/" + group.ObjectMeta.Name

	c.mu.Lock()
	group.ObjectMeta.Generation++
	group.ObjectMeta.CreationTimestamp = time.Now()
	c.featureGroups[key] = group
	c.stats.featureGroups = int64(len(c.featureGroups))
	c.mu.Unlock()

	c.enqueue("FeatureGroup", group.ObjectMeta.Namespace, group.ObjectMeta.Name)
	return nil
}

// GetFeatureGroup gets a FeatureGroup.
func (c *Controller) GetFeatureGroup(namespace, name string) (*FeatureGroup, error) {
	key := namespace + "/" + name

	c.mu.RLock()
	defer c.mu.RUnlock()

	group, ok := c.featureGroups[key]
	if !ok {
		return nil, fmt.Errorf("feature group not found: %s", key)
	}
	return group, nil
}

// ListFeatureGroups lists FeatureGroups.
func (c *Controller) ListFeatureGroups(namespace string) []*FeatureGroup {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*FeatureGroup, 0)
	for _, group := range c.featureGroups {
		if namespace == "" || group.ObjectMeta.Namespace == namespace {
			result = append(result, group)
		}
	}
	return result
}

// DeleteFeatureGroup deletes a FeatureGroup.
func (c *Controller) DeleteFeatureGroup(namespace, name string) error {
	key := namespace + "/" + name

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.featureGroups, key)
	c.stats.featureGroups = int64(len(c.featureGroups))
	return nil
}

// CreateFeatureView creates a FeatureView.
func (c *Controller) CreateFeatureView(view *FeatureView) error {
	key := view.ObjectMeta.Namespace + "/" + view.ObjectMeta.Name

	c.mu.Lock()
	view.ObjectMeta.Generation++
	view.ObjectMeta.CreationTimestamp = time.Now()
	c.featureViews[key] = view
	c.stats.featureViews = int64(len(c.featureViews))
	c.mu.Unlock()

	c.enqueue("FeatureView", view.ObjectMeta.Namespace, view.ObjectMeta.Name)
	return nil
}

// GetFeatureView gets a FeatureView.
func (c *Controller) GetFeatureView(namespace, name string) (*FeatureView, error) {
	key := namespace + "/" + name

	c.mu.RLock()
	defer c.mu.RUnlock()

	view, ok := c.featureViews[key]
	if !ok {
		return nil, fmt.Errorf("feature view not found: %s", key)
	}
	return view, nil
}

// ListFeatureViews lists FeatureViews.
func (c *Controller) ListFeatureViews(namespace string) []*FeatureView {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*FeatureView, 0)
	for _, view := range c.featureViews {
		if namespace == "" || view.ObjectMeta.Namespace == namespace {
			result = append(result, view)
		}
	}
	return result
}

// DeleteFeatureView deletes a FeatureView.
func (c *Controller) DeleteFeatureView(namespace, name string) error {
	key := namespace + "/" + name

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.featureViews, key)
	c.stats.featureViews = int64(len(c.featureViews))
	return nil
}

func (c *Controller) enqueue(kind, namespace, name string) {
	select {
	case c.reconcileQueue <- reconcileRequest{kind: kind, namespace: namespace, name: name}:
	default:
		// Queue full, skip
	}
}

// Stats returns controller statistics.
func (c *Controller) Stats() ControllerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return ControllerStats{
		ReconcileCount:   c.stats.reconcileCount,
		ReconcileErrors:  c.stats.reconcileErrors,
		FeatureStores:    c.stats.featureStores,
		FeatureGroups:    c.stats.featureGroups,
		FeatureViews:     c.stats.featureViews,
		LastReconcile:    c.stats.lastReconcile,
		AvgReconcileTime: c.stats.avgReconcileTime,
	}
}

// ControllerStats contains controller statistics.
type ControllerStats struct {
	ReconcileCount   int64         `json:"reconcile_count"`
	ReconcileErrors  int64         `json:"reconcile_errors"`
	FeatureStores    int64         `json:"feature_stores"`
	FeatureGroups    int64         `json:"feature_groups"`
	FeatureViews     int64         `json:"feature_views"`
	LastReconcile    time.Time     `json:"last_reconcile"`
	AvgReconcileTime time.Duration `json:"avg_reconcile_time"`
}
