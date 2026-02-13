// Package domain defines the core domain types for the Feather feature store.
//
// This package contains the foundational types used throughout Feather including:
//   - Data types and validation specifications for features
//   - Feature groups and their schemas
//   - Request/response structures for the API layer
//   - Error types and codes
//
// # Feature Values
//
// Features are stored as [FeatureValue] instances which contain the actual value,
// a nanosecond-precision timestamp, and an optional version for optimistic locking:
//
//	value := &domain.FeatureValue{
//	    Value:     42,
//	    Timestamp: time.Now().UnixNano(),
//	    Version:   1,
//	}
//
// # Feature Groups
//
// Related features are organized into [FeatureGroup] instances which define
// the schema, TTL, and validation rules:
//
//	group := &domain.FeatureGroup{
//	    Name:       "user_engagement",
//	    EntityType: "user",
//	    TTL:        24 * time.Hour,
//	    Features: []domain.FeatureSpec{
//	        {Name: "click_count", DataType: DataTypeInt64},
//	        {Name: "purchase_total", DataType: DataTypeFloat64},
//	    },
//	}
//
// # API Responses
//
// All API responses use the [APIResponse] envelope which provides consistent
// error handling and metadata across all endpoints.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// DataType enumerates the supported feature data types in Feather.
//
// Each data type maps to a corresponding Go type for storage and validation:
//   - DataTypeInt64: int64
//   - DataTypeFloat64: float64
//   - DataTypeString: string
//   - DataTypeBool: bool
//   - DataTypeBytes: []byte
//   - DataTypeVector: []float32 (for embeddings)
//   - DataTypeTimestamp: time.Time (stored as RFC3339)
type DataType int

const (
	// DataTypeInt64 represents a 64-bit signed integer.
	DataTypeInt64 DataType = iota
	// DataTypeFloat64 represents a 64-bit IEEE 754 floating point number.
	DataTypeFloat64
	// DataTypeString represents a UTF-8 encoded string.
	DataTypeString
	// DataTypeBool represents a boolean value (true/false).
	DataTypeBool
	// DataTypeBytes represents arbitrary binary data (base64 encoded in JSON).
	DataTypeBytes
	// DataTypeVector represents a float32 array for ML embeddings.
	DataTypeVector
	// DataTypeTimestamp represents a point in time (RFC3339 in JSON, UnixNano internally).
	DataTypeTimestamp
)

func (d DataType) String() string {
	switch d {
	case DataTypeInt64:
		return "int64"
	case DataTypeFloat64:
		return "float64"
	case DataTypeString:
		return "string"
	case DataTypeBool:
		return "bool"
	case DataTypeBytes:
		return "bytes"
	case DataTypeVector:
		return "vector"
	case DataTypeTimestamp:
		return "timestamp"
	default:
		return "unknown"
	}
}

// ErrUnknownDataType indicates an unsupported feature data type.
var ErrUnknownDataType = errors.New("unknown data type")

// ParseDataType parses a data type string to DataType.
func ParseDataType(s string) (DataType, error) {
	switch s {
	case "int64":
		return DataTypeInt64, nil
	case "float64":
		return DataTypeFloat64, nil
	case "string":
		return DataTypeString, nil
	case "bool":
		return DataTypeBool, nil
	case "bytes":
		return DataTypeBytes, nil
	case "vector":
		return DataTypeVector, nil
	case "timestamp":
		return DataTypeTimestamp, nil
	default:
		return DataTypeString, fmt.Errorf("%w: %q", ErrUnknownDataType, s)
	}
}

// MarshalJSON marshals the data type to JSON.
func (d DataType) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON unmarshals the data type from JSON.
func (d *DataType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseDataType(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// AggFunction defines the available aggregation function types for sliding window computations.
//
// Aggregations are computed incrementally as data arrives, using pre-aggregated buckets
// to maintain O(buckets) computation complexity regardless of data volume.
type AggFunction string

const (
	// AggCount counts the number of values in the window.
	AggCount AggFunction = "count"
	// AggSum computes the sum of all values in the window.
	AggSum AggFunction = "sum"
	// AggAvg computes the arithmetic mean of values in the window.
	AggAvg AggFunction = "avg"
	// AggMin returns the minimum value in the window.
	AggMin AggFunction = "min"
	// AggMax returns the maximum value in the window.
	AggMax AggFunction = "max"
	// AggLast returns the most recently added value in the window.
	AggLast AggFunction = "last"
)

// AggregationSpec defines a sliding window aggregation configuration.
//
// Aggregations are computed over a time window that slides at a configurable interval.
// For example, a 1-hour window sliding every 1 minute computes statistics over
// the most recent 60 minutes of data, updated every minute.
//
// Example:
//
//	spec := &AggregationSpec{
//	    Function: AggCount,
//	    Window:   time.Hour,
//	    SlideBy:  time.Minute,
//	}
type AggregationSpec struct {
	// Function specifies which aggregation to compute (count, sum, avg, min, max, last).
	Function AggFunction `json:"function" yaml:"function"`
	// Window is the time duration over which to aggregate (e.g., 1h, 24h, 7d).
	Window time.Duration `json:"window" yaml:"window"`
	// SlideBy is the interval at which the window advances. Defaults to Window if not set.
	SlideBy time.Duration `json:"slide_by,omitempty" yaml:"slide_by,omitempty"`
}

// ValidationSpec defines validation rules applied to feature values during ingestion.
//
// Multiple validation rules can be combined. All specified rules must pass for
// the value to be accepted.
type ValidationSpec struct {
	// Min specifies the minimum allowed value (inclusive) for numeric types.
	Min *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	// Max specifies the maximum allowed value (inclusive) for numeric types.
	Max *float64 `json:"max,omitempty" yaml:"max,omitempty"`
	// NotNull when true requires the value to be non-nil.
	NotNull bool `json:"not_null" yaml:"not_null"`
	// OneOf restricts string values to a predefined set of allowed values.
	OneOf []string `json:"one_of,omitempty" yaml:"one_of,omitempty"`
	// Regex specifies a regular expression pattern that string values must match.
	Regex string `json:"regex,omitempty" yaml:"regex,omitempty"`
}

// FeatureSpec defines the schema for a single feature within a feature group.
//
// Each feature has a name, data type, and optional configuration for validation
// and aggregation. Features are uniquely identified by name within their group.
type FeatureSpec struct {
	// Name is the unique identifier for this feature within its group.
	Name string `json:"name" yaml:"name"`
	// DataType specifies the value type (int64, float64, string, bool, bytes, vector, timestamp).
	DataType DataType `json:"data_type" yaml:"data_type"`
	// Dimensions specifies the shape for vector/tensor types (e.g., [384] for embeddings).
	Dimensions []int `json:"dimensions,omitempty" yaml:"dimensions,omitempty"`
	// Default is the value returned when no value exists for an entity.
	Default interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	// Aggregation configures sliding window aggregation for this feature.
	Aggregation *AggregationSpec `json:"aggregation,omitempty" yaml:"aggregation,omitempty"`
	// Validation defines rules for validating incoming values.
	Validation *ValidationSpec `json:"validation,omitempty" yaml:"validation,omitempty"`
}

// FeatureGroup defines a collection of related features that share an entity type.
//
// Feature groups are the primary organizational unit in Feather. All features in a group
// share the same entity type (e.g., "user", "product", "transaction") and TTL settings.
//
// Example:
//
//	group := &FeatureGroup{
//	    Name:       "user_engagement",
//	    EntityType: "user",
//	    TTL:        30 * 24 * time.Hour, // 30 days
//	    Features: []FeatureSpec{
//	        {Name: "click_count", DataType: DataTypeInt64},
//	        {Name: "last_login", DataType: DataTypeTimestamp},
//	    },
//	}
type FeatureGroup struct {
	// Name uniquely identifies this feature group.
	Name string `json:"name" yaml:"name"`
	// EntityType specifies the type of entity these features describe (e.g., "user", "product").
	EntityType string `json:"entity_type" yaml:"entity_type"`
	// Description provides human-readable documentation for the group.
	Description string `json:"description" yaml:"description"`
	// Features lists all feature specifications in this group.
	Features []FeatureSpec `json:"features" yaml:"features"`
	// TTL specifies how long feature values are retained before expiration.
	TTL time.Duration `json:"ttl" yaml:"ttl"`
	// Tags provides arbitrary key-value metadata for organization and filtering.
	Tags map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeatureValue represents a stored feature value with metadata.
//
// Each feature value includes the actual value, a nanosecond-precision timestamp
// for point-in-time queries, and a version number for optimistic concurrency control.
type FeatureValue struct {
	// Value holds the actual feature value. Type depends on the feature's DataType.
	Value interface{} `json:"value"`
	// Timestamp is the Unix timestamp in nanoseconds when this value was recorded.
	Timestamp int64 `json:"timestamp"`
	// Version is used for optimistic locking. Higher versions overwrite lower versions.
	Version int64 `json:"version"`
}

// FeatureUpdate represents an incoming feature update from ingestion sources.
//
// Updates can set multiple features for a single entity in one operation.
// If Timestamp is zero, the current time is used.
type FeatureUpdate struct {
	// EntityKey uniquely identifies the entity (e.g., "user:123", "product:abc").
	EntityKey string `json:"entity_key"`
	// Features maps feature names to their new values.
	Features map[string]interface{} `json:"features"`
	// Timestamp is the Unix nanosecond timestamp for this update. Defaults to now.
	Timestamp int64 `json:"timestamp"`
	// Version is used for optimistic concurrency. Only updates with higher versions are applied.
	Version int64 `json:"version"`
}

// GetFeaturesRequest defines the request format.
type GetFeaturesRequest struct {
	Entities []string `json:"entities"`
	Features []string `json:"features"`
}

// GetFeaturesResponse returns feature vectors.
type GetFeaturesResponse struct {
	Entities map[string]*EntityFeatures `json:"entities"`
}

// EntityFeatures contains features for one entity.
type EntityFeatures struct {
	Features map[string]*Feature `json:"features"`
}

// Feature represents a single feature value.
type Feature struct {
	Value     interface{} `json:"value"`
	Timestamp int64       `json:"timestamp"`
}

// GetAsOfRequest defines point-in-time feature retrieval.
type GetAsOfRequest struct {
	EntityKey string    `json:"entity_key"`
	Features  []string  `json:"features"`
	AsOf      time.Time `json:"as_of"`
}

// APIResponse is the standard response envelope for all API responses.
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Meta      *MetaInfo   `json:"meta,omitempty"`
}

// ErrorInfo contains structured error information.
type ErrorInfo struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// MetaInfo contains metadata about the response.
type MetaInfo struct {
	TotalCount int    `json:"total_count,omitempty"`
	PageSize   int    `json:"page_size,omitempty"`
	PageToken  string `json:"page_token,omitempty"`
	NextToken  string `json:"next_token,omitempty"`
}

// NewSuccessResponse creates a successful API response.
func NewSuccessResponse(data interface{}) *APIResponse {
	return &APIResponse{
		Success: true,
		Data:    data,
	}
}

// NewErrorResponse creates an error API response.
func NewErrorResponse(code, message string) *APIResponse {
	return &APIResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
}

// WithRequestID adds request ID to the response.
func (r *APIResponse) WithRequestID(requestID string) *APIResponse {
	r.RequestID = requestID
	return r
}

// WithMeta adds metadata to the response.
func (r *APIResponse) WithMeta(meta *MetaInfo) *APIResponse {
	r.Meta = meta
	return r
}

// WithErrorDetails adds details to an error response.
func (r *APIResponse) WithErrorDetails(details map[string]string) *APIResponse {
	if r.Error != nil {
		r.Error.Details = details
	}
	return r
}
