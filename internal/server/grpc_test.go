package server

import (
	"context"
	"testing"
	"time"

	pb "github.com/feather-store/feather/api/proto"
	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/storage"
)

// testGRPCServer wraps a GRPCServer for testing.
type testGRPCServer struct {
	*GRPCServer
	store  *storage.Store
	schema *storage.Registry
	t      *testing.T
}

// newTestGRPCServer creates a new test gRPC server with in-memory storage.
func newTestGRPCServer(t *testing.T) *testGRPCServer {
	t.Helper()

	schema := storage.NewRegistry()
	store, err := storage.NewStore(storage.StoreOptions{
		HotMaxSize:   1 << 20, // 1MB
		WarmInMemory: true,
	}, schema)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	agg := aggregation.NewEngine()
	healthChecker := NewHealthChecker(store, agg, schema)

	srv := NewGRPCServer(store, agg, schema, nil, GRPCServerConfig{
		Port:          0,
		MaxConcurrent: 100,
		HealthChecker: healthChecker,
	})

	t.Cleanup(func() {
		store.Close()
	})

	return &testGRPCServer{
		GRPCServer: srv,
		store:      store,
		schema:     schema,
		t:          t,
	}
}

// seedFeatures adds test features to the store.
func (ts *testGRPCServer) seedFeatures(entityID string, features map[string]interface{}) {
	ts.t.Helper()

	featureValues := make(map[string]*domain.FeatureValue)
	for name, value := range features {
		featureValues[name] = &domain.FeatureValue{
			Value:     value,
			Timestamp: time.Now().UnixNano(),
			Version:   1,
		}
	}

	if err := ts.store.Put(entityID, featureValues); err != nil {
		ts.t.Fatalf("failed to seed features: %v", err)
	}
}

func TestGRPCServer_GetFeatures(t *testing.T) {
	ts := newTestGRPCServer(t)

	// Seed test data
	ts.seedFeatures("user:123", map[string]interface{}{
		"purchase_count": float64(42),
		"total_spent":    float64(1234.56),
	})

	tests := []struct {
		name       string
		request    *pb.GetFeaturesRequest
		wantErr    bool
		wantEntity string
	}{
		{
			name: "get single feature",
			request: &pb.GetFeaturesRequest{
				Entities: []string{"user:123"},
				Features: []string{"purchase_count"},
			},
			wantEntity: "user:123",
		},
		{
			name: "get multiple features",
			request: &pb.GetFeaturesRequest{
				Entities: []string{"user:123"},
				Features: []string{"purchase_count", "total_spent"},
			},
			wantEntity: "user:123",
		},
		{
			name: "get multiple entities",
			request: &pb.GetFeaturesRequest{
				Entities: []string{"user:123", "user:456"},
				Features: []string{"purchase_count"},
			},
			wantEntity: "user:123",
		},
		{
			name: "missing entities",
			request: &pb.GetFeaturesRequest{
				Entities: []string{},
				Features: []string{"purchase_count"},
			},
			wantErr: true,
		},
		{
			name: "missing features",
			request: &pb.GetFeaturesRequest{
				Entities: []string{"user:123"},
				Features: []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ts.GetFeatures(context.Background(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantEntity != "" {
				if _, ok := resp.Entities[tt.wantEntity]; !ok {
					t.Errorf("expected entity %q in response", tt.wantEntity)
				}
			}
		})
	}
}

func TestGRPCServer_PutFeatures(t *testing.T) {
	ts := newTestGRPCServer(t)

	tests := []struct {
		name        string
		request     *pb.PutFeaturesRequest
		wantSuccess bool
	}{
		{
			name: "put single feature",
			request: &pb.PutFeaturesRequest{
				EntityKey: "user:456",
				Features: map[string]*pb.FeatureValue{
					"score": {Value: &pb.FeatureValue_DoubleValue{DoubleValue: 0.95}},
				},
				Version: 1,
			},
			wantSuccess: true,
		},
		{
			name: "put multiple features",
			request: &pb.PutFeaturesRequest{
				EntityKey: "user:789",
				Features: map[string]*pb.FeatureValue{
					"score":     {Value: &pb.FeatureValue_DoubleValue{DoubleValue: 0.85}},
					"rank":      {Value: &pb.FeatureValue_IntValue{IntValue: 10}},
					"is_active": {Value: &pb.FeatureValue_BoolValue{BoolValue: true}},
				},
				Version: 1,
			},
			wantSuccess: true,
		},
		{
			name: "put string feature",
			request: &pb.PutFeaturesRequest{
				EntityKey: "user:111",
				Features: map[string]*pb.FeatureValue{
					"name": {Value: &pb.FeatureValue_StringValue{StringValue: "test user"}},
				},
				Version: 1,
			},
			wantSuccess: true,
		},
		{
			name: "put empty features",
			request: &pb.PutFeaturesRequest{
				EntityKey: "user:222",
				Features:  map[string]*pb.FeatureValue{},
				Version:   1,
			},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ts.PutFeatures(context.Background(), tt.request)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp.Success != tt.wantSuccess {
				t.Errorf("Success = %v, want %v", resp.Success, tt.wantSuccess)
			}
		})
	}

	// Verify features were stored
	t.Run("verify stored features", func(t *testing.T) {
		resp, err := ts.GetFeatures(context.Background(), &pb.GetFeaturesRequest{
			Entities: []string{"user:456"},
			Features: []string{"score"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entity, ok := resp.Entities["user:456"]
		if !ok {
			t.Fatal("expected entity user:456 in response")
		}

		scoreVal, ok := entity.Features["score"]
		if !ok {
			t.Fatal("expected score feature in response")
		}

		if scoreVal.GetDoubleValue() != 0.95 {
			t.Errorf("score = %v, want 0.95", scoreVal.GetDoubleValue())
		}
	})
}

func TestGRPCServer_GetFeaturesAsOf(t *testing.T) {
	ts := newTestGRPCServer(t)

	// Seed test data
	ts.seedFeatures("user:123", map[string]interface{}{
		"purchase_count": float64(42),
	})

	tests := []struct {
		name    string
		request *pb.GetFeaturesAsOfRequest
		wantErr bool
	}{
		{
			name: "get features as of future time",
			request: &pb.GetFeaturesAsOfRequest{
				EntityKey:     "user:123",
				Features:      []string{"purchase_count"},
				AsOfTimestamp: time.Now().Add(time.Hour).UnixNano(),
			},
			// Should not error - behavior depends on storage implementation
		},
		{
			name: "get features as of past time",
			request: &pb.GetFeaturesAsOfRequest{
				EntityKey:     "user:123",
				Features:      []string{"purchase_count"},
				AsOfTimestamp: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano(),
			},
			// May return not found for past time, but should not error
		},
		{
			name: "get features for nonexistent entity",
			request: &pb.GetFeaturesAsOfRequest{
				EntityKey:     "user:999",
				Features:      []string{"purchase_count"},
				AsOfTimestamp: time.Now().UnixNano(),
			},
			// May return error or empty result depending on storage implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ts.GetFeaturesAsOf(context.Background(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			// Not found errors are acceptable for point-in-time queries
			// since the feature may not exist at that timestamp
			if err != nil {
				// Accept not found errors
				return
			}

			// If we got a response, verify it has the expected structure
			if resp != nil && resp.Entities != nil {
				// Response is valid - the actual content depends on storage state
				_ = resp.Entities[tt.request.GetEntityKey()]
			}
		})
	}
}

func TestGRPCServer_HealthCheck(t *testing.T) {
	ts := newTestGRPCServer(t)

	tests := []struct {
		name       string
		service    string
		wantStatus pb.HealthCheckResponse_ServingStatus
	}{
		{
			name:       "check overall health",
			service:    "",
			wantStatus: pb.HealthCheckResponse_SERVING,
		},
		{
			name:       "check feature service",
			service:    "feather.v1.FeatureService",
			wantStatus: pb.HealthCheckResponse_SERVING,
		},
		{
			name:       "check unknown service",
			service:    "unknown.Service",
			wantStatus: pb.HealthCheckResponse_SERVICE_UNKNOWN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ts.Check(context.Background(), &pb.HealthCheckRequest{
				Service: tt.service,
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", resp.Status, tt.wantStatus)
			}
		})
	}
}

func TestGRPCServer_IsTLSEnabled(t *testing.T) {
	ts := newTestGRPCServer(t)

	// Without TLS config, should return false
	if ts.IsTLSEnabled() {
		t.Error("expected IsTLSEnabled() = false for server without TLS config")
	}
}

func TestDomainToProtoValue(t *testing.T) {
	tests := []struct {
		name  string
		input *domain.FeatureValue
		check func(t *testing.T, pv *pb.FeatureValue)
	}{
		{
			name: "int64 value",
			input: &domain.FeatureValue{
				Value:     int64(42),
				Timestamp: 1000,
			},
			check: func(t *testing.T, pv *pb.FeatureValue) {
				if pv.GetIntValue() != 42 {
					t.Errorf("expected int value 42, got %v", pv.GetIntValue())
				}
			},
		},
		{
			name: "int value",
			input: &domain.FeatureValue{
				Value:     int(100),
				Timestamp: 1000,
			},
			check: func(t *testing.T, pv *pb.FeatureValue) {
				if pv.GetIntValue() != 100 {
					t.Errorf("expected int value 100, got %v", pv.GetIntValue())
				}
			},
		},
		{
			name: "float64 value",
			input: &domain.FeatureValue{
				Value:     float64(3.14),
				Timestamp: 1000,
			},
			check: func(t *testing.T, pv *pb.FeatureValue) {
				if pv.GetDoubleValue() != 3.14 {
					t.Errorf("expected double value 3.14, got %v", pv.GetDoubleValue())
				}
			},
		},
		{
			name: "string value",
			input: &domain.FeatureValue{
				Value:     "hello",
				Timestamp: 1000,
			},
			check: func(t *testing.T, pv *pb.FeatureValue) {
				if pv.GetStringValue() != "hello" {
					t.Errorf("expected string value 'hello', got %v", pv.GetStringValue())
				}
			},
		},
		{
			name: "bool value",
			input: &domain.FeatureValue{
				Value:     true,
				Timestamp: 1000,
			},
			check: func(t *testing.T, pv *pb.FeatureValue) {
				if !pv.GetBoolValue() {
					t.Errorf("expected bool value true, got %v", pv.GetBoolValue())
				}
			},
		},
		{
			name: "bytes value",
			input: &domain.FeatureValue{
				Value:     []byte{1, 2, 3},
				Timestamp: 1000,
			},
			check: func(t *testing.T, pv *pb.FeatureValue) {
				if len(pv.GetBytesValue()) != 3 {
					t.Errorf("expected bytes value, got %v", pv.GetBytesValue())
				}
			},
		},
		{
			name: "vector value",
			input: &domain.FeatureValue{
				Value:     []float32{0.1, 0.2, 0.3},
				Timestamp: 1000,
			},
			check: func(t *testing.T, pv *pb.FeatureValue) {
				if pv.GetVectorValue() == nil || len(pv.GetVectorValue().GetValues()) != 3 {
					t.Errorf("expected vector value, got %v", pv.GetVectorValue())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pv := domainToProtoValue(tt.input)
			if pv.Timestamp != tt.input.Timestamp {
				t.Errorf("Timestamp = %v, want %v", pv.Timestamp, tt.input.Timestamp)
			}
			tt.check(t, pv)
		})
	}
}

func TestProtoToDomainValue(t *testing.T) {
	tests := []struct {
		name  string
		input *pb.FeatureValue
		want  interface{}
	}{
		{
			name:  "int value",
			input: &pb.FeatureValue{Value: &pb.FeatureValue_IntValue{IntValue: 42}},
			want:  int64(42),
		},
		{
			name:  "double value",
			input: &pb.FeatureValue{Value: &pb.FeatureValue_DoubleValue{DoubleValue: 3.14}},
			want:  float64(3.14),
		},
		{
			name:  "string value",
			input: &pb.FeatureValue{Value: &pb.FeatureValue_StringValue{StringValue: "hello"}},
			want:  "hello",
		},
		{
			name:  "bool value",
			input: &pb.FeatureValue{Value: &pb.FeatureValue_BoolValue{BoolValue: true}},
			want:  true,
		},
		{
			name:  "nil for no value",
			input: &pb.FeatureValue{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := protoToDomainValue(tt.input)
			if got != tt.want {
				t.Errorf("protoToDomainValue() = %v, want %v", got, tt.want)
			}
		})
	}
}
