//go:build !cgo

package ingestion

import (
	"context"
	"errors"
	"log/slog"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/storage"
)

// KafkaConsumer is a stub when built without CGO.
// Build with CGO_ENABLED=1 and librdkafka to enable Kafka support.
type KafkaConsumer struct{}

// NewKafkaConsumer returns an error when built without CGO.
func NewKafkaConsumer(
	_ KafkaConfig,
	_ *storage.Store,
	_ *aggregation.Engine,
	_ *slog.Logger,
) (*KafkaConsumer, error) {
	return nil, errors.New("Kafka support requires CGO (build with CGO_ENABLED=1 and librdkafka installed)")
}

// Start is a no-op stub.
func (k *KafkaConsumer) Start(_ context.Context) error {
	return errors.New("kafka not available: built without CGO")
}

// CircuitBreakerStatus returns nil (no circuit breaker in stub).
func (k *KafkaConsumer) CircuitBreakerStatus() *CircuitBreakerState { return nil }

// Stop is a no-op stub.
func (k *KafkaConsumer) Stop() {}

// Close is a no-op stub.
func (k *KafkaConsumer) Close() error { return nil }

// Metrics returns zero-value metrics.
func (k *KafkaConsumer) Metrics() IngestionMetrics { return IngestionMetrics{} }
