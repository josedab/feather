package operator

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FeatureStoreSpec defines the desired state of FeatureStore.
type FeatureStoreSpec struct {
	// Replicas is the number of feature store instances
	Replicas int32 `json:"replicas"`

	// Image is the container image to use
	Image string `json:"image,omitempty"`

	// Version is the Feather version to deploy
	Version string `json:"version,omitempty"`

	// Storage configures persistent storage
	Storage StorageSpec `json:"storage,omitempty"`

	// Resources defines compute resources
	Resources ResourceSpec `json:"resources,omitempty"`

	// Config holds feature store configuration
	Config FeatherConfig `json:"config,omitempty"`

	// Autoscaling configures horizontal pod autoscaling
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// HighAvailability configures HA mode
	HighAvailability *HASpec `json:"highAvailability,omitempty"`

	// Monitoring configures observability
	Monitoring *MonitoringSpec `json:"monitoring,omitempty"`

	// Backup configures automated backups
	Backup *BackupSpec `json:"backup,omitempty"`
}

// StorageSpec defines storage configuration.
type StorageSpec struct {
	// HotTier configures in-memory storage
	HotTier HotTierSpec `json:"hotTier,omitempty"`

	// WarmTier configures persistent storage
	WarmTier WarmTierSpec `json:"warmTier,omitempty"`
}

// HotTierSpec defines hot tier (memory) configuration.
type HotTierSpec struct {
	// MaxMemory is the maximum memory for hot tier (e.g., "4Gi")
	MaxMemory string `json:"maxMemory,omitempty"`

	// TTL is the default TTL for hot tier entries
	TTL string `json:"ttl,omitempty"`
}

// WarmTierSpec defines warm tier (disk) configuration.
type WarmTierSpec struct {
	// StorageClass is the Kubernetes storage class to use
	StorageClass string `json:"storageClass,omitempty"`

	// Size is the volume size (e.g., "100Gi")
	Size string `json:"size,omitempty"`

	// Path is the data directory path
	Path string `json:"path,omitempty"`
}

// ResourceSpec defines compute resources.
type ResourceSpec struct {
	// CPU request and limit (e.g., "500m", "2")
	CPURequest string `json:"cpuRequest,omitempty"`
	CPULimit   string `json:"cpuLimit,omitempty"`

	// Memory request and limit (e.g., "1Gi", "8Gi")
	MemoryRequest string `json:"memoryRequest,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

// FeatherConfig holds Feather-specific configuration.
type FeatherConfig struct {
	// HTTPPort is the HTTP server port
	HTTPPort int32 `json:"httpPort,omitempty"`

	// GRPCPort is the gRPC server port
	GRPCPort int32 `json:"grpcPort,omitempty"`

	// MetricsPort is the Prometheus metrics port
	MetricsPort int32 `json:"metricsPort,omitempty"`

	// LogLevel sets the logging level
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat sets the log format (json, text)
	LogFormat string `json:"logFormat,omitempty"`

	// TracingEnabled enables OpenTelemetry tracing
	TracingEnabled bool `json:"tracingEnabled,omitempty"`

	// TracingEndpoint is the OTLP endpoint
	TracingEndpoint string `json:"tracingEndpoint,omitempty"`

	// KafkaEnabled enables Kafka ingestion
	KafkaEnabled bool `json:"kafkaEnabled,omitempty"`

	// KafkaBrokers is the Kafka broker list
	KafkaBrokers string `json:"kafkaBrokers,omitempty"`

	// ExtraEnv holds additional environment variables
	ExtraEnv map[string]string `json:"extraEnv,omitempty"`
}

// AutoscalingSpec defines autoscaling configuration.
type AutoscalingSpec struct {
	// Enabled enables HPA
	Enabled bool `json:"enabled"`

	// MinReplicas is the minimum number of replicas
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// TargetCPUUtilization is the target CPU percentage
	TargetCPUUtilization int32 `json:"targetCPUUtilization,omitempty"`

	// TargetMemoryUtilization is the target memory percentage
	TargetMemoryUtilization int32 `json:"targetMemoryUtilization,omitempty"`

	// CustomMetrics defines custom metrics for scaling
	CustomMetrics []CustomMetricSpec `json:"customMetrics,omitempty"`
}

// CustomMetricSpec defines a custom metric for autoscaling.
type CustomMetricSpec struct {
	// Name is the metric name
	Name string `json:"name"`

	// TargetValue is the target value
	TargetValue string `json:"targetValue"`

	// Type is the metric type (Pods, Object, External)
	Type string `json:"type,omitempty"`
}

// HASpec defines high availability configuration.
type HASpec struct {
	// Enabled enables HA mode
	Enabled bool `json:"enabled"`

	// PodDisruptionBudget configures PDB
	PodDisruptionBudget *PDBSpec `json:"podDisruptionBudget,omitempty"`

	// TopologySpreadConstraints for zone spreading
	TopologySpread bool `json:"topologySpread,omitempty"`

	// AntiAffinity enables pod anti-affinity
	AntiAffinity bool `json:"antiAffinity,omitempty"`
}

// PDBSpec defines PodDisruptionBudget configuration.
type PDBSpec struct {
	// MinAvailable is the minimum available pods
	MinAvailable *int32 `json:"minAvailable,omitempty"`

	// MaxUnavailable is the maximum unavailable pods
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

// MonitoringSpec defines monitoring configuration.
type MonitoringSpec struct {
	// Enabled enables monitoring
	Enabled bool `json:"enabled"`

	// ServiceMonitor creates a Prometheus ServiceMonitor
	ServiceMonitor bool `json:"serviceMonitor,omitempty"`

	// ScrapeInterval is the Prometheus scrape interval (e.g., "30s")
	ScrapeInterval string `json:"scrapeInterval,omitempty"`

	// GrafanaDashboard creates Grafana dashboards
	GrafanaDashboard bool `json:"grafanaDashboard,omitempty"`

	// AlertRules creates PrometheusRule alerts
	AlertRules bool `json:"alertRules,omitempty"`
}

// BackupSpec defines backup configuration.
type BackupSpec struct {
	// Enabled enables automated backups
	Enabled bool `json:"enabled"`

	// Schedule is the cron schedule for backups
	Schedule string `json:"schedule,omitempty"`

	// Retention is how long to keep backups
	Retention string `json:"retention,omitempty"`

	// Storage configures backup storage
	Storage BackupStorageSpec `json:"storage,omitempty"`
}

// BackupStorageSpec defines backup storage configuration.
type BackupStorageSpec struct {
	// Type is the storage type (s3, gcs, azure, local)
	Type string `json:"type"`

	// Bucket is the bucket name
	Bucket string `json:"bucket,omitempty"`

	// Prefix is the object prefix
	Prefix string `json:"prefix,omitempty"`

	// SecretRef references credentials secret
	SecretRef string `json:"secretRef,omitempty"`
}

// FeatureStoreStatus defines the observed state of FeatureStore.
type FeatureStoreStatus struct {
	// Phase is the current phase (Pending, Running, Failed)
	Phase string `json:"phase"`

	// Replicas is the current number of replicas
	Replicas int32 `json:"replicas"`

	// ReadyReplicas is the number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas"`

	// Conditions represent the latest observations
	Conditions []Condition `json:"conditions,omitempty"`

	// LastBackup is the timestamp of the last backup
	LastBackup *metav1.Time `json:"lastBackup,omitempty"`

	// Version is the running version
	Version string `json:"version,omitempty"`

	// Endpoints contains service endpoints
	Endpoints EndpointStatus `json:"endpoints,omitempty"`
}

// Condition represents a condition of the resource.
type Condition struct {
	// Type is the condition type
	Type string `json:"type"`

	// Status is the condition status (True, False, Unknown)
	Status string `json:"status"`

	// Reason is a brief reason for the condition
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message
	Message string `json:"message,omitempty"`

	// LastTransitionTime is when the condition last changed
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// EndpointStatus contains service endpoint information.
type EndpointStatus struct {
	// HTTP is the HTTP endpoint
	HTTP string `json:"http,omitempty"`

	// GRPC is the gRPC endpoint
	GRPC string `json:"grpc,omitempty"`

	// Metrics is the metrics endpoint
	Metrics string `json:"metrics,omitempty"`
}

// FeatureStore is the Schema for the featurestores API.
type FeatureStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureStoreSpec   `json:"spec,omitempty"`
	Status FeatureStoreStatus `json:"status,omitempty"`
}

// FeatureStoreList contains a list of FeatureStore.
type FeatureStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FeatureStore `json:"items"`
}

// FeatureGroupSpec defines the desired state of FeatureGroup.
type FeatureGroupSpec struct {
	// Store references the parent FeatureStore
	Store string `json:"store"`

	// Name is the feature group name
	Name string `json:"name"`

	// Description describes the feature group
	Description string `json:"description,omitempty"`

	// Entity is the entity key field
	Entity string `json:"entity"`

	// Features defines the features in this group
	Features []FeatureSpec `json:"features"`

	// TTL is the feature TTL
	TTL string `json:"ttl,omitempty"`

	// Tags are metadata tags
	Tags map[string]string `json:"tags,omitempty"`

	// Owner is the feature group owner
	Owner string `json:"owner,omitempty"`
}

// FeatureSpec defines a single feature.
type FeatureSpec struct {
	// Name is the feature name
	Name string `json:"name"`

	// Type is the data type
	Type string `json:"type"`

	// Description describes the feature
	Description string `json:"description,omitempty"`

	// DefaultValue is the default value
	DefaultValue string `json:"defaultValue,omitempty"`

	// Validation rules
	Validation *ValidationSpec `json:"validation,omitempty"`
}

// ValidationSpec defines validation rules for a feature.
type ValidationSpec struct {
	// Required indicates the feature is required
	Required bool `json:"required,omitempty"`

	// Min is the minimum value (for numeric types)
	Min *float64 `json:"min,omitempty"`

	// Max is the maximum value (for numeric types)
	Max *float64 `json:"max,omitempty"`

	// Pattern is a regex pattern (for string types)
	Pattern string `json:"pattern,omitempty"`

	// Enum is a list of allowed values
	Enum []string `json:"enum,omitempty"`
}

// FeatureGroupStatus defines the observed state of FeatureGroup.
type FeatureGroupStatus struct {
	// Phase is the current phase
	Phase string `json:"phase"`

	// FeatureCount is the number of features
	FeatureCount int `json:"featureCount"`

	// LastUpdated is when the group was last updated
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`

	// Conditions represent the latest observations
	Conditions []Condition `json:"conditions,omitempty"`
}

// FeatureGroup is the Schema for the featuregroups API.
type FeatureGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureGroupSpec   `json:"spec,omitempty"`
	Status FeatureGroupStatus `json:"status,omitempty"`
}

// FeatureGroupList contains a list of FeatureGroup.
type FeatureGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FeatureGroup `json:"items"`
}

// FeatureViewSpec defines the desired state of FeatureView.
type FeatureViewSpec struct {
	// Store references the parent FeatureStore
	Store string `json:"store"`

	// Name is the view name
	Name string `json:"name"`

	// Description describes the view
	Description string `json:"description,omitempty"`

	// Sources defines the feature sources
	Sources []FeatureSourceSpec `json:"sources"`

	// Transformations defines transformations to apply
	Transformations []TransformationSpec `json:"transformations,omitempty"`

	// Materialization configures materialization
	Materialization *MaterializationSpec `json:"materialization,omitempty"`

	// Tags are metadata tags
	Tags map[string]string `json:"tags,omitempty"`
}

// FeatureSourceSpec defines a feature source.
type FeatureSourceSpec struct {
	// Group references a FeatureGroup
	Group string `json:"group"`

	// Features is the list of features to include
	Features []string `json:"features,omitempty"`

	// Alias renames features
	Alias map[string]string `json:"alias,omitempty"`
}

// TransformationSpec defines a transformation.
type TransformationSpec struct {
	// Name is the transformation name
	Name string `json:"name"`

	// Type is the transformation type
	Type string `json:"type"`

	// Expression is the transformation expression
	Expression string `json:"expression,omitempty"`

	// Config holds transformation config
	Config map[string]string `json:"config,omitempty"`
}

// MaterializationSpec defines materialization configuration.
type MaterializationSpec struct {
	// Enabled enables materialization
	Enabled bool `json:"enabled"`

	// Schedule is the cron schedule
	Schedule string `json:"schedule,omitempty"`

	// Mode is the materialization mode (incremental, full)
	Mode string `json:"mode,omitempty"`

	// Destination configures where to materialize
	Destination MaterializationDestSpec `json:"destination,omitempty"`
}

// MaterializationDestSpec defines materialization destination.
type MaterializationDestSpec struct {
	// Type is the destination type (online, offline, both)
	Type string `json:"type"`

	// OnlineStore configures online store materialization
	OnlineStore *OnlineStoreSpec `json:"onlineStore,omitempty"`

	// OfflineStore configures offline store materialization
	OfflineStore *OfflineStoreSpec `json:"offlineStore,omitempty"`
}

// OnlineStoreSpec defines online store configuration.
type OnlineStoreSpec struct {
	// TTL is the online store TTL
	TTL string `json:"ttl,omitempty"`
}

// OfflineStoreSpec defines offline store configuration.
type OfflineStoreSpec struct {
	// Format is the output format (parquet, csv)
	Format string `json:"format,omitempty"`

	// Path is the output path
	Path string `json:"path,omitempty"`
}

// FeatureViewStatus defines the observed state of FeatureView.
type FeatureViewStatus struct {
	// Phase is the current phase
	Phase string `json:"phase"`

	// LastMaterialization is when materialization last ran
	LastMaterialization *metav1.Time `json:"lastMaterialization,omitempty"`

	// NextMaterialization is when materialization will next run
	NextMaterialization *metav1.Time `json:"nextMaterialization,omitempty"`

	// Conditions represent the latest observations
	Conditions []Condition `json:"conditions,omitempty"`
}

// FeatureView is the Schema for the featureviews API.
type FeatureView struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FeatureViewSpec   `json:"spec,omitempty"`
	Status FeatureViewStatus `json:"status,omitempty"`
}

// FeatureViewList contains a list of FeatureView.
type FeatureViewList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FeatureView `json:"items"`
}

// Constants for phases and conditions.
const (
	PhasePending  = "Pending"
	PhaseRunning  = "Running"
	PhaseFailed   = "Failed"
	PhaseDeleting = "Deleting"

	ConditionTypeReady       = "Ready"
	ConditionTypeProgressing = "Progressing"
	ConditionTypeDegraded    = "Degraded"

	ConditionStatusTrue    = "True"
	ConditionStatusFalse   = "False"
	ConditionStatusUnknown = "Unknown"
)

// ReconcileResult holds the result of a reconciliation.
type ReconcileResult struct {
	Requeue      bool
	RequeueAfter time.Duration
	Error        error
}
