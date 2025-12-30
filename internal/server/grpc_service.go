package server

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/feather-store/feather/api/proto"
	"github.com/feather-store/feather/internal/aggregation"
	"github.com/feather-store/feather/internal/domain"
	"github.com/feather-store/feather/internal/metrics"
	"github.com/feather-store/feather/internal/storage"
)

// FeatureServiceImpl implements the gRPC FeatureService.
type FeatureServiceImpl struct {
	store       *storage.Store
	aggregation *aggregation.Engine
	schema      *storage.Registry
	metrics     *metrics.Metrics
}

// NewFeatureServiceImpl creates a new feature service implementation.
func NewFeatureServiceImpl(
	store *storage.Store,
	agg *aggregation.Engine,
	schema *storage.Registry,
	m *metrics.Metrics,
) *FeatureServiceImpl {
	return &FeatureServiceImpl{
		store:       store,
		aggregation: agg,
		schema:      schema,
		metrics:     m,
	}
}

// StreamingGetFeaturesRequest represents a streaming request.
type StreamingGetFeaturesRequest struct {
	Entities []string
	Features []string
}

// StreamingGetFeaturesResponse represents a streaming response chunk.
type StreamingGetFeaturesResponse struct {
	EntityKey string
	Features  map[string]*pb.FeatureValue
	Error     string
}

// FeatureStream is the server stream for GetFeaturesStream.
type FeatureStream interface {
	Send(*StreamingGetFeaturesResponse) error
	Context() context.Context
}

// PutFeatureStream is the client stream for PutFeaturesStream.
type PutFeatureStream interface {
	Recv() (*pb.PutFeaturesRequest, error)
	SendAndClose(*pb.PutFeaturesResponse) error
	Context() context.Context
}

// GetFeaturesStream streams features for multiple entities.
func (s *FeatureServiceImpl) GetFeaturesStream(req *StreamingGetFeaturesRequest, stream FeatureStream) error {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("GetFeaturesStream", time.Since(start))
		}
	}()

	ctx := stream.Context()

	for _, entityKey := range req.Entities {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp := &StreamingGetFeaturesResponse{
			EntityKey: entityKey,
			Features:  make(map[string]*pb.FeatureValue),
		}

		// Get features from storage
		features, err := s.store.Get(entityKey, req.Features)
		if err != nil && !domain.IsNotFound(err) {
			resp.Error = err.Error()
			if err := stream.Send(resp); err != nil {
				return err
			}
			continue
		}

		// Convert to response format
		for name, val := range features {
			resp.Features[name] = domainToProtoValue(val)
		}

		// Add computed aggregations
		for _, name := range req.Features {
			if _, ok := resp.Features[name]; ok {
				continue
			}

			if spec := s.aggregation.GetSpec(name); spec != nil {
				val, err := s.aggregation.ComputeWithSpec(entityKey, name)
				if err == nil {
					resp.Features[name] = &pb.FeatureValue{
						Value:     &pb.FeatureValue_DoubleValue{DoubleValue: val},
						Timestamp: time.Now().UnixNano(),
					}
				}
			}
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

// PutFeaturesStream accepts a stream of feature updates.
func (s *FeatureServiceImpl) PutFeaturesStream(stream PutFeatureStream) error {
	start := time.Now()
	ctx := stream.Context()

	var successCount, errorCount int64

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Process the request
		features := make(map[string]*domain.FeatureValue)
		timestamp := time.Now().UnixNano()

		for name, val := range req.GetFeatures() {
			features[name] = &domain.FeatureValue{
				Value:     protoToDomainValue(val),
				Timestamp: timestamp,
				Version:   req.GetVersion(),
			}

			// Update aggregations
			if s.aggregation.GetSpec(name) != nil {
				if floatVal, ok := features[name].Value.(float64); ok {
					s.aggregation.Update(req.GetEntityKey(), name, floatVal, time.Now())
				}
			}
		}

		if err := s.store.Put(req.GetEntityKey(), features); err != nil {
			errorCount++
			continue
		}

		successCount++
	}

	if s.metrics != nil {
		s.metrics.RecordGRPCLatency("PutFeaturesStream", time.Since(start))
	}

	return stream.SendAndClose(&pb.PutFeaturesResponse{
		Success: errorCount == 0,
		Error:   fmt.Sprintf("processed %d, errors %d", successCount, errorCount),
	})
}

// BatchGetFeatures retrieves features for multiple entities efficiently.
func (s *FeatureServiceImpl) BatchGetFeatures(ctx context.Context, entities []string, features []string) (map[string]*pb.EntityFeatures, error) {
	start := time.Now()
	defer func() {
		if s.metrics != nil {
			s.metrics.RecordGRPCLatency("BatchGetFeatures", time.Since(start))
		}
	}()

	results := make(map[string]*pb.EntityFeatures, len(entities))

	// Process in parallel for better performance
	type result struct {
		entityKey string
		features  *pb.EntityFeatures
		err       error
	}

	resultChan := make(chan result, len(entities))

	for _, entityKey := range entities {
		go func(ek string) {
			ef, err := s.getEntityFeatures(ctx, ek, features)
			resultChan <- result{entityKey: ek, features: ef, err: err}
		}(entityKey)
	}

	for i := 0; i < len(entities); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-resultChan:
			if r.err != nil && !domain.IsNotFound(r.err) {
				return nil, r.err
			}
			if r.features != nil {
				results[r.entityKey] = r.features
			}
		}
	}

	return results, nil
}

func (s *FeatureServiceImpl) getEntityFeatures(ctx context.Context, entityKey string, featureNames []string) (*pb.EntityFeatures, error) {
	features, err := s.store.Get(entityKey, featureNames)
	if err != nil && !domain.IsNotFound(err) {
		return nil, err
	}

	result := &pb.EntityFeatures{
		Features: make(map[string]*pb.FeatureValue),
	}

	for name, val := range features {
		result.Features[name] = domainToProtoValue(val)
	}

	// Compute aggregations for missing features
	for _, name := range featureNames {
		if _, ok := result.Features[name]; ok {
			continue
		}

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
