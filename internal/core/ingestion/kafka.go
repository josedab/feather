//go:build cgo

package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/feather-store/feather/internal/core/aggregation"
	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

// KafkaConsumer consumes feature updates from Kafka.
type KafkaConsumer struct {
	consumer       *kafka.Consumer
	store          *storage.Store
	agg            *aggregation.Engine
	decoder        FeatureDecoder
	metrics        *IngestionMetrics
	circuitBreaker *CircuitBreaker
	running        int32
	logger         *slog.Logger
}

// NewKafkaConsumer creates a new Kafka consumer.
func NewKafkaConsumer(
	config KafkaConfig,
	store *storage.Store,
	agg *aggregation.Engine,
	logger *slog.Logger,
) (*KafkaConsumer, error) {
	autoOffset := config.AutoOffset
	if autoOffset == "" {
		autoOffset = "latest"
	}

	brokers := ""
	for i, b := range config.Brokers {
		if i > 0 {
			brokers += ","
		}
		brokers += b
	}

	configMap := &kafka.ConfigMap{
		"bootstrap.servers":  brokers,
		"group.id":           config.ConsumerGroup,
		"auto.offset.reset":  autoOffset,
		"enable.auto.commit": false,
	}

	// Configure security protocol
	if config.SecurityProtocol != "" {
		if err := configMap.SetKey("security.protocol", config.SecurityProtocol); err != nil {
			return nil, fmt.Errorf("setting security protocol: %w", err)
		}
	}

	// Configure SASL authentication
	if config.SASLMechanism != "" {
		if err := configMap.SetKey("sasl.mechanism", config.SASLMechanism); err != nil {
			return nil, fmt.Errorf("setting sasl mechanism: %w", err)
		}
	}
	if config.SASLUsername != "" {
		if err := configMap.SetKey("sasl.username", config.SASLUsername); err != nil {
			return nil, fmt.Errorf("setting sasl username: %w", err)
		}
	}
	if config.SASLPassword != "" {
		if err := configMap.SetKey("sasl.password", config.SASLPassword); err != nil {
			return nil, fmt.Errorf("setting sasl password: %w", err)
		}
	}

	// Configure SSL/TLS
	if config.SSLCAFile != "" {
		if err := configMap.SetKey("ssl.ca.location", config.SSLCAFile); err != nil {
			return nil, fmt.Errorf("setting ssl ca location: %w", err)
		}
	}
	if config.SSLCertFile != "" {
		if err := configMap.SetKey("ssl.certificate.location", config.SSLCertFile); err != nil {
			return nil, fmt.Errorf("setting ssl certificate location: %w", err)
		}
	}
	if config.SSLKeyFile != "" {
		if err := configMap.SetKey("ssl.key.location", config.SSLKeyFile); err != nil {
			return nil, fmt.Errorf("setting ssl key location: %w", err)
		}
	}

	c, err := kafka.NewConsumer(configMap)
	if err != nil {
		return nil, fmt.Errorf("creating kafka consumer: %w", err)
	}

	if err := c.Subscribe(config.Topic, nil); err != nil {
		if closeErr := c.Close(); closeErr != nil {
			return nil, fmt.Errorf("closing kafka consumer: %w", closeErr)
		}
		return nil, fmt.Errorf("subscribing to topic: %w", err)
	}

	kc := &KafkaConsumer{
		consumer: c,
		store:    store,
		agg:      agg,
		decoder:  &JSONDecoder{},
		metrics:  &IngestionMetrics{},
		logger:   logger,
	}

	// Initialize circuit breaker if enabled
	if config.CircuitBreakerEnabled {
		threshold := config.CircuitBreakerThreshold
		if threshold == 0 {
			threshold = 5 // default: 5 failures
		}
		timeout := config.CircuitBreakerTimeout
		if timeout == 0 {
			timeout = 30 * time.Second // default: 30 seconds
		}
		kc.circuitBreaker = NewCircuitBreaker(threshold, timeout)
	}

	return kc, nil
}

// Start begins consuming from Kafka.
func (k *KafkaConsumer) Start(ctx context.Context) error {
	atomic.StoreInt32(&k.running, 1)

	for atomic.LoadInt32(&k.running) == 1 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Check circuit breaker
			if k.circuitBreaker != nil && !k.circuitBreaker.Allow() {
				// Circuit is open, wait before retrying
				time.Sleep(100 * time.Millisecond)
				continue
			}

			msg, err := k.consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				// Timeout is not an error
				var kafkaErr kafka.Error
				if errors.As(err, &kafkaErr) && kafkaErr.Code() == kafka.ErrTimedOut {
					continue
				}
				atomic.AddInt64(&k.metrics.MessagesError, 1)
				if k.circuitBreaker != nil {
					k.circuitBreaker.RecordFailure()
				}
				continue
			}

			atomic.AddInt64(&k.metrics.MessagesReceived, 1)
			atomic.AddInt64(&k.metrics.BytesReceived, int64(len(msg.Value)))
			atomic.StoreInt64(&k.metrics.LastMessageTime, time.Now().UnixNano())

			if err := k.processMessage(ctx, msg); err != nil {
				atomic.AddInt64(&k.metrics.MessagesError, 1)
				if k.circuitBreaker != nil {
					k.circuitBreaker.RecordFailure()
				}
				continue
			}

			atomic.AddInt64(&k.metrics.MessagesSuccess, 1)
			if k.circuitBreaker != nil {
				k.circuitBreaker.RecordSuccess()
			}

			// Commit offset
			if _, err := k.consumer.CommitMessage(msg); err != nil {
				if k.logger != nil {
					k.logger.Warn("failed to commit Kafka offset",
						"error", err,
						"topic", *msg.TopicPartition.Topic,
						"partition", msg.TopicPartition.Partition,
						"offset", msg.TopicPartition.Offset,
					)
				}
			}
		}
	}

	return nil
}

// CircuitBreakerStatus returns the current circuit breaker state, or nil if not enabled.
func (k *KafkaConsumer) CircuitBreakerStatus() *CircuitBreakerState {
	if k.circuitBreaker == nil {
		return nil
	}
	state := k.circuitBreaker.State()
	return &state
}

// Stop stops the consumer.
func (k *KafkaConsumer) Stop() {
	atomic.StoreInt32(&k.running, 0)
}

// Close closes the consumer.
func (k *KafkaConsumer) Close() error {
	k.Stop()
	return k.consumer.Close()
}

func (k *KafkaConsumer) processMessage(ctx context.Context, msg *kafka.Message) error {
	update, err := k.decoder.Decode(msg.Value)
	if err != nil {
		return fmt.Errorf("decoding message: %w", err)
	}

	if update.Timestamp == 0 {
		update.Timestamp = time.Now().UnixNano()
	}

	// Store features
	features := make(map[string]*domain.FeatureValue)
	for name, val := range update.Features {
		features[name] = &domain.FeatureValue{
			Value:     val,
			Timestamp: update.Timestamp,
			Version:   update.Version,
		}
	}

	if err := k.store.Put(ctx, update.EntityKey, features); err != nil {
		return fmt.Errorf("storing features: %w", err)
	}

	// Update aggregations
	for name, val := range update.Features {
		if k.agg.GetSpec(name) != nil {
			if floatVal, ok := domain.ToFloat64(val); ok {
				if err := k.agg.Update(update.EntityKey, name, floatVal, time.Unix(0, update.Timestamp)); err != nil {
					return fmt.Errorf("updating aggregation: %w", err)
				}
			}
		}
	}

	return nil
}

// Metrics returns current metrics.
func (k *KafkaConsumer) Metrics() IngestionMetrics {
	return IngestionMetrics{
		MessagesReceived: atomic.LoadInt64(&k.metrics.MessagesReceived),
		MessagesSuccess:  atomic.LoadInt64(&k.metrics.MessagesSuccess),
		MessagesError:    atomic.LoadInt64(&k.metrics.MessagesError),
		BytesReceived:    atomic.LoadInt64(&k.metrics.BytesReceived),
		LastMessageTime:  atomic.LoadInt64(&k.metrics.LastMessageTime),
	}
}
