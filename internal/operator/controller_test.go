package operator

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestReconciler_ReconcileFeatureStore(t *testing.T) {
	client := fake.NewSimpleClientset()
	reconciler := NewReconciler(client, nil, "default")

	store := &FeatureStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-store",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: FeatureStoreSpec{
			Replicas: 3,
			Version:  "1.0.0",
			Image:    "feather/feather:1.0.0",
			Config: FeatherConfig{
				HTTPPort:    8080,
				GRPCPort:    50051,
				MetricsPort: 9090,
				LogLevel:    "info",
			},
			Resources: ResourceSpec{
				CPURequest:    "500m",
				CPULimit:      "2",
				MemoryRequest: "1Gi",
				MemoryLimit:   "4Gi",
			},
			Storage: StorageSpec{
				HotTier: HotTierSpec{
					MaxMemory: "2Gi",
					TTL:       "1h",
				},
				WarmTier: WarmTierSpec{
					StorageClass: "standard",
					Size:         "100Gi",
				},
			},
		},
	}

	ctx := context.Background()
	result := reconciler.ReconcileFeatureStore(ctx, store)

	if result.Error != nil {
		t.Fatalf("ReconcileFeatureStore failed: %v", result.Error)
	}

	if store.Status.Phase != PhaseRunning {
		t.Errorf("expected phase Running, got %s", store.Status.Phase)
	}

	// Verify ConfigMap was created
	cm, err := client.CoreV1().ConfigMaps("default").Get(ctx, "test-store-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap not created: %v", err)
	}

	if cm.Data["FEATHER_HTTP_PORT"] != "8080" {
		t.Errorf("expected HTTP port 8080, got %s", cm.Data["FEATHER_HTTP_PORT"])
	}

	// Verify Service was created
	svc, err := client.CoreV1().Services("default").Get(ctx, "test-store", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Service not created: %v", err)
	}

	if len(svc.Spec.Ports) != 3 {
		t.Errorf("expected 3 service ports, got %d", len(svc.Spec.Ports))
	}

	// Verify StatefulSet was created
	sts, err := client.AppsV1().StatefulSets("default").Get(ctx, "test-store", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("StatefulSet not created: %v", err)
	}

	if *sts.Spec.Replicas != 3 {
		t.Errorf("expected 3 replicas, got %d", *sts.Spec.Replicas)
	}

	if sts.Spec.Template.Spec.Containers[0].Image != "feather/feather:1.0.0" {
		t.Errorf("unexpected image: %s", sts.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestReconciler_ReconcileFeatureStore_WithAutoscaling(t *testing.T) {
	client := fake.NewSimpleClientset()
	reconciler := NewReconciler(client, nil, "default")

	store := &FeatureStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "autoscale-store",
			Namespace: "default",
			UID:       "test-uid-2",
		},
		Spec: FeatureStoreSpec{
			Replicas: 2,
			Version:  "1.0.0",
			Autoscaling: &AutoscalingSpec{
				Enabled:              true,
				MinReplicas:          2,
				MaxReplicas:          10,
				TargetCPUUtilization: 70,
			},
		},
	}

	ctx := context.Background()
	result := reconciler.ReconcileFeatureStore(ctx, store)

	if result.Error != nil {
		t.Fatalf("ReconcileFeatureStore failed: %v", result.Error)
	}
}

func TestReconciler_ReconcileFeatureStore_WithHA(t *testing.T) {
	client := fake.NewSimpleClientset()
	reconciler := NewReconciler(client, nil, "default")

	minAvailable := int32(2)
	store := &FeatureStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ha-store",
			Namespace: "default",
			UID:       "test-uid-3",
		},
		Spec: FeatureStoreSpec{
			Replicas: 3,
			Version:  "1.0.0",
			HighAvailability: &HASpec{
				Enabled:      true,
				AntiAffinity: true,
				PodDisruptionBudget: &PDBSpec{
					MinAvailable: &minAvailable,
				},
			},
		},
	}

	ctx := context.Background()
	result := reconciler.ReconcileFeatureStore(ctx, store)

	if result.Error != nil {
		t.Fatalf("ReconcileFeatureStore failed: %v", result.Error)
	}

	// Verify anti-affinity was set
	sts, err := client.AppsV1().StatefulSets("default").Get(ctx, "ha-store", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("StatefulSet not created: %v", err)
	}

	if sts.Spec.Template.Spec.Affinity == nil {
		t.Error("expected affinity to be set")
	}
}

func TestReconciler_ReconcileFeatureStore_Deletion(t *testing.T) {
	client := fake.NewSimpleClientset()
	reconciler := NewReconciler(client, nil, "default")

	now := metav1.Now()
	store := &FeatureStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "delete-store",
			Namespace:         "default",
			UID:               "test-uid-4",
			DeletionTimestamp: &now,
			Finalizers:        []string{"feather.io/finalizer"},
		},
		Spec: FeatureStoreSpec{
			Replicas: 1,
			Version:  "1.0.0",
		},
	}

	ctx := context.Background()
	result := reconciler.ReconcileFeatureStore(ctx, store)

	if result.Error != nil {
		t.Fatalf("ReconcileFeatureStore deletion failed: %v", result.Error)
	}

	if store.Status.Phase != PhaseDeleting {
		t.Errorf("expected phase Deleting, got %s", store.Status.Phase)
	}

	// Finalizer should be removed
	if containsString(store.ObjectMeta.Finalizers, "feather.io/finalizer") {
		t.Error("finalizer should be removed after deletion")
	}
}

func TestReconciler_ReconcileFeatureGroup(t *testing.T) {
	client := fake.NewSimpleClientset()
	reconciler := NewReconciler(client, nil, "default")

	minVal := 0.0
	maxVal := 100.0
	group := &FeatureGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-features",
			Namespace: "default",
		},
		Spec: FeatureGroupSpec{
			Store:       "test-store",
			Name:        "user_features",
			Description: "User behavior features",
			Entity:      "user_id",
			Owner:       "data-team",
			Features: []FeatureSpec{
				{
					Name:        "purchase_count",
					Type:        "int64",
					Description: "Number of purchases",
					Validation: &ValidationSpec{
						Required: true,
						Min:      &minVal,
					},
				},
				{
					Name:        "total_spend",
					Type:        "float64",
					Description: "Total spend in USD",
					Validation: &ValidationSpec{
						Min: &minVal,
						Max: &maxVal,
					},
				},
			},
			Tags: map[string]string{
				"team":     "data-science",
				"priority": "high",
			},
		},
	}

	ctx := context.Background()
	result := reconciler.ReconcileFeatureGroup(ctx, group)

	if result.Error != nil {
		t.Fatalf("ReconcileFeatureGroup failed: %v", result.Error)
	}

	if group.Status.Phase != PhaseRunning {
		t.Errorf("expected phase Running, got %s", group.Status.Phase)
	}

	if group.Status.FeatureCount != 2 {
		t.Errorf("expected 2 features, got %d", group.Status.FeatureCount)
	}

	if group.Status.LastUpdated == nil {
		t.Error("expected LastUpdated to be set")
	}
}

func TestReconciler_ReconcileFeatureView(t *testing.T) {
	client := fake.NewSimpleClientset()
	reconciler := NewReconciler(client, nil, "default")

	view := &FeatureView{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-view",
			Namespace: "default",
		},
		Spec: FeatureViewSpec{
			Store:       "test-store",
			Name:        "user_feature_view",
			Description: "Combined user features for ML",
			Sources: []FeatureSourceSpec{
				{
					Group:    "user-features",
					Features: []string{"purchase_count", "total_spend"},
				},
				{
					Group:    "product-features",
					Features: []string{"avg_rating"},
					Alias: map[string]string{
						"avg_rating": "user_avg_product_rating",
					},
				},
			},
			Transformations: []TransformationSpec{
				{
					Name:       "normalize_spend",
					Type:       "normalize",
					Expression: "(total_spend - min) / (max - min)",
				},
			},
			Materialization: &MaterializationSpec{
				Enabled:  true,
				Schedule: "0 * * * *",
				Mode:     "incremental",
				Destination: MaterializationDestSpec{
					Type: "both",
					OnlineStore: &OnlineStoreSpec{
						TTL: "24h",
					},
					OfflineStore: &OfflineStoreSpec{
						Format: "parquet",
						Path:   "/data/features/user_view",
					},
				},
			},
		},
	}

	ctx := context.Background()
	result := reconciler.ReconcileFeatureView(ctx, view)

	if result.Error != nil {
		t.Fatalf("ReconcileFeatureView failed: %v", result.Error)
	}

	if view.Status.Phase != PhaseRunning {
		t.Errorf("expected phase Running, got %s", view.Status.Phase)
	}
}

func TestLabels(t *testing.T) {
	client := fake.NewSimpleClientset()
	reconciler := NewReconciler(client, nil, "default")

	store := &FeatureStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-store",
		},
		Spec: FeatureStoreSpec{
			Version: "1.2.3",
		},
	}

	labels := reconciler.labels(store)

	if labels["app.kubernetes.io/name"] != "feather" {
		t.Errorf("unexpected name label: %s", labels["app.kubernetes.io/name"])
	}

	if labels["app.kubernetes.io/instance"] != "test-store" {
		t.Errorf("unexpected instance label: %s", labels["app.kubernetes.io/instance"])
	}

	if labels["app.kubernetes.io/version"] != "1.2.3" {
		t.Errorf("unexpected version label: %s", labels["app.kubernetes.io/version"])
	}
}

func TestDefaultControllerConfig(t *testing.T) {
	config := DefaultControllerConfig()

	if config.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", config.Namespace)
	}

	if config.Workers != 2 {
		t.Errorf("expected 2 workers, got %d", config.Workers)
	}

	if !config.LeaderElection {
		t.Error("expected leader election to be enabled")
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !containsString(slice, "b") {
		t.Error("expected to find 'b'")
	}

	if containsString(slice, "d") {
		t.Error("did not expect to find 'd'")
	}
}

func TestRemoveString(t *testing.T) {
	slice := []string{"a", "b", "c"}
	result := removeString(slice, "b")

	if len(result) != 2 {
		t.Errorf("expected length 2, got %d", len(result))
	}

	if containsString(result, "b") {
		t.Error("'b' should be removed")
	}
}
