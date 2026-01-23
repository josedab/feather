// Package crd defines Kubernetes Custom Resource Definition types for the Feather operator.
package crd

import "time"

// TypeMeta describes the API version and kind of a Kubernetes resource.
type TypeMeta struct {
	APIVersion string
	Kind       string
}

// ObjectMeta contains standard metadata for Kubernetes objects.
type ObjectMeta struct {
	Name        string
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
}

// FeatherCluster represents a managed Feather deployment.
type FeatherCluster struct {
	TypeMeta
	ObjectMeta
	Spec   FeatherClusterSpec
	Status FeatherClusterStatus
}

// FeatherClusterSpec defines the desired state of a FeatherCluster.
type FeatherClusterSpec struct {
	Replicas int
	Version  string
	Image    string
	Storage  StorageSpec
	Config   map[string]string
}

// StorageSpec configures storage tiers.
type StorageSpec struct {
	HotMemoryGB  int
	WarmDiskGB   int
	StorageClass string
}

// FeatherClusterStatus reflects the observed state of a FeatherCluster.
type FeatherClusterStatus struct {
	Phase          string
	ReadyReplicas  int
	CurrentVersion string
	Conditions     []Condition
}

// Condition describes an aspect of the cluster's current state.
type Condition struct {
	Type               string
	Status             string
	Reason             string
	Message            string
	LastTransitionTime time.Time
}

// FeatureGroupSpec defines a feature group custom resource.
type FeatureGroupSpec struct {
	Name        string
	EntityType  string
	Description string
	TTL         string
	Features    []FeatureFieldSpec
	Owner       string
}

// FeatureFieldSpec describes a single feature field within a group.
type FeatureFieldSpec struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// FeatureSLASpec defines SLA constraints for a feature group.
type FeatureSLASpec struct {
	FeatureGroup        string
	MaxLatencyMs        int
	MaxStalenessMinutes int
	MinAvailability     float64
}

// DefaultFeatherClusterSpec returns a FeatherClusterSpec with sensible defaults.
func DefaultFeatherClusterSpec() FeatherClusterSpec {
	return FeatherClusterSpec{
		Replicas: 1,
		Version:  "latest",
		Image:    "ghcr.io/feather-store/feather:latest",
		Storage: StorageSpec{
			HotMemoryGB:  4,
			WarmDiskGB:   50,
			StorageClass: "standard",
		},
		Config: map[string]string{},
	}
}

// ValidateFeatureGroup checks a FeatureGroupSpec for common errors.
func ValidateFeatureGroup(fg FeatureGroupSpec) []string {
	var errs []string
	if fg.Name == "" {
		errs = append(errs, "name is required")
	}
	if fg.EntityType == "" {
		errs = append(errs, "entityType is required")
	}
	if len(fg.Features) == 0 {
		errs = append(errs, "at least one feature is required")
	}
	for _, f := range fg.Features {
		if f.Name == "" {
			errs = append(errs, "feature name is required")
		}
		if f.Type == "" {
			errs = append(errs, "feature type is required for "+f.Name)
		}
	}
	return errs
}
