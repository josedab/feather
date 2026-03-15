package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"math"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "github.com/feather-store/feather/api/proto"
	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/config"
	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/metrics"
	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/core/tracing"
)

// GRPCServer provides gRPC feature serving.
type GRPCServer struct {
	pb.UnimplementedFeatureServiceServer
	pb.UnimplementedHealthServer

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
	Tracer        *tracing.Tracer
}

// NewGRPCServer creates a new gRPC server.
func NewGRPCServer(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
	m *metrics.Metrics,
	cfg GRPCServerConfig,
) *GRPCServer {
	opts := []grpc.ServerOption{}
	if cfg.MaxConcurrent > 0 {
		maxConcurrent := cfg.MaxConcurrent
		if maxConcurrent > math.MaxUint32 {
			maxConcurrent = math.MaxUint32
		}
		//nolint:gosec // MaxConcurrent is clamped to uint32 range.
		opts = append(opts, grpc.MaxConcurrentStreams(uint32(maxConcurrent)))
	}

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.Enabled {
		cert, err := cfg.TLS.LoadCertificate()
		if err != nil {
			slog.Error("gRPC TLS: failed to load certificate, server will NOT use TLS", "error", err)
		} else {
			tlsConfig, err := cfg.TLS.BuildTLSConfig()
			if err != nil {
				slog.Error("gRPC TLS: failed to build TLS config, server will NOT use TLS", "error", err)
			} else if tlsConfig != nil {
				tlsConfig.Certificates = []tls.Certificate{cert}
				creds := credentials.NewTLS(tlsConfig)
				opts = append(opts, grpc.Creds(creds))
			}
		}
	}

	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	if m != nil {
		unaryInterceptors = append(unaryInterceptors, m.UnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, m.StreamServerInterceptor())
	}
	if cfg.Tracer != nil {
		unaryInterceptors = append(unaryInterceptors, cfg.Tracer.GRPCUnaryInterceptor())
		streamInterceptors = append(streamInterceptors, cfg.Tracer.GRPCStreamInterceptor())
	}
	if len(unaryInterceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
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

	// Register services using generated registration functions
	pb.RegisterFeatureServiceServer(s.server, s)
	pb.RegisterHealthServer(s.server, s)

	return s.server.Serve(lis)
}

// Stop gracefully stops the server with a timeout.
// If the context expires, it forces an immediate stop.
func (s *GRPCServer) Stop(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
	case <-ctx.Done():
		// Timeout reached, force stop
		s.server.Stop()
	}
}

// IsTLSEnabled returns true if the server is configured to use TLS.
func (s *GRPCServer) IsTLSEnabled() bool {
	return s.tlsConfig != nil && s.tlsConfig.Enabled
}

// maxGRPCEntities is the maximum number of entities allowed per gRPC request.
const maxGRPCEntities = 10000

// GetFeatures retrieves features for one or more entities.
func (s *GRPCServer) GetFeatures(ctx context.Context, req *pb.GetFeaturesRequest) (*pb.GetFeaturesResponse, error) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("GetFeatures", time.Since(start))
		}
	}()

	if len(req.GetEntities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "entities required")
	}
	if len(req.GetFeatures()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "features required")
	}
	if len(req.GetEntities()) > maxGRPCEntities {
		return nil, status.Errorf(codes.InvalidArgument, "too many entities: %d exceeds maximum %d", len(req.GetEntities()), maxGRPCEntities)
	}
	for _, e := range req.GetEntities() {
		if err := domain.ValidateEntityKey(e); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid entity key: %s", err.Error())
		}
	}
	if err := domain.ValidateFeatureNames(req.GetFeatures()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid feature name: %s", err.Error())
	}

	result := &pb.GetFeaturesResponse{
		Entities: make(map[string]*pb.EntityFeatures),
	}

	for _, entityKey := range req.GetEntities() {
		features, err := s.getFeaturesForEntity(ctx, entityKey, req.GetFeatures())
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

// GetFeaturesStream retrieves features with server-side streaming.
func (s *GRPCServer) GetFeaturesStream(req *pb.GetFeaturesRequest, stream grpc.ServerStreamingServer[pb.EntityFeaturesResponse]) error {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("GetFeaturesStream", time.Since(start))
		}
	}()

	if len(req.GetEntities()) == 0 {
		return status.Error(codes.InvalidArgument, "entities required")
	}
	if len(req.GetFeatures()) == 0 {
		return status.Error(codes.InvalidArgument, "features required")
	}
	if len(req.GetEntities()) > maxGRPCEntities {
		return status.Errorf(codes.InvalidArgument, "too many entities: %d exceeds maximum %d", len(req.GetEntities()), maxGRPCEntities)
	}
	if err := domain.ValidateFeatureNames(req.GetFeatures()); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid feature name: %s", err.Error())
	}

	for _, entityKey := range req.GetEntities() {
		if err := domain.ValidateEntityKey(entityKey); err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid entity key: %s", err.Error())
		}
		features, err := s.getFeaturesForEntity(stream.Context(), entityKey, req.GetFeatures())
		if err != nil {
			if domain.IsNotFound(err) {
				continue
			}
			return status.Error(codes.Internal, err.Error())
		}

		if err := stream.Send(&pb.EntityFeaturesResponse{
			EntityKey: entityKey,
			Features:  features,
		}); err != nil {
			return fmt.Errorf("sending features for entity %s: %w", entityKey, err)
		}
	}

	return nil
}

// GetFeaturesAsOf retrieves features as of a specific timestamp.
func (s *GRPCServer) GetFeaturesAsOf(ctx context.Context, req *pb.GetFeaturesAsOfRequest) (*pb.GetFeaturesResponse, error) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("GetFeaturesAsOf", time.Since(start))
		}
	}()

	if req.GetEntityKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "entity_key required")
	}
	if err := domain.ValidateEntityKey(req.GetEntityKey()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid entity key: %s", err.Error())
	}
	if len(req.GetFeatures()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "features required")
	}
	if err := domain.ValidateFeatureNames(req.GetFeatures()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid feature name: %s", err.Error())
	}
	if req.GetAsOfTimestamp() == 0 {
		return nil, status.Error(codes.InvalidArgument, "as_of_timestamp required")
	}

	asOf := time.Unix(0, req.GetAsOfTimestamp())
	features, err := s.store.GetAsOf(ctx, req.GetEntityKey(), req.GetFeatures(), asOf)
	if err != nil {
		if domain.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	entityFeatures := &pb.EntityFeatures{
		Features: make(map[string]*pb.FeatureValue),
	}
	for name, val := range features {
		entityFeatures.Features[name] = domainToProtoValue(val)
	}

	return &pb.GetFeaturesResponse{
		Entities: map[string]*pb.EntityFeatures{
			req.GetEntityKey(): entityFeatures,
		},
	}, nil
}

// PutFeatures stores features for an entity.
func (s *GRPCServer) PutFeatures(ctx context.Context, req *pb.PutFeaturesRequest) (*pb.PutFeaturesResponse, error) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("PutFeatures", time.Since(start))
		}
	}()

	if req.GetEntityKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "entity_key required")
	}
	if err := domain.ValidateEntityKey(req.GetEntityKey()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid entity key: %s", err.Error())
	}
	if len(req.GetFeatures()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "features required")
	}

	features := make(map[string]*domain.FeatureValue)
	timestamp := time.Now().UnixNano()

	for name, val := range req.GetFeatures() {
		features[name] = &domain.FeatureValue{
			Value:     protoToDomainValue(val),
			Timestamp: timestamp,
			Version:   req.GetVersion(),
		}

		// Update aggregations if applicable
		if s.aggregation.GetSpec(name) != nil {
			if floatVal, ok := domain.ToFloat64(features[name].Value); ok {
				if err := s.aggregation.Update(req.GetEntityKey(), name, floatVal, time.Unix(0, timestamp)); err != nil {
					return &pb.PutFeaturesResponse{
						Success: false,
						Error:   err.Error(),
					}, nil
				}
			}
		}
	}

	if err := s.store.Put(ctx, req.GetEntityKey(), features); err != nil {
		return &pb.PutFeaturesResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.PutFeaturesResponse{
		Success: true,
	}, nil
}

// getFeaturesForEntity retrieves features for a single entity.
func (s *GRPCServer) getFeaturesForEntity(ctx context.Context, entityKey string, featureNames []string) (*pb.EntityFeatures, error) {
	// Get from storage
	features, err := s.store.Get(ctx, entityKey, featureNames)
	if err != nil && !domain.IsNotFound(err) {
		return nil, fmt.Errorf("getting features for entity %s: %w", entityKey, err)
	}

	result := &pb.EntityFeatures{
		Features: make(map[string]*pb.FeatureValue),
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
				result.Features[name] = &pb.FeatureValue{
					Value:     &pb.FeatureValue_DoubleValue{DoubleValue: val},
					Timestamp: time.Now().UnixNano(),
				}
			}
		}
	}

	return result, nil
}

// domainToProtoValue converts a domain FeatureValue to a protobuf FeatureValue.
func domainToProtoValue(val *domain.FeatureValue) *pb.FeatureValue {
	pv := &pb.FeatureValue{
		Timestamp: val.Timestamp,
	}

	switch v := val.Value.(type) {
	case int64:
		pv.Value = &pb.FeatureValue_IntValue{IntValue: v}
	case int:
		pv.Value = &pb.FeatureValue_IntValue{IntValue: int64(v)}
	case float64:
		pv.Value = &pb.FeatureValue_DoubleValue{DoubleValue: v}
	case string:
		pv.Value = &pb.FeatureValue_StringValue{StringValue: v}
	case bool:
		pv.Value = &pb.FeatureValue_BoolValue{BoolValue: v}
	case []byte:
		pv.Value = &pb.FeatureValue_BytesValue{BytesValue: v}
	case []float32:
		pv.Value = &pb.FeatureValue_VectorValue{VectorValue: &pb.VectorValue{Values: v}}
	}

	return pv
}

// protoToDomainValue converts a protobuf FeatureValue to a domain value.
func protoToDomainValue(val *pb.FeatureValue) interface{} {
	switch v := val.GetValue().(type) {
	case *pb.FeatureValue_IntValue:
		return v.IntValue
	case *pb.FeatureValue_DoubleValue:
		return v.DoubleValue
	case *pb.FeatureValue_StringValue:
		return v.StringValue
	case *pb.FeatureValue_BoolValue:
		return v.BoolValue
	case *pb.FeatureValue_BytesValue:
		return v.BytesValue
	case *pb.FeatureValue_VectorValue:
		if v.VectorValue != nil {
			return v.VectorValue.GetValues()
		}
		return nil
	default:
		return nil
	}
}

// Check implements the gRPC health check service.
func (s *GRPCServer) Check(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	serviceName := req.GetService()
	if serviceName == "" || serviceName == "feather.v1.FeatureService" {
		// Check overall health
		if s.healthChecker != nil {
			if !s.healthChecker.IsHealthy() {
				return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_NOT_SERVING}, nil
			}
			if !s.healthChecker.IsReady() {
				return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_NOT_SERVING}, nil
			}
		}
		return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_SERVING}, nil
	}

	// Unknown service
	return &pb.HealthCheckResponse{Status: pb.HealthCheckResponse_SERVICE_UNKNOWN}, nil
}

// Watch implements the streaming health check service.
func (s *GRPCServer) Watch(req *pb.HealthCheckRequest, stream grpc.ServerStreamingServer[pb.HealthCheckResponse]) error {
	// Send initial status
	resp, err := s.Check(stream.Context(), req)
	if err != nil {
		return fmt.Errorf("checking health: %w", err)
	}

	if err := stream.Send(resp); err != nil {
		return fmt.Errorf("sending health response: %w", err)
	}

	// For now, we just send one response.
	// In a production implementation, this would send updates when health changes.
	<-stream.Context().Done()
	return stream.Context().Err()
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
func (h *GRPCHealthService) Check(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return h.server.Check(ctx, req)
}
