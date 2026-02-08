package ingestion

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/feather-store/feather/internal/core/domain"
)

// CircuitBreaker implements a simple circuit breaker pattern.
type CircuitBreaker struct {
	failures    int64
	successes   int64
	state       int32 // 0=closed, 1=open, 2=half-open
	lastFailure int64
	threshold   int64
	timeout     time.Duration
}

// CircuitBreakerState represents the circuit breaker state.
type CircuitBreakerState int32

// CircuitBreakerState constants.
const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(threshold int64, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
	}
}

// Allow checks if a request should be allowed.
func (cb *CircuitBreaker) Allow() bool {
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	switch state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed
		lastFailure := atomic.LoadInt64(&cb.lastFailure)
		if time.Since(time.Unix(0, lastFailure)) > cb.timeout {
			// Transition to half-open
			atomic.CompareAndSwapInt32(&cb.state, int32(CircuitOpen), int32(CircuitHalfOpen))
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return true
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	atomic.AddInt64(&cb.successes, 1)
	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	if state == CircuitHalfOpen {
		// Reset failures and close circuit
		atomic.StoreInt64(&cb.failures, 0)
		atomic.StoreInt32(&cb.state, int32(CircuitClosed))
	}
}

// RecordFailure records a failed operation.
func (cb *CircuitBreaker) RecordFailure() {
	failures := atomic.AddInt64(&cb.failures, 1)
	atomic.StoreInt64(&cb.lastFailure, time.Now().UnixNano())

	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))

	if state == CircuitHalfOpen {
		// Back to open
		atomic.StoreInt32(&cb.state, int32(CircuitOpen))
	} else if state == CircuitClosed && failures >= cb.threshold {
		// Open the circuit
		atomic.StoreInt32(&cb.state, int32(CircuitOpen))
	}
}

// State returns the current state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&cb.state))
}

// Reset resets the circuit breaker.
func (cb *CircuitBreaker) Reset() {
	atomic.StoreInt64(&cb.failures, 0)
	atomic.StoreInt64(&cb.successes, 0)
	atomic.StoreInt32(&cb.state, int32(CircuitClosed))
}

// KafkaConfig configures the Kafka consumer.
type KafkaConfig struct {
	Brokers                 []string
	Topic                   string
	ConsumerGroup           string
	AutoOffset              string // "earliest" or "latest"
	CircuitBreakerEnabled   bool
	CircuitBreakerThreshold int64         // Number of failures before opening
	CircuitBreakerTimeout   time.Duration // Time before attempting recovery

	// Security configuration
	SecurityProtocol string // "PLAINTEXT", "SSL", "SASL_PLAINTEXT", "SASL_SSL"
	SASLMechanism    string // "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"
	SASLUsername     string
	SASLPassword     string
	SSLCAFile        string
	SSLCertFile      string
	SSLKeyFile       string
}

// FeatureDecoder decodes feature updates from message bytes.
type FeatureDecoder interface {
	Decode(data []byte) (*domain.FeatureUpdate, error)
}

// JSONDecoder decodes JSON-encoded feature updates.
type JSONDecoder struct{}

// Decode parses a JSON-encoded feature update.
func (d *JSONDecoder) Decode(data []byte) (*domain.FeatureUpdate, error) {
	var update domain.FeatureUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		return nil, err
	}
	return &update, nil
}

// IngestionMetrics tracks ingestion performance.
type IngestionMetrics struct { //nolint:revive
	MessagesReceived int64
	MessagesSuccess  int64
	MessagesError    int64
	BytesReceived    int64
	LastMessageTime  int64
}
