package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/config"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/metrics"
	"github.com/feather-store/feather/internal/storage"
)

// GRPCServer provides gRPC feature serving.
type GRPCServer struct {
	store         *storage.Store
	aggregation   *aggregation.Engine
	schema        *storage.Registry
	metrics       *metrics.Metrics
	healthChecker *HealthChecker
	server        *grpc.Server
	port          int
	tlsConfig     *config.TLSConfig
}

// GRPCServerConfig configures the gRPC server.
type GRPCServerConfig struct {
	Port          int
	MaxConcurrent int
	HealthChecker *HealthChecker
	TLS           *config.TLSConfig
}

// NewGRPCServer creates a new gRPC server.
func NewGRPCServer(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
	m *metrics.Metrics,
	cfg GRPCServerConfig,
) *GRPCServer {
	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(cfg.MaxConcurrent)),
	}

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		cert, err := cfg.TLS.LoadCertificate()
		if err == nil {
			tlsConfig, _ := cfg.TLS.BuildTLSConfig()
			if tlsConfig != nil {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
			creds := credentials.NewTLS(tlsConfig)
			opts = append(opts, grpc.Creds(creds))
		}
	}

	if m != nil {
		opts = append(opts,
			grpc.UnaryInterceptor(m.UnaryServerInterceptor()),
			grpc.StreamInterceptor(m.StreamServerInterceptor()),
		)
	}

	return &GRPCServer{
		store:         store,
		aggregation:   agg,
		schema:        schema,
		metrics:       m,
		healthChecker: cfg.HealthChecker,
		server:        grpc.NewServer(opts...),
		port:          cfg.Port,
		tlsConfig:     cfg.TLS,
	}
}

// Start starts the gRPC server.
func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Register service
	RegisterFeatureServiceServer(s.server, s)

	return s.server.Serve(lis)
}

// Stop gracefully stops the server.
func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
}

// IsTLSEnabled returns true if the server is configured to use TLS.
func (s *GRPCServer) IsTLSEnabled() bool {
	return s.tlsConfig != nil && s.tlsConfig.Enabled
}

// GetFeatures retrieves features for one or more entities.
func (s *GRPCServer) GetFeatures(ctx context.Context, req *GetFeaturesRequest) (*GetFeaturesResponse, error) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("GetFeatures", time.Since(start))
		}
	}()

	if len(req.Entities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "entities required")
	}
	if len(req.Features) == 0 {
		return nil, status.Error(codes.InvalidArgument, "features required")
	}

	result := &GetFeaturesResponse{
		Entities: make(map[string]*EntityFeatures),
	}

	for _, entityKey := range req.Entities {
		features, err := s.getFeaturesForEntity(ctx, entityKey, req.Features)
		if err != nil {
			if domain.IsNotFound(err) {
				continue
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		result.Entities[entityKey] = features
	}

	return result, nil
}

// GetFeaturesAsOf retrieves features as of a specific timestamp.
func (s *GRPCServer) GetFeaturesAsOf(ctx context.Context, req *GetFeaturesAsOfRequest) (*GetFeaturesResponse, error) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("GetFeaturesAsOf", time.Since(start))
		}
	}()

	asOf := time.Unix(0, req.AsOfTimestamp)
	features, err := s.store.GetAsOf(req.EntityKey, req.Features, asOf)
	if err != nil {
		if domain.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	entityFeatures := &EntityFeatures{
		Features: make(map[string]*FeatureValue),
	}
	for name, val := range features {
		entityFeatures.Features[name] = domainToProtoValue(val)
	}

	return &GetFeaturesResponse{
		Entities: map[string]*EntityFeatures{
			req.EntityKey: entityFeatures,
		},
	}, nil
}

// PutFeatures stores features for an entity.
func (s *GRPCServer) PutFeatures(ctx context.Context, req *PutFeaturesRequest) (*PutFeaturesResponse, error) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("PutFeatures", time.Since(start))
		}
	}()

	features := make(map[string]*domain.FeatureValue)
	timestamp := time.Now().UnixNano()

	for name, val := range req.Features {
		features[name] = &domain.FeatureValue{
			Value:     protoToDomainValue(val),
			Timestamp: timestamp,
			Version:   req.Version,
		}

		// Update aggregations if applicable
		if s.aggregation.GetSpec(name) != nil {
			if floatVal, ok := features[name].Value.(float64); ok {
				s.aggregation.Update(req.EntityKey, name, floatVal, time.Now())
			}
		}
	}

	if err := s.store.Put(req.EntityKey, features); err != nil {
		return &PutFeaturesResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &PutFeaturesResponse{
		Success: true,
	}, nil
}

// getFeaturesForEntity retrieves features for a single entity.
func (s *GRPCServer) getFeaturesForEntity(ctx context.Context, entityKey string, featureNames []string) (*EntityFeatures, error) {
	// Get from storage
	features, err := s.store.Get(entityKey, featureNames)
	if err != nil && !domain.IsNotFound(err) {
		return nil, err
	}

	result := &EntityFeatures{
		Features: make(map[string]*FeatureValue),
	}

	// Add found features
	for name, val := range features {
		result.Features[name] = domainToProtoValue(val)
	}

	// Compute aggregations for missing features
	for _, name := range featureNames {
		if _, ok := result.Features[name]; ok {
			continue
		}

		// Check if this is an aggregation feature
		if spec := s.aggregation.GetSpec(name); spec != nil {
			val, err := s.aggregation.ComputeWithSpec(entityKey, name)
			if err == nil {
				result.Features[name] = &FeatureValue{
					DoubleValue:    val,
					HasDoubleValue: true,
					Timestamp:      time.Now().UnixNano(),
				}
			}
		}
	}

	return result, nil
}

// Helper types for gRPC (simplified - in production would use generated protobuf)

// GetFeaturesRequest is the gRPC request type.
type GetFeaturesRequest struct {
	Entities []string
	Features []string
}

// GetFeaturesAsOfRequest is the gRPC request for point-in-time retrieval.
type GetFeaturesAsOfRequest struct {
	EntityKey     string
	Features      []string
	AsOfTimestamp int64
}

// GetFeaturesResponse is the gRPC response type.
type GetFeaturesResponse struct {
	Entities map[string]*EntityFeatures
}

// EntityFeatures contains features for an entity.
type EntityFeatures struct {
	Features map[string]*FeatureValue
}

// FeatureValue represents a gRPC feature value.
type FeatureValue struct {
	IntValue       int64
	DoubleValue    float64
	StringValue    string
	BoolValue      bool
	BytesValue     []byte
	VectorValue    []float32
	Timestamp      int64
	HasIntValue    bool
	HasDoubleValue bool
	HasStringValue bool
	HasBoolValue   bool
	HasBytesValue  bool
	HasVectorValue bool
}

// PutFeaturesRequest is the gRPC request to store features.
type PutFeaturesRequest struct {
	EntityKey string
	Features  map[string]*FeatureValue
	Version   int64
}

// PutFeaturesResponse is the gRPC response after storing.
type PutFeaturesResponse struct {
	Success bool
	Error   string
}

func domainToProtoValue(val *domain.FeatureValue) *FeatureValue {
	pv := &FeatureValue{
		Timestamp: val.Timestamp,
	}

	switch v := val.Value.(type) {
	case int64:
		pv.IntValue = v
		pv.HasIntValue = true
	case int:
		pv.IntValue = int64(v)
		pv.HasIntValue = true
	case float64:
		pv.DoubleValue = v
		pv.HasDoubleValue = true
	case string:
		pv.StringValue = v
		pv.HasStringValue = true
	case bool:
		pv.BoolValue = v
		pv.HasBoolValue = true
	case []byte:
		pv.BytesValue = v
		pv.HasBytesValue = true
	case []float32:
		pv.VectorValue = v
		pv.HasVectorValue = true
	}

	return pv
}

func protoToDomainValue(val *FeatureValue) interface{} {
	switch {
	case val.HasIntValue:
		return val.IntValue
	case val.HasDoubleValue:
		return val.DoubleValue
	case val.HasStringValue:
		return val.StringValue
	case val.HasBoolValue:
		return val.BoolValue
	case val.HasBytesValue:
		return val.BytesValue
	case val.HasVectorValue:
		return val.VectorValue
	default:
		return nil
	}
}

// RegisterFeatureServiceServer registers the service (placeholder for generated code).
func RegisterFeatureServiceServer(s *grpc.Server, srv *GRPCServer) {
	// In production, this would be generated by protoc
	// For now, we use a custom registration
}

// gRPC Health Check Service Implementation
// Follows the grpc.health.v1.Health protocol for Kubernetes compatibility

// HealthCheckRequest is the request for health checking.
type HealthCheckRequest struct {
	Service string
}

// HealthCheckResponse is the response for health checking.
type HealthCheckResponse struct {
	Status ServingStatus
}

// ServingStatus represents the health status of a service.
type ServingStatus int32

const (
	// ServingStatusUnknown indicates an unknown status.
	ServingStatusUnknown ServingStatus = 0
	// ServingStatusServing indicates the service is serving.
	ServingStatusServing ServingStatus = 1
	// ServingStatusNotServing indicates the service is not serving.
	ServingStatusNotServing ServingStatus = 2
	// ServingStatusServiceUnknown indicates the service is unknown.
	ServingStatusServiceUnknown ServingStatus = 3
)

// Check implements the gRPC health check service.
func (s *GRPCServer) Check(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	// Check specific service or overall health
	serviceName := req.Service
	if serviceName == "" || serviceName == "feather.FeatureService" {
		// Check overall health
		if s.healthChecker != nil {
			if !s.healthChecker.IsHealthy() {
				return &HealthCheckResponse{Status: ServingStatusNotServing}, nil
			}
			if !s.healthChecker.IsReady() {
				return &HealthCheckResponse{Status: ServingStatusNotServing}, nil
			}
		}
		return &HealthCheckResponse{Status: ServingStatusServing}, nil
	}

	// Unknown service
	return &HealthCheckResponse{Status: ServingStatusServiceUnknown}, nil
}

// Watch implements the streaming health check service.
func (s *GRPCServer) Watch(req *HealthCheckRequest, stream grpc.ServerStream) error {
	// Send initial status
	resp, err := s.Check(stream.Context(), req)
	if err != nil {
		return err
	}

	if err := stream.SendMsg(resp); err != nil {
		return err
	}

	// For now, we just send one response.
	// In a production implementation, this would send updates when health changes.
	<-stream.Context().Done()
	return stream.Context().Err()
}

// RegisterHealthServer registers the health service on the gRPC server.
func RegisterHealthServer(s *grpc.Server, srv *GRPCServer) {
	// In production, this would use the generated health service registration.
	// For now, we document that the health methods are available on the server.
}

// HealthService returns information about the health service for external registration.
func (s *GRPCServer) HealthService() *GRPCHealthService {
	return &GRPCHealthService{server: s}
}

// GRPCHealthService wraps the health check functionality.
type GRPCHealthService struct {
	server *GRPCServer
}

// Check performs a health check (implements grpc_health_v1.HealthServer interface pattern).
func (h *GRPCHealthService) Check(ctx context.Context, req *HealthCheckRequest) (*HealthCheckResponse, error) {
	return h.server.Check(ctx, req)
}
