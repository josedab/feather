package operator

import (
	"context"
	"fmt"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// Controller manages FeatureStore resources.
type Controller struct {
	mu            sync.RWMutex
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
	stores        map[string]*FeatureStore
	groups        map[string]*FeatureGroup
	views         map[string]*FeatureView
	reconciler    *Reconciler
}

// ControllerConfig holds controller configuration.
type ControllerConfig struct {
	Namespace       string
	ResyncPeriod    time.Duration
	Workers         int
	LeaderElection  bool
	MetricsAddr     string
	HealthProbeAddr string
}

// DefaultControllerConfig returns default configuration.
func DefaultControllerConfig() ControllerConfig {
	return ControllerConfig{
		Namespace:       "default",
		ResyncPeriod:    30 * time.Second,
		Workers:         2,
		LeaderElection:  true,
		MetricsAddr:     ":8080",
		HealthProbeAddr: ":8081",
	}
}

// NewController creates a new operator controller.
func NewController(client kubernetes.Interface, dynamicClient dynamic.Interface, config ControllerConfig) *Controller {
	c := &Controller{
		client:        client,
		dynamicClient: dynamicClient,
		namespace:     config.Namespace,
		stores:        make(map[string]*FeatureStore),
		groups:        make(map[string]*FeatureGroup),
		views:         make(map[string]*FeatureView),
	}
	c.reconciler = NewReconciler(client, dynamicClient, config.Namespace)
	return c
}

// Reconciler handles reconciliation of resources.
type Reconciler struct {
	client        kubernetes.Interface
	dynamicClient dynamic.Interface
	namespace     string
}

// NewReconciler creates a new reconciler.
func NewReconciler(client kubernetes.Interface, dynamicClient dynamic.Interface, namespace string) *Reconciler {
	return &Reconciler{
		client:        client,
		dynamicClient: dynamicClient,
		namespace:     namespace,
	}
}

// ReconcileFeatureStore reconciles a FeatureStore resource.
func (r *Reconciler) ReconcileFeatureStore(ctx context.Context, store *FeatureStore) ReconcileResult {
	// Check if being deleted
	if store.ObjectMeta.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, store)
	}

	// Ensure finalizer
	if !containsString(store.ObjectMeta.Finalizers, "feather.io/finalizer") {
		store.ObjectMeta.Finalizers = append(store.ObjectMeta.Finalizers, "feather.io/finalizer")
	}

	// Create or update resources
	if err := r.ensureConfigMap(ctx, store); err != nil {
		return ReconcileResult{Error: fmt.Errorf("ensuring configmap: %w", err)}
	}

	if err := r.ensureService(ctx, store); err != nil {
		return ReconcileResult{Error: fmt.Errorf("ensuring service: %w", err)}
	}

	if err := r.ensureStatefulSet(ctx, store); err != nil {
		return ReconcileResult{Error: fmt.Errorf("ensuring statefulset: %w", err)}
	}

	if store.Spec.Autoscaling != nil && store.Spec.Autoscaling.Enabled {
		if err := r.ensureHPA(ctx, store); err != nil {
			return ReconcileResult{Error: fmt.Errorf("ensuring hpa: %w", err)}
		}
	}

	if store.Spec.HighAvailability != nil && store.Spec.HighAvailability.PodDisruptionBudget != nil {
		if err := r.ensurePDB(ctx, store); err != nil {
			return ReconcileResult{Error: fmt.Errorf("ensuring pdb: %w", err)}
		}
	}

	if store.Spec.Monitoring != nil && store.Spec.Monitoring.ServiceMonitor {
		if err := r.ensureServiceMonitor(ctx, store); err != nil {
			return ReconcileResult{Error: fmt.Errorf("ensuring servicemonitor: %w", err)}
		}
	}

	// Update status
	store.Status.Phase = PhaseRunning
	store.Status.Version = store.Spec.Version

	return ReconcileResult{RequeueAfter: 30 * time.Second}
}

func (r *Reconciler) handleDeletion(ctx context.Context, store *FeatureStore) ReconcileResult {
	store.Status.Phase = PhaseDeleting

	// Cleanup resources
	name := store.ObjectMeta.Name
	namespace := store.ObjectMeta.Namespace
	if namespace == "" {
		namespace = r.namespace
	}

	// Delete in reverse order
	r.client.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	r.client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	r.client.CoreV1().ConfigMaps(namespace).Delete(ctx, name+"-config", metav1.DeleteOptions{})

	// Remove finalizer
	store.ObjectMeta.Finalizers = removeString(store.ObjectMeta.Finalizers, "feather.io/finalizer")

	return ReconcileResult{}
}

func (r *Reconciler) ensureConfigMap(ctx context.Context, store *FeatureStore) error {
	name := store.ObjectMeta.Name + "-config"
	namespace := store.ObjectMeta.Namespace
	if namespace == "" {
		namespace = r.namespace
	}

	config := store.Spec.Config

	data := map[string]string{
		"FEATHER_HTTP_PORT":    fmt.Sprintf("%d", config.HTTPPort),
		"FEATHER_GRPC_PORT":    fmt.Sprintf("%d", config.GRPCPort),
		"FEATHER_METRICS_PORT": fmt.Sprintf("%d", config.MetricsPort),
		"FEATHER_LOG_LEVEL":    config.LogLevel,
		"FEATHER_LOG_FORMAT":   config.LogFormat,
	}

	if config.TracingEnabled {
		data["FEATHER_TRACING_ENABLED"] = "true"
		data["FEATHER_TRACING_ENDPOINT"] = config.TracingEndpoint
	}

	if config.KafkaEnabled {
		data["FEATHER_KAFKA_ENABLED"] = "true"
		data["FEATHER_KAFKA_BROKERS"] = config.KafkaBrokers
	}

	// Add extra env vars
	for k, v := range config.ExtraEnv {
		data[k] = v
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    r.labels(store),
			OwnerReferences: []metav1.OwnerReference{
				r.ownerRef(store),
			},
		},
		Data: data,
	}

	_, err := r.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = r.client.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	_, err = r.client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) ensureService(ctx context.Context, store *FeatureStore) error {
	name := store.ObjectMeta.Name
	namespace := store.ObjectMeta.Namespace
	if namespace == "" {
		namespace = r.namespace
	}

	config := store.Spec.Config

	httpPort := config.HTTPPort
	if httpPort == 0 {
		httpPort = 8080
	}
	grpcPort := config.GRPCPort
	if grpcPort == 0 {
		grpcPort = 50051
	}
	metricsPort := config.MetricsPort
	if metricsPort == 0 {
		metricsPort = 9090
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    r.labels(store),
			OwnerReferences: []metav1.OwnerReference{
				r.ownerRef(store),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: r.selectorLabels(store),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       httpPort,
					TargetPort: intstr.FromInt(int(httpPort)),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "grpc",
					Port:       grpcPort,
					TargetPort: intstr.FromInt(int(grpcPort)),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "metrics",
					Port:       metricsPort,
					TargetPort: intstr.FromInt(int(metricsPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	_, err := r.client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = r.client.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	_, err = r.client.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) ensureStatefulSet(ctx context.Context, store *FeatureStore) error {
	name := store.ObjectMeta.Name
	namespace := store.ObjectMeta.Namespace
	if namespace == "" {
		namespace = r.namespace
	}

	replicas := store.Spec.Replicas
	if replicas == 0 {
		replicas = 1
	}

	image := store.Spec.Image
	if image == "" {
		image = "feather/feather:" + store.Spec.Version
	}

	config := store.Spec.Config
	resources := store.Spec.Resources

	// Build resource requirements
	resourceReqs := corev1.ResourceRequirements{}
	if resources.CPURequest != "" || resources.MemoryRequest != "" {
		resourceReqs.Requests = corev1.ResourceList{}
		if resources.CPURequest != "" {
			resourceReqs.Requests[corev1.ResourceCPU] = resource.MustParse(resources.CPURequest)
		}
		if resources.MemoryRequest != "" {
			resourceReqs.Requests[corev1.ResourceMemory] = resource.MustParse(resources.MemoryRequest)
		}
	}
	if resources.CPULimit != "" || resources.MemoryLimit != "" {
		resourceReqs.Limits = corev1.ResourceList{}
		if resources.CPULimit != "" {
			resourceReqs.Limits[corev1.ResourceCPU] = resource.MustParse(resources.CPULimit)
		}
		if resources.MemoryLimit != "" {
			resourceReqs.Limits[corev1.ResourceMemory] = resource.MustParse(resources.MemoryLimit)
		}
	}

	httpPort := config.HTTPPort
	if httpPort == 0 {
		httpPort = 8080
	}
	grpcPort := config.GRPCPort
	if grpcPort == 0 {
		grpcPort = 50051
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    r.labels(store),
			OwnerReferences: []metav1.OwnerReference{
				r.ownerRef(store),
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: name,
			Selector: &metav1.LabelSelector{
				MatchLabels: r.selectorLabels(store),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: r.labels(store),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      "feather",
							Image:     image,
							Resources: resourceReqs,
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: httpPort},
								{Name: "grpc", ContainerPort: grpcPort},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: name + "-config",
										},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/live",
										Port: intstr.FromInt(int(httpPort)),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(int(httpPort)),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}

	// Add volume claim template if warm tier storage is configured
	if store.Spec.Storage.WarmTier.Size != "" {
		storageClass := store.Spec.Storage.WarmTier.StorageClass
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: &storageClass,
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(store.Spec.Storage.WarmTier.Size),
						},
					},
				},
			},
		}

		// Add volume mount
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/var/lib/feather/data",
			},
		}
	}

	// Add anti-affinity if HA is enabled
	if store.Spec.HighAvailability != nil && store.Spec.HighAvailability.AntiAffinity {
		sts.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: r.selectorLabels(store),
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		}
	}

	_, err := r.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = r.client.AppsV1().StatefulSets(namespace).Create(ctx, sts, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	_, err = r.client.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) ensureHPA(ctx context.Context, store *FeatureStore) error {
	name := store.ObjectMeta.Name
	namespace := store.ObjectMeta.Namespace
	if namespace == "" {
		namespace = r.namespace
	}

	autoscaling := store.Spec.Autoscaling
	if autoscaling == nil || !autoscaling.Enabled {
		// Delete HPA if it exists but autoscaling is disabled
		err := r.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	minReplicas := autoscaling.MinReplicas
	if minReplicas == 0 {
		minReplicas = 1
	}
	maxReplicas := autoscaling.MaxReplicas
	if maxReplicas == 0 {
		maxReplicas = 10
	}
	targetCPU := autoscaling.TargetCPUUtilization
	if targetCPU == 0 {
		targetCPU = 80
	}
	targetMemory := autoscaling.TargetMemoryUtilization
	if targetMemory == 0 {
		targetMemory = 80
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    r.labels(store),
			OwnerReferences: []metav1.OwnerReference{
				r.ownerRef(store),
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &targetCPU,
						},
					},
				},
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &targetMemory,
						},
					},
				},
			},
		},
	}

	// Add scale-down behavior for stability
	hpa.Spec.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{
		ScaleDown: &autoscalingv2.HPAScalingRules{
			StabilizationWindowSeconds: int32Ptr(300), // 5 minutes
			Policies: []autoscalingv2.HPAScalingPolicy{
				{
					Type:          autoscalingv2.PercentScalingPolicy,
					Value:         10,
					PeriodSeconds: 60,
				},
			},
		},
		ScaleUp: &autoscalingv2.HPAScalingRules{
			StabilizationWindowSeconds: int32Ptr(0),
			Policies: []autoscalingv2.HPAScalingPolicy{
				{
					Type:          autoscalingv2.PercentScalingPolicy,
					Value:         100,
					PeriodSeconds: 15,
				},
				{
					Type:          autoscalingv2.PodsScalingPolicy,
					Value:         4,
					PeriodSeconds: 15,
				},
			},
			SelectPolicy: selectPolicyPtr(autoscalingv2.MaxChangePolicySelect),
		},
	}

	_, err := r.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = r.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Create(ctx, hpa, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	_, err = r.client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(ctx, hpa, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) ensurePDB(ctx context.Context, store *FeatureStore) error {
	name := store.ObjectMeta.Name
	namespace := store.ObjectMeta.Namespace
	if namespace == "" {
		namespace = r.namespace
	}

	ha := store.Spec.HighAvailability
	if ha == nil || ha.PodDisruptionBudget == nil {
		// Delete PDB if it exists but HA is disabled
		err := r.client.PolicyV1().PodDisruptionBudgets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	pdbSpec := ha.PodDisruptionBudget

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    r.labels(store),
			OwnerReferences: []metav1.OwnerReference{
				r.ownerRef(store),
			},
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: r.selectorLabels(store),
			},
		},
	}

	// Set either MinAvailable or MaxUnavailable (not both)
	if pdbSpec.MinAvailable != nil {
		minAvailable := intstr.FromInt32(*pdbSpec.MinAvailable)
		pdb.Spec.MinAvailable = &minAvailable
	} else if pdbSpec.MaxUnavailable != nil {
		maxUnavailable := intstr.FromInt32(*pdbSpec.MaxUnavailable)
		pdb.Spec.MaxUnavailable = &maxUnavailable
	} else {
		// Default to maxUnavailable=1
		maxUnavailable := intstr.FromInt(1)
		pdb.Spec.MaxUnavailable = &maxUnavailable
	}

	_, err := r.client.PolicyV1().PodDisruptionBudgets(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = r.client.PolicyV1().PodDisruptionBudgets(namespace).Create(ctx, pdb, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	_, err = r.client.PolicyV1().PodDisruptionBudgets(namespace).Update(ctx, pdb, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) ensureServiceMonitor(ctx context.Context, store *FeatureStore) error {
	name := store.ObjectMeta.Name
	namespace := store.ObjectMeta.Namespace
	if namespace == "" {
		namespace = r.namespace
	}

	monitoring := store.Spec.Monitoring
	if monitoring == nil || !monitoring.ServiceMonitor {
		// Delete ServiceMonitor if it exists but monitoring is disabled
		if r.dynamicClient != nil {
			gvr := schema.GroupVersionResource{
				Group:    "monitoring.coreos.com",
				Version:  "v1",
				Resource: "servicemonitors",
			}
			err := r.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	if r.dynamicClient == nil {
		// No dynamic client available, skip ServiceMonitor creation
		return nil
	}

	config := store.Spec.Config
	metricsPort := config.MetricsPort
	if metricsPort == 0 {
		metricsPort = 9090
	}

	scrapeInterval := monitoring.ScrapeInterval
	if scrapeInterval == "" {
		scrapeInterval = "30s"
	}

	// Build the ServiceMonitor as unstructured
	sm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    r.labelsAsInterface(store),
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         "feather.io/v1alpha1",
						"kind":               "FeatureStore",
						"name":               store.ObjectMeta.Name,
						"uid":                string(store.ObjectMeta.UID),
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": r.selectorLabelsAsInterface(store),
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"port":     "metrics",
						"interval": scrapeInterval,
						"path":     "/metrics",
					},
				},
				"namespaceSelector": map[string]interface{}{
					"matchNames": []interface{}{namespace},
				},
			},
		},
	}

	gvr := schema.GroupVersionResource{
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "servicemonitors",
	}

	_, err := r.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = r.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, sm, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	_, err = r.dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, sm, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) labelsAsInterface(store *FeatureStore) map[string]interface{} {
	return map[string]interface{}{
		"app.kubernetes.io/name":       "feather",
		"app.kubernetes.io/instance":   store.ObjectMeta.Name,
		"app.kubernetes.io/version":    store.Spec.Version,
		"app.kubernetes.io/component":  "feature-store",
		"app.kubernetes.io/managed-by": "feather-operator",
	}
}

func (r *Reconciler) selectorLabelsAsInterface(store *FeatureStore) map[string]interface{} {
	return map[string]interface{}{
		"app.kubernetes.io/name":     "feather",
		"app.kubernetes.io/instance": store.ObjectMeta.Name,
	}
}

func (r *Reconciler) labels(store *FeatureStore) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "feather",
		"app.kubernetes.io/instance":   store.ObjectMeta.Name,
		"app.kubernetes.io/version":    store.Spec.Version,
		"app.kubernetes.io/component":  "feature-store",
		"app.kubernetes.io/managed-by": "feather-operator",
	}
}

func (r *Reconciler) selectorLabels(store *FeatureStore) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "feather",
		"app.kubernetes.io/instance": store.ObjectMeta.Name,
	}
}

func (r *Reconciler) ownerRef(store *FeatureStore) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "feather.io/v1alpha1",
		Kind:       "FeatureStore",
		Name:       store.ObjectMeta.Name,
		UID:        store.ObjectMeta.UID,
		Controller: boolPtr(true),
	}
}

// ReconcileFeatureGroup reconciles a FeatureGroup resource.
func (r *Reconciler) ReconcileFeatureGroup(ctx context.Context, group *FeatureGroup) ReconcileResult {
	// Update status
	group.Status.Phase = PhaseRunning
	group.Status.FeatureCount = len(group.Spec.Features)
	now := metav1.Now()
	group.Status.LastUpdated = &now

	return ReconcileResult{RequeueAfter: time.Minute}
}

// ReconcileFeatureView reconciles a FeatureView resource.
func (r *Reconciler) ReconcileFeatureView(ctx context.Context, view *FeatureView) ReconcileResult {
	// Update status
	view.Status.Phase = PhaseRunning

	// Handle materialization scheduling
	if view.Spec.Materialization != nil && view.Spec.Materialization.Enabled {
		// Would schedule materialization job
	}

	return ReconcileResult{RequeueAfter: time.Minute}
}

// Helper functions

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

func selectPolicyPtr(p autoscalingv2.ScalingPolicySelect) *autoscalingv2.ScalingPolicySelect {
	return &p
}
