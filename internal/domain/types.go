// Package domain defines the core types for the Feather feature store.
package domain

import (
	"encoding/json"
	"time"
)

// DataType enumerates supported feature data types.
type DataType int

const (
	DataTypeInt64 DataType = iota
	DataTypeFloat64
	DataTypeString
	DataTypeBool
	DataTypeBytes
	DataTypeVector // Float32 array
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

func ParseDataType(s string) DataType {
	switch s {
	case "int64":
		return DataTypeInt64
	case "float64":
		return DataTypeFloat64
	case "string":
		return DataTypeString
	case "bool":
		return DataTypeBool
	case "bytes":
		return DataTypeBytes
	case "vector":
		return DataTypeVector
	case "timestamp":
		return DataTypeTimestamp
	default:
		return DataTypeString
	}
}

func (d DataType) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *DataType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*d = ParseDataType(s)
	return nil
}

// AggFunction defines aggregation function types.
type AggFunction string

const (
	AggCount AggFunction = "count"
	AggSum   AggFunction = "sum"
	AggAvg   AggFunction = "avg"
	AggMin   AggFunction = "min"
	AggMax   AggFunction = "max"
	AggLast  AggFunction = "last"
)

// AggregationSpec defines time-window aggregations.
type AggregationSpec struct {
	Function AggFunction   `json:"function" yaml:"function"`
	Window   time.Duration `json:"window" yaml:"window"`
	SlideBy  time.Duration `json:"slide_by,omitempty" yaml:"slide_by,omitempty"`
}

// ValidationSpec defines feature validation rules.
type ValidationSpec struct {
	Min     *float64 `json:"min,omitempty" yaml:"min,omitempty"`
	Max     *float64 `json:"max,omitempty" yaml:"max,omitempty"`
	NotNull bool     `json:"not_null" yaml:"not_null"`
	OneOf   []string `json:"one_of,omitempty" yaml:"one_of,omitempty"`
	Regex   string   `json:"regex,omitempty" yaml:"regex,omitempty"`
}

// FeatureSpec defines a single feature.
type FeatureSpec struct {
	Name        string           `json:"name" yaml:"name"`
	DataType    DataType         `json:"data_type" yaml:"data_type"`
	Dimensions  []int            `json:"dimensions,omitempty" yaml:"dimensions,omitempty"`
	Default     interface{}      `json:"default,omitempty" yaml:"default,omitempty"`
	Aggregation *AggregationSpec `json:"aggregation,omitempty" yaml:"aggregation,omitempty"`
	Validation  *ValidationSpec  `json:"validation,omitempty" yaml:"validation,omitempty"`
}

// FeatureGroup defines a collection of related features.
type FeatureGroup struct {
	Name        string            `json:"name" yaml:"name"`
	EntityType  string            `json:"entity_type" yaml:"entity_type"`
	Description string            `json:"description" yaml:"description"`
	Features    []FeatureSpec     `json:"features" yaml:"features"`
	TTL         time.Duration     `json:"ttl" yaml:"ttl"`
	Tags        map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// FeatureValue stores a feature with metadata.
type FeatureValue struct {
	Value     interface{} `json:"value"`
	Timestamp int64       `json:"timestamp"` // Unix nanos
	Version   int64       `json:"version"`
}

// FeatureUpdate represents an incoming feature update.
type FeatureUpdate struct {
	EntityKey string                 `json:"entity_key"`
	Features  map[string]interface{} `json:"features"`
	Timestamp int64                  `json:"timestamp"`
	Version   int64                  `json:"version"`
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

// PaginatedRequest contains pagination parameters.
type PaginatedRequest struct {
	PageSize  int    `json:"page_size"`
	PageToken string `json:"page_token"`
}
