package ingestion

import (
	"testing"
	"time"
)

func TestJSONDecoder_MalformedMessages(t *testing.T) {
	dec := &JSONDecoder{}

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"valid message", []byte(`{"entity_key":"user:1","features":{"score":0.95}}`), false},
		{"empty JSON object", []byte(`{}`), false},
		{"with timestamp", []byte(`{"entity_key":"user:1","features":{"score":0.5},"timestamp":1704067200}`), false},
		{"with version", []byte(`{"entity_key":"user:1","features":{"score":0.5},"version":2}`), false},
		{"empty bytes", []byte(``), true},
		{"invalid JSON", []byte(`not json`), true},
		{"truncated JSON", []byte(`{"entity_key":"user:1`), true},
		{"null value", []byte(`null`), false}, // decodes to zero-value struct
		{"array instead of object", []byte(`[1,2,3]`), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update, err := dec.Decode(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if update == nil {
				t.Error("expected non-nil update")
			}
		})
	}
}

func TestJSONDecoder_FeatureTypes(t *testing.T) {
	dec := &JSONDecoder{}

	data := []byte(`{
		"entity_key": "user:42",
		"features": {
			"float_val": 3.14,
			"int_val": 100,
			"str_val": "hello",
			"bool_val": true
		},
		"timestamp": 1704067200000000000,
		"version": 5
	}`)

	update, err := dec.Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if update.EntityKey != "user:42" {
		t.Errorf("EntityKey = %q, want %q", update.EntityKey, "user:42")
	}
	if len(update.Features) != 4 {
		t.Errorf("got %d features, want 4", len(update.Features))
	}
	if update.Timestamp != 1704067200000000000 {
		t.Errorf("Timestamp = %d, want 1704067200000000000", update.Timestamp)
	}
	if update.Version != 5 {
		t.Errorf("Version = %d, want 5", update.Version)
	}
}

func TestJSONDecoder_EmptyFeatures(t *testing.T) {
	dec := &JSONDecoder{}

	data := []byte(`{"entity_key":"user:1","features":{}}`)
	update, err := dec.Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(update.Features) != 0 {
		t.Errorf("expected empty features, got %d", len(update.Features))
	}
}

func TestJSONDecoder_MissingFields(t *testing.T) {
	dec := &JSONDecoder{}

	// Missing entity_key - should parse without error but entity_key will be empty
	data := []byte(`{"features":{"score":0.5}}`)
	update, err := dec.Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update.EntityKey != "" {
		t.Errorf("expected empty EntityKey, got %q", update.EntityKey)
	}

	// Missing features
	data = []byte(`{"entity_key":"user:1"}`)
	update, err = dec.Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if update.Features != nil && len(update.Features) != 0 {
		t.Errorf("expected nil/empty features")
	}
}

func TestKafkaConfig_Defaults(t *testing.T) {
	config := KafkaConfig{
		Brokers:       []string{"localhost:9092"},
		Topic:         "features",
		ConsumerGroup: "test-group",
	}

	if len(config.Brokers) != 1 {
		t.Errorf("expected 1 broker, got %d", len(config.Brokers))
	}
	if config.AutoOffset != "" {
		t.Errorf("AutoOffset should be empty by default, got %q", config.AutoOffset)
	}
	if config.CircuitBreakerEnabled {
		t.Error("CircuitBreakerEnabled should be false by default")
	}
}

func TestKafkaConfig_SecurityFields(t *testing.T) {
	config := KafkaConfig{
		Brokers:          []string{"broker1:9092", "broker2:9092"},
		Topic:            "features",
		ConsumerGroup:    "group",
		SecurityProtocol: "SASL_SSL",
		SASLMechanism:    "SCRAM-SHA-256",
		SASLUsername:      "user",
		SASLPassword:      "pass",
		SSLCAFile:        "/path/to/ca.pem",
		SSLCertFile:      "/path/to/cert.pem",
		SSLKeyFile:       "/path/to/key.pem",
	}

	if config.SecurityProtocol != "SASL_SSL" {
		t.Errorf("SecurityProtocol = %q, want SASL_SSL", config.SecurityProtocol)
	}
	if len(config.Brokers) != 2 {
		t.Errorf("expected 2 brokers, got %d", len(config.Brokers))
	}
}

func TestIngestionMetrics_ZeroValues(t *testing.T) {
	m := &IngestionMetrics{}

	if m.MessagesReceived != 0 {
		t.Errorf("MessagesReceived = %d, want 0", m.MessagesReceived)
	}
	if m.MessagesSuccess != 0 {
		t.Errorf("MessagesSuccess = %d, want 0", m.MessagesSuccess)
	}
	if m.MessagesError != 0 {
		t.Errorf("MessagesError = %d, want 0", m.MessagesError)
	}
	if m.BytesReceived != 0 {
		t.Errorf("BytesReceived = %d, want 0", m.BytesReceived)
	}
}

func TestCircuitBreaker_SuccessDoesNotOpenCircuit(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Second)

	// Record some successes - circuit should stay closed
	for i := 0; i < 10; i++ {
		cb.RecordSuccess()
	}

	if cb.State() != CircuitClosed {
		t.Errorf("expected CircuitClosed after successes, got %d", cb.State())
	}
	if !cb.Allow() {
		t.Error("closed circuit should allow requests")
	}
}

func TestCircuitBreaker_MixedFailuresAndSuccesses(t *testing.T) {
	cb := NewCircuitBreaker(5, time.Second)

	// Record failures below threshold
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitClosed {
		t.Error("circuit should still be closed below threshold")
	}

	// Successes don't reset failure count when closed
	cb.RecordSuccess()

	// Continue failures to reach threshold
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen, got %d", cb.State())
	}
}

func TestCircuitBreaker_ExactThreshold(t *testing.T) {
	cb := NewCircuitBreaker(1, time.Second)

	// Single failure should open circuit with threshold=1
	cb.RecordFailure()

	if cb.State() != CircuitOpen {
		t.Errorf("expected CircuitOpen with threshold=1, got %d", cb.State())
	}
}
