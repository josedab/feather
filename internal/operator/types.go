package operator

import (
	"time"
)

// FeatureStore represents a Feather feature store deployment.
type FeatureStore struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       FeatureStoreSpec   `json:"spec"`
	Status     FeatureStoreStatus `json:"status,omitempty"`
}

// FeatureStoreSpec defines the desired state of a FeatureStore.
type FeatureStoreSpec struct {
	// Replicas is the number of feature store instances.
	Replicas int32 `json:"replicas"`

	// Image is the container image to use.
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies when to pull the image.
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// Resources defines compute resources.
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Storage configures persistent storage.
	Storage StorageSpec `json:"storage,omitempty"`

	// Config contains feature store configuration.
	Config FeatureStoreConfig `json:"config,omitempty"`

	// Autoscaling configures automatic scaling.
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// ServiceAccount is the service account to use.
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// NodeSelector for pod placement.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod scheduling.
	Tolerations []Toleration `json:"tolerations,omitempty"`

	// Affinity rules for pod placement.
	Affinity *Affinity `json:"affinity,omitempty"`
}

// FeatureStoreConfig contains feature store-specific configuration.
type FeatureStoreConfig struct {
	// HTTPPort is the HTTP server port.
	HTTPPort int32 `json:"httpPort,omitempty"`

	// GRPCPort is the gRPC server port.
	GRPCPort int32 `json:"grpcPort,omitempty"`

	// MetricsPort is the metrics server port.
	MetricsPort int32 `json:"metricsPort,omitempty"`

	// HotTierMaxMemory is the hot tier memory limit.
	HotTierMaxMemory string `json:"hotTierMaxMemory,omitempty"`

	// WarmTierPath is the warm tier storage path.
	WarmTierPath string `json:"warmTierPath,omitempty"`

	// KafkaEnabled enables Kafka ingestion.
	KafkaEnabled bool `json:"kafkaEnabled,omitempty"`

	// KafkaBrokers is the Kafka broker addresses.
	KafkaBrokers []string `json:"kafkaBrokers,omitempty"`

	// TracingEnabled enables distributed tracing.
	TracingEnabled bool `json:"tracingEnabled,omitempty"`

	// TracingEndpoint is the OTLP endpoint.
	TracingEndpoint string `json:"tracingEndpoint,omitempty"`

	// LogLevel sets the logging level.
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat sets the log format (json/text).
	LogFormat string `json:"logFormat,omitempty"`
}

// StorageSpec defines storage configuration.
type StorageSpec struct {
	// StorageClassName is the storage class to use.
	StorageClassName string `json:"storageClassName,omitempty"`

	// Size is the storage size (e.g., "10Gi").
	Size string `json:"size,omitempty"`

	// AccessMode is the volume access mode.
	AccessMode string `json:"accessMode,omitempty"`
}

// AutoscalingSpec defines autoscaling behavior.
type AutoscalingSpec struct {
	// Enabled enables autoscaling.
	Enabled bool `json:"enabled,omitempty"`

	// MinReplicas is the minimum number of replicas.
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas.
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// TargetCPUUtilization is the target CPU percentage.
	TargetCPUUtilization int32 `json:"targetCPUUtilization,omitempty"`

	// TargetMemoryUtilization is the target memory percentage.
	TargetMemoryUtilization int32 `json:"targetMemoryUtilization,omitempty"`

	// ScaleUpStabilization is the scale-up window.
	ScaleUpStabilization int32 `json:"scaleUpStabilization,omitempty"`

	// ScaleDownStabilization is the scale-down window.
	ScaleDownStabilization int32 `json:"scaleDownStabilization,omitempty"`
}

// FeatureStoreStatus defines the observed state of a FeatureStore.
type FeatureStoreStatus struct {
	// Phase is the current lifecycle phase.
	Phase FeatureStorePhase `json:"phase,omitempty"`

	// Conditions represent the current conditions.
	Conditions []Condition `json:"conditions,omitempty"`

	// ReadyReplicas is the number of ready replicas.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the number of available replicas.
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ObservedGeneration is the generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastUpdateTime is when the status was last updated.
	LastUpdateTime time.Time `json:"lastUpdateTime,omitempty"`

	// Endpoints contains service endpoints.
	Endpoints EndpointStatus `json:"endpoints,omitempty"`

	// Stats contains runtime statistics.
	Stats RuntimeStats `json:"stats,omitempty"`
}

// FeatureStorePhase represents the lifecycle phase.
type FeatureStorePhase string

// FeatureStorePhase values represent the lifecycle state.
const (
	PhasePending     FeatureStorePhase = "Pending"
	PhaseCreating    FeatureStorePhase = "Creating"
	PhaseRunning     FeatureStorePhase = "Running"
	PhaseUpdating    FeatureStorePhase = "Updating"
	PhaseScaling     FeatureStorePhase = "Scaling"
	PhaseFailed      FeatureStorePhase = "Failed"
	PhaseTerminating FeatureStorePhase = "Terminating"
)

// Condition represents a condition of a resource.
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// EndpointStatus contains service endpoint information.
type EndpointStatus struct {
	HTTP    string `json:"http,omitempty"`
	GRPC    string `json:"grpc,omitempty"`
	Metrics string `json:"metrics,omitempty"`
}

// RuntimeStats contains runtime statistics.
type RuntimeStats struct {
	TotalFeatures    int64 `json:"totalFeatures,omitempty"`
	TotalEntities    int64 `json:"totalEntities,omitempty"`
	RequestsPerSec   int64 `json:"requestsPerSec,omitempty"`
	AvgLatencyMs     int64 `json:"avgLatencyMs,omitempty"`
	CacheHitRate     int64 `json:"cacheHitRate,omitempty"`
	StorageUsedBytes int64 `json:"storageUsedBytes,omitempty"`
}

// FeatureGroup represents a feature group CRD.
type FeatureGroup struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       FeatureGroupSpec   `json:"spec"`
	Status     FeatureGroupStatus `json:"status,omitempty"`
}

// FeatureGroupSpec defines the desired state of a FeatureGroup.
type FeatureGroupSpec struct {
	// FeatureStoreRef references the parent FeatureStore.
	FeatureStoreRef string `json:"featureStoreRef"`

	// Description describes the feature group.
	Description string `json:"description,omitempty"`

	// EntityType is the entity type (e.g., "user", "product").
	EntityType string `json:"entityType"`

	// Features defines the features in this group.
	Features []FeatureSpec `json:"features"`

	// Source defines the data source.
	Source *SourceSpec `json:"source,omitempty"`

	// TTL is the feature time-to-live.
	TTL string `json:"ttl,omitempty"`

	// Owner is the owner of this feature group.
	Owner string `json:"owner,omitempty"`

	// Tags for categorization.
	Tags map[string]string `json:"tags,omitempty"`
}

// FeatureSpec defines a single feature.
type FeatureSpec struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Nullable    bool   `json:"nullable,omitempty"`
	Validator   string `json:"validator,omitempty"`
}

// SourceSpec defines a data source for a feature group.
type SourceSpec struct {
	Type   string            `json:"type"`
	Config map[string]string `json:"config,omitempty"`
}

// FeatureGroupStatus defines the observed state of a FeatureGroup.
type FeatureGroupStatus struct {
	Phase              string      `json:"phase,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	FeatureCount       int         `json:"featureCount,omitempty"`
	EntityCount        int64       `json:"entityCount,omitempty"`
	LastSyncTime       time.Time   `json:"lastSyncTime,omitempty"`
}

// FeatureView represents a materialized feature view CRD.
type FeatureView struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`
	Spec       FeatureViewSpec   `json:"spec"`
	Status     FeatureViewStatus `json:"status,omitempty"`
}

// FeatureViewSpec defines the desired state of a FeatureView.
type FeatureViewSpec struct {
	// FeatureStoreRef references the parent FeatureStore.
	FeatureStoreRef string `json:"featureStoreRef"`

	// Description describes the feature view.
	Description string `json:"description,omitempty"`

	// Features lists features to include from various groups.
	Features []FeatureRef `json:"features"`

	// Transformations defines transformations to apply.
	Transformations []TransformSpec `json:"transformations,omitempty"`

	// Schedule for materialization.
	Schedule string `json:"schedule,omitempty"`

	// TTL for materialized data.
	TTL string `json:"ttl,omitempty"`
}

// FeatureRef references a feature in a feature group.
type FeatureRef struct {
	Group   string `json:"group"`
	Feature string `json:"feature"`
	Alias   string `json:"alias,omitempty"`
}

// TransformSpec defines a transformation.
type TransformSpec struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config,omitempty"`
}

// FeatureViewStatus defines the observed state of a FeatureView.
type FeatureViewStatus struct {
	Phase                   string      `json:"phase,omitempty"`
	Conditions              []Condition `json:"conditions,omitempty"`
	ObservedGeneration      int64       `json:"observedGeneration,omitempty"`
	LastMaterializationTime time.Time   `json:"lastMaterializationTime,omitempty"`
	NextMaterializationTime time.Time   `json:"nextMaterializationTime,omitempty"`
	RowCount                int64       `json:"rowCount,omitempty"`
}

// TypeMeta contains Kubernetes-style type metadata.
type TypeMeta struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

// ObjectMeta contains standard object metadata.
type ObjectMeta struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	UID               string            `json:"uid,omitempty"`
	Generation        int64             `json:"generation,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp,omitempty"`
	DeletionTimestamp *time.Time        `json:"deletionTimestamp,omitempty"`
	Finalizers        []string          `json:"finalizers,omitempty"`
	OwnerReferences   []OwnerReference  `json:"ownerReferences,omitempty"`
}

// OwnerReference defines an owner reference.
type OwnerReference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Controller *bool  `json:"controller,omitempty"`
}

// ResourceRequirements defines compute resources.
type ResourceRequirements struct {
	Limits   ResourceList `json:"limits,omitempty"`
	Requests ResourceList `json:"requests,omitempty"`
}

// ResourceList is a map of resource names to quantities.
type ResourceList map[string]string

// Toleration defines a pod toleration.
type Toleration struct {
	Key               string `json:"key,omitempty"`
	Operator          string `json:"operator,omitempty"`
	Value             string `json:"value,omitempty"`
	Effect            string `json:"effect,omitempty"`
	TolerationSeconds *int64 `json:"tolerationSeconds,omitempty"`
}

// Affinity defines pod affinity rules.
type Affinity struct {
	NodeAffinity    *NodeAffinity    `json:"nodeAffinity,omitempty"`
	PodAffinity     *PodAffinity     `json:"podAffinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
}

// NodeAffinity defines node affinity rules.
type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// NodeSelector defines node selector requirements.
type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms"`
}

// NodeSelectorTerm defines a node selector term.
type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `json:"matchFields,omitempty"`
}

// NodeSelectorRequirement defines a node selector requirement.
type NodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// PreferredSchedulingTerm defines a preferred scheduling term.
type PreferredSchedulingTerm struct {
	Weight     int32            `json:"weight"`
	Preference NodeSelectorTerm `json:"preference"`
}

// PodAffinity defines pod affinity rules.
type PodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAntiAffinity defines pod anti-affinity rules.
type PodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAffinityTerm defines a pod affinity term.
type PodAffinityTerm struct {
	LabelSelector *LabelSelector `json:"labelSelector,omitempty"`
	TopologyKey   string         `json:"topologyKey"`
}

// WeightedPodAffinityTerm defines a weighted pod affinity term.
type WeightedPodAffinityTerm struct {
	Weight          int32           `json:"weight"`
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm"`
}

// LabelSelector defines a label selector.
type LabelSelector struct {
	MatchLabels      map[string]string          `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement defines a label selector requirement.
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}
