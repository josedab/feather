package operator

import (
	"context"
	"testing"
	"time"
)

func TestController_FeatureStore(t *testing.T) {
	ctrl := NewController(2)

	// Start controller in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ctrl.Start(ctx)

	// Create a FeatureStore
	store := &FeatureStore{
		TypeMeta: TypeMeta{
			Kind:       "FeatureStore",
			APIVersion: "feather.io/v1",
		},
		ObjectMeta: ObjectMeta{
			Name:      "test-store",
			Namespace: "default",
		},
		Spec: FeatureStoreSpec{
			Replicas: 3,
			Config: FeatureStoreConfig{
				HTTPPort:    8080,
				GRPCPort:    50051,
				MetricsPort: 9090,
			},
		},
	}

	if err := ctrl.CreateFeatureStore(store); err != nil {
		t.Fatalf("create feature store: %v", err)
	}

	// Wait for reconciliation
	time.Sleep(100 * time.Millisecond)

	// Get and verify
	retrieved, err := ctrl.GetFeatureStore("default", "test-store")
	if err != nil {
		t.Fatalf("get feature store: %v", err)
	}

	if retrieved.Status.Phase != PhaseRunning {
		t.Errorf("expected phase Running, got %s", retrieved.Status.Phase)
	}

	if retrieved.Status.ReadyReplicas != 3 {
		t.Errorf("expected 3 ready replicas, got %d", retrieved.Status.ReadyReplicas)
	}

	// List
	stores := ctrl.ListFeatureStores("default")
	if len(stores) != 1 {
		t.Errorf("expected 1 store, got %d", len(stores))
	}

	// Delete
	if err := ctrl.DeleteFeatureStore("default", "test-store"); err != nil {
		t.Fatalf("delete feature store: %v", err)
	}

	stores = ctrl.ListFeatureStores("default")
	if len(stores) != 0 {
		t.Errorf("expected 0 stores after delete, got %d", len(stores))
	}
}

func TestController_FeatureGroup(t *testing.T) {
	ctrl := NewController(2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ctrl.Start(ctx)

	group := &FeatureGroup{
		TypeMeta: TypeMeta{
			Kind:       "FeatureGroup",
			APIVersion: "feather.io/v1",
		},
		ObjectMeta: ObjectMeta{
			Name:      "user-features",
			Namespace: "default",
		},
		Spec: FeatureGroupSpec{
			FeatureStoreRef: "test-store",
			EntityType:      "user",
			Features: []FeatureSpec{
				{Name: "click_count", Type: "int64"},
				{Name: "last_seen", Type: "timestamp"},
			},
		},
	}

	if err := ctrl.CreateFeatureGroup(group); err != nil {
		t.Fatalf("create feature group: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	retrieved, err := ctrl.GetFeatureGroup("default", "user-features")
	if err != nil {
		t.Fatalf("get feature group: %v", err)
	}

	if retrieved.Status.FeatureCount != 2 {
		t.Errorf("expected 2 features, got %d", retrieved.Status.FeatureCount)
	}

	groups := ctrl.ListFeatureGroups("default")
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

func TestController_FeatureView(t *testing.T) {
	ctrl := NewController(2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ctrl.Start(ctx)

	view := &FeatureView{
		TypeMeta: TypeMeta{
			Kind:       "FeatureView",
			APIVersion: "feather.io/v1",
		},
		ObjectMeta: ObjectMeta{
			Name:      "user-view",
			Namespace: "default",
		},
		Spec: FeatureViewSpec{
			FeatureStoreRef: "test-store",
			Features: []FeatureRef{
				{Group: "user-features", Feature: "click_count"},
			},
			Schedule: "0 * * * *",
		},
	}

	if err := ctrl.CreateFeatureView(view); err != nil {
		t.Fatalf("create feature view: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	retrieved, err := ctrl.GetFeatureView("default", "user-view")
	if err != nil {
		t.Fatalf("get feature view: %v", err)
	}

	if retrieved.Status.Phase != "Ready" {
		t.Errorf("expected phase Ready, got %s", retrieved.Status.Phase)
	}

	views := ctrl.ListFeatureViews("default")
	if len(views) != 1 {
		t.Errorf("expected 1 view, got %d", len(views))
	}
}

func TestController_Callbacks(t *testing.T) {
	ctrl := NewController(2)

	callbackCh := make(chan struct{}, 1)
	ctrl.OnFeatureStoreChange(func(store *FeatureStore) error {
		callbackCh <- struct{}{}
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ctrl.Start(ctx)

	store := &FeatureStore{
		ObjectMeta: ObjectMeta{
			Name:      "callback-test",
			Namespace: "default",
		},
		Spec: FeatureStoreSpec{
			Replicas: 1,
		},
	}

	ctrl.CreateFeatureStore(store)
	select {
	case <-callbackCh:
	case <-time.After(200 * time.Millisecond):
		t.Error("expected store callback to be called")
	}
}

func TestController_Stats(t *testing.T) {
	ctrl := NewController(2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ctrl.Start(ctx)

	ctrl.CreateFeatureStore(&FeatureStore{
		ObjectMeta: ObjectMeta{Name: "s1", Namespace: "default"},
		Spec:       FeatureStoreSpec{Replicas: 1},
	})

	ctrl.CreateFeatureGroup(&FeatureGroup{
		ObjectMeta: ObjectMeta{Name: "g1", Namespace: "default"},
		Spec:       FeatureGroupSpec{EntityType: "user"},
	})

	time.Sleep(100 * time.Millisecond)

	stats := ctrl.Stats()

	if stats.FeatureStores != 1 {
		t.Errorf("expected 1 feature store, got %d", stats.FeatureStores)
	}

	if stats.FeatureGroups != 1 {
		t.Errorf("expected 1 feature group, got %d", stats.FeatureGroups)
	}

	if stats.ReconcileCount < 2 {
		t.Errorf("expected at least 2 reconciles, got %d", stats.ReconcileCount)
	}
}

func TestController_ValidationFailure(t *testing.T) {
	ctrl := NewController(2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ctrl.Start(ctx)

	// Create with invalid replicas
	store := &FeatureStore{
		ObjectMeta: ObjectMeta{
			Name:      "invalid-store",
			Namespace: "default",
		},
		Spec: FeatureStoreSpec{
			Replicas: 0, // Invalid
		},
	}

	ctrl.CreateFeatureStore(store)
	time.Sleep(100 * time.Millisecond)

	retrieved, _ := ctrl.GetFeatureStore("default", "invalid-store")
	if retrieved.Status.Phase != PhaseFailed {
		t.Errorf("expected phase Failed, got %s", retrieved.Status.Phase)
	}
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.Workers != 4 {
		t.Errorf("expected 4 workers, got %d", config.Workers)
	}

	if config.MetricsAddr != ":8081" {
		t.Errorf("expected :8081, got %s", config.MetricsAddr)
	}

	if !config.LeaderElect {
		t.Error("expected leader election enabled")
	}
}
