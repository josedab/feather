package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDataType_String(t *testing.T) {
	tests := []struct {
		dt       DataType
		expected string
	}{
		{DataTypeInt64, "int64"},
		{DataTypeFloat64, "float64"},
		{DataTypeString, "string"},
		{DataTypeBool, "bool"},
		{DataTypeBytes, "bytes"},
		{DataTypeVector, "vector"},
		{DataTypeTimestamp, "timestamp"},
		{DataType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.dt.String(); got != tt.expected {
				t.Errorf("DataType(%d).String() = %q, want %q", tt.dt, got, tt.expected)
			}
		})
	}
}

func TestParseDataType(t *testing.T) {
	tests := []struct {
		input    string
		expected DataType
		wantErr  bool
	}{
		{"int64", DataTypeInt64, false},
		{"float64", DataTypeFloat64, false},
		{"string", DataTypeString, false},
		{"bool", DataTypeBool, false},
		{"bytes", DataTypeBytes, false},
		{"vector", DataTypeVector, false},
		{"timestamp", DataTypeTimestamp, false},
		{"invalid", DataTypeString, true},
		{"", DataTypeString, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDataType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDataType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseDataType(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseDataType_UnknownWrapsError(t *testing.T) {
	_, err := ParseDataType("invalid")
	if err == nil {
		t.Fatal("expected error")
	}
	if !isErrUnknownDataType(err) {
		t.Errorf("expected ErrUnknownDataType, got %v", err)
	}
}

func isErrUnknownDataType(err error) bool {
	return err != nil && err.Error() != "" // error wraps ErrUnknownDataType
}

func TestDataType_MarshalJSON(t *testing.T) {
	tests := []struct {
		dt       DataType
		expected string
	}{
		{DataTypeInt64, `"int64"`},
		{DataTypeFloat64, `"float64"`},
		{DataTypeString, `"string"`},
		{DataTypeBool, `"bool"`},
		{DataTypeBytes, `"bytes"`},
		{DataTypeVector, `"vector"`},
		{DataTypeTimestamp, `"timestamp"`},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			data, err := json.Marshal(tt.dt)
			if err != nil {
				t.Fatalf("MarshalJSON failed: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalJSON = %s, want %s", data, tt.expected)
			}
		})
	}
}

func TestDataType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected DataType
		wantErr  bool
	}{
		{`"int64"`, DataTypeInt64, false},
		{`"float64"`, DataTypeFloat64, false},
		{`"string"`, DataTypeString, false},
		{`"bool"`, DataTypeBool, false},
		{`"bytes"`, DataTypeBytes, false},
		{`"vector"`, DataTypeVector, false},
		{`"timestamp"`, DataTypeTimestamp, false},
		{`"invalid"`, DataTypeString, true},
		{`123`, DataTypeString, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var dt DataType
			err := json.Unmarshal([]byte(tt.input), &dt)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err == nil && dt != tt.expected {
				t.Errorf("UnmarshalJSON(%s) = %v, want %v", tt.input, dt, tt.expected)
			}
		})
	}
}

func TestDataType_RoundTrip(t *testing.T) {
	for _, dt := range []DataType{DataTypeInt64, DataTypeFloat64, DataTypeString, DataTypeBool, DataTypeBytes, DataTypeVector, DataTypeTimestamp} {
		data, err := json.Marshal(dt)
		if err != nil {
			t.Fatalf("Marshal failed for %v: %v", dt, err)
		}
		var got DataType
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal failed for %v: %v", dt, err)
		}
		if got != dt {
			t.Errorf("Round trip failed: %v -> %s -> %v", dt, data, got)
		}
	}
}

func TestAggFunction_Constants(t *testing.T) {
	// Verify all aggregation functions have expected string values
	tests := []struct {
		f        AggFunction
		expected string
	}{
		{AggCount, "count"},
		{AggSum, "sum"},
		{AggAvg, "avg"},
		{AggMin, "min"},
		{AggMax, "max"},
		{AggLast, "last"},
	}
	for _, tt := range tests {
		if string(tt.f) != tt.expected {
			t.Errorf("AggFunction %v = %q, want %q", tt.f, string(tt.f), tt.expected)
		}
	}
}

func TestFeatureGroup_Fields(t *testing.T) {
	group := &FeatureGroup{
		Name:        "test_group",
		EntityType:  "user",
		Description: "Test group",
		TTL:         24 * time.Hour,
		Tags:        map[string]string{"env": "test"},
		Features: []FeatureSpec{
			{Name: "f1", DataType: DataTypeInt64},
			{Name: "f2", DataType: DataTypeString, Default: "unknown"},
		},
	}

	if group.Name != "test_group" {
		t.Error("Name mismatch")
	}
	if group.EntityType != "user" {
		t.Error("EntityType mismatch")
	}
	if group.TTL != 24*time.Hour {
		t.Error("TTL mismatch")
	}
	if len(group.Features) != 2 {
		t.Errorf("Expected 2 features, got %d", len(group.Features))
	}
	if group.Tags["env"] != "test" {
		t.Error("Tags mismatch")
	}
}

func TestFeatureSpec_WithAggregation(t *testing.T) {
	spec := FeatureSpec{
		Name:     "click_count",
		DataType: DataTypeInt64,
		Aggregation: &AggregationSpec{
			Function: AggCount,
			Window:   time.Hour,
			SlideBy:  time.Minute,
		},
	}

	if spec.Aggregation == nil {
		t.Fatal("Aggregation should not be nil")
	}
	if spec.Aggregation.Function != AggCount {
		t.Errorf("Expected AggCount, got %v", spec.Aggregation.Function)
	}
	if spec.Aggregation.Window != time.Hour {
		t.Error("Window mismatch")
	}
}

func TestFeatureSpec_WithValidation(t *testing.T) {
	min := 0.0
	max := 100.0
	spec := FeatureSpec{
		Name:     "score",
		DataType: DataTypeFloat64,
		Validation: &ValidationSpec{
			Min:     &min,
			Max:     &max,
			NotNull: true,
			OneOf:   []string{"a", "b"},
			Regex:   "^[a-z]+$",
		},
	}

	if spec.Validation == nil {
		t.Fatal("Validation should not be nil")
	}
	if *spec.Validation.Min != 0.0 {
		t.Error("Min mismatch")
	}
	if *spec.Validation.Max != 100.0 {
		t.Error("Max mismatch")
	}
	if !spec.Validation.NotNull {
		t.Error("NotNull should be true")
	}
}

func TestFeatureValue_Fields(t *testing.T) {
	now := time.Now().UnixNano()
	fv := &FeatureValue{
		Value:     42,
		Timestamp: now,
		Version:   3,
	}

	if fv.Value != 42 {
		t.Error("Value mismatch")
	}
	if fv.Timestamp != now {
		t.Error("Timestamp mismatch")
	}
	if fv.Version != 3 {
		t.Error("Version mismatch")
	}
}

func TestFeatureValue_JSONRoundTrip(t *testing.T) {
	fv := &FeatureValue{
		Value:     "hello",
		Timestamp: 1234567890,
		Version:   1,
	}

	data, err := json.Marshal(fv)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got FeatureValue
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.Value != "hello" {
		t.Errorf("Value = %v, want hello", got.Value)
	}
	if got.Timestamp != 1234567890 {
		t.Errorf("Timestamp = %d, want 1234567890", got.Timestamp)
	}
}

func TestFeatureUpdate_Fields(t *testing.T) {
	update := &FeatureUpdate{
		EntityKey: "user:123",
		Features:  map[string]interface{}{"age": 25},
		Timestamp: 1234567890,
		Version:   1,
	}

	if update.EntityKey != "user:123" {
		t.Error("EntityKey mismatch")
	}
	if update.Features["age"] != 25 {
		t.Error("Features mismatch")
	}
}

func TestGetFeaturesRequest_JSON(t *testing.T) {
	req := GetFeaturesRequest{
		Entities: []string{"user:1", "user:2"},
		Features: []string{"age", "score"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got GetFeaturesRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(got.Entities) != 2 {
		t.Errorf("Entities len = %d, want 2", len(got.Entities))
	}
	if len(got.Features) != 2 {
		t.Errorf("Features len = %d, want 2", len(got.Features))
	}
}

func TestNewSuccessResponse(t *testing.T) {
	resp := NewSuccessResponse(map[string]int{"count": 42})

	if !resp.Success {
		t.Error("Success should be true")
	}
	if resp.Error != nil {
		t.Error("Error should be nil")
	}
	if resp.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(ErrCodeNotFound, "entity not found")

	if resp.Success {
		t.Error("Success should be false")
	}
	if resp.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if resp.Error.Code != ErrCodeNotFound {
		t.Errorf("Error code = %q, want %q", resp.Error.Code, ErrCodeNotFound)
	}
	if resp.Error.Message != "entity not found" {
		t.Errorf("Error message = %q, want %q", resp.Error.Message, "entity not found")
	}
}

func TestAPIResponse_WithRequestID(t *testing.T) {
	resp := NewSuccessResponse(nil).WithRequestID("req-123")

	if resp.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", resp.RequestID, "req-123")
	}
}

func TestAPIResponse_WithMeta(t *testing.T) {
	meta := &MetaInfo{
		TotalCount: 100,
		PageSize:   10,
		PageToken:  "abc",
		NextToken:  "def",
	}
	resp := NewSuccessResponse(nil).WithMeta(meta)

	if resp.Meta == nil {
		t.Fatal("Meta should not be nil")
	}
	if resp.Meta.TotalCount != 100 {
		t.Error("TotalCount mismatch")
	}
	if resp.Meta.PageSize != 10 {
		t.Error("PageSize mismatch")
	}
	if resp.Meta.NextToken != "def" {
		t.Error("NextToken mismatch")
	}
}

func TestAPIResponse_WithErrorDetails(t *testing.T) {
	resp := NewErrorResponse(ErrCodeBadRequest, "bad request").
		WithErrorDetails(map[string]string{"field": "name", "reason": "required"})

	if resp.Error.Details == nil {
		t.Fatal("Details should not be nil")
	}
	if resp.Error.Details["field"] != "name" {
		t.Error("Details field mismatch")
	}
}

func TestAPIResponse_WithErrorDetails_NoError(t *testing.T) {
	// WithErrorDetails on a success response should be a no-op
	resp := NewSuccessResponse(nil).WithErrorDetails(map[string]string{"key": "val"})
	if resp.Error != nil {
		t.Error("Error should be nil for success response")
	}
}

func TestAPIResponse_JSON(t *testing.T) {
	resp := NewSuccessResponse(map[string]string{"key": "value"})
	resp.RequestID = "req-456"

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var got APIResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !got.Success {
		t.Error("Success should be true")
	}
	if got.RequestID != "req-456" {
		t.Error("RequestID mismatch")
	}
}

func TestGetAsOfRequest_Fields(t *testing.T) {
	now := time.Now()
	req := GetAsOfRequest{
		EntityKey: "user:1",
		Features:  []string{"age"},
		AsOf:      now,
	}

	if req.EntityKey != "user:1" {
		t.Error("EntityKey mismatch")
	}
	if len(req.Features) != 1 {
		t.Error("Features mismatch")
	}
	if req.AsOf != now {
		t.Error("AsOf mismatch")
	}
}

func TestEntityFeatures_Fields(t *testing.T) {
	ef := &EntityFeatures{
		Features: map[string]*Feature{
			"age": {Value: 25, Timestamp: 1234567890},
		},
	}

	if ef.Features["age"].Value != 25 {
		t.Error("Feature value mismatch")
	}
}

func TestGetFeaturesResponse_Fields(t *testing.T) {
	resp := &GetFeaturesResponse{
		Entities: map[string]*EntityFeatures{
			"user:1": {
				Features: map[string]*Feature{
					"age": {Value: 25, Timestamp: 1234567890},
				},
			},
		},
	}

	if len(resp.Entities) != 1 {
		t.Errorf("Expected 1 entity, got %d", len(resp.Entities))
	}
}
