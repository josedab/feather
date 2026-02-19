package ml

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/feather-store/feather/internal/core/domain"
	"github.com/feather-store/feather/internal/core/storage"
)

func createTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.NewStore(context.Background(), storage.StoreOptions{
		HotMaxSize:   100 * 1024 * 1024,
		WarmInMemory: true,
	}, nil)
	require.NoError(t, err)
	return store
}

func TestNewBaseConnector(t *testing.T) {
	config := ConnectorConfig{
		Name:     "test-connector",
		Endpoint: "http://localhost:8080",
	}

	connector := NewBaseConnector(config)

	assert.NotNil(t, connector)
	assert.Equal(t, "test-connector", connector.Name())
	assert.False(t, connector.IsConnected())
	assert.Equal(t, 30*time.Second, connector.config.Timeout)
	assert.Equal(t, 3, connector.config.MaxRetries)
}

func TestBaseConnector_GetFeatures(t *testing.T) {
	store := createTestStore(t)

	// Add test features
	err := store.Put(context.Background(), "entity:1", map[string]*domain.FeatureValue{
		"feature_a": {Value: 1.5, Timestamp: time.Now().UnixNano(), Version: 1},
		"feature_b": {Value: "test", Timestamp: time.Now().UnixNano(), Version: 1},
	})
	require.NoError(t, err)

	connector := NewBaseConnector(ConnectorConfig{
		Name:  "test",
		Store: store,
	})

	ctx := context.Background()
	features, err := connector.GetFeatures(ctx, "entity:1", []string{"feature_a", "feature_b"})
	require.NoError(t, err)

	assert.Equal(t, 1.5, features["feature_a"])
	assert.Equal(t, "test", features["feature_b"])
}

func TestBaseConnector_BatchGetFeatures(t *testing.T) {
	store := createTestStore(t)

	// Add test features
	for i := 0; i < 3; i++ {
		err := store.Put(context.Background(), "entity:"+string(rune('1'+i)), map[string]*domain.FeatureValue{
			"feature_x": {Value: float64(i), Timestamp: time.Now().UnixNano(), Version: 1},
		})
		require.NoError(t, err)
	}

	connector := NewBaseConnector(ConnectorConfig{
		Name:  "test",
		Store: store,
	})

	ctx := context.Background()
	entityIDs := []string{"entity:1", "entity:2", "entity:3"}
	results, err := connector.BatchGetFeatures(ctx, entityIDs, []string{"feature_x"})
	require.NoError(t, err)

	assert.Len(t, results, 3)
}

func TestTensorFlowConnector(t *testing.T) {
	// Create mock TensorFlow Serving server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/models":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"model_version_status": []interface{}{},
			})

		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"predictions": []interface{}{0.85, 0.15},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	connector := NewTensorFlowConnector(TensorFlowConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "tf-test",
			Endpoint: server.URL,
		},
	})

	assert.Equal(t, "tensorflow", connector.Type())
	assert.Equal(t, "tf-test", connector.Name())

	ctx := context.Background()

	// Test connect
	err := connector.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, connector.IsConnected())

	// Test predict
	resp, err := connector.Predict(ctx, &PredictRequest{
		ModelName: "test_model",
		Features: map[string]interface{}{
			"feature_a": 1.0,
			"feature_b": 2.0,
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp.Predictions)
	assert.Equal(t, "test_model", resp.ModelName)

	// Test batch predict
	batchResp, err := connector.BatchPredict(ctx, &BatchPredictRequest{
		ModelName: "test_model",
		Features: []map[string]interface{}{
			{"feature_a": 1.0},
			{"feature_a": 2.0},
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, batchResp.Predictions)

	// Test disconnect
	err = connector.Disconnect(ctx)
	require.NoError(t, err)
	assert.False(t, connector.IsConnected())
}

func TestMLflowConnector(t *testing.T) {
	// Create mock MLflow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/2.0/mlflow/experiments/list":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"experiments": []interface{}{},
			})

		case r.URL.Path == "/invocations":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]interface{}{0.9})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	connector := NewMLflowConnector(MLflowConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "mlflow-test",
			Endpoint: server.URL,
		},
		TrackingURI: server.URL,
	})

	assert.Equal(t, "mlflow", connector.Type())

	ctx := context.Background()

	// Test connect
	err := connector.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, connector.IsConnected())

	// Test predict
	resp, err := connector.Predict(ctx, &PredictRequest{
		ModelName: "test_model",
		Features: map[string]interface{}{
			"feature_a": 1.0,
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp.Predictions)

	// Test disconnect
	err = connector.Disconnect(ctx)
	require.NoError(t, err)
	assert.False(t, connector.IsConnected())
}

func TestSageMakerConnector(t *testing.T) {
	// Create mock SageMaker endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"predictions": []interface{}{0.75},
		})
	}))
	defer server.Close()

	connector := NewSageMakerConnector(SageMakerConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "sagemaker-test",
			Endpoint: server.URL,
		},
		Region:       "us-east-1",
		EndpointName: "test-endpoint",
	})

	assert.Equal(t, "sagemaker", connector.Type())

	ctx := context.Background()

	// Test connect
	err := connector.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, connector.IsConnected())

	// Test predict
	resp, err := connector.Predict(ctx, &PredictRequest{
		ModelName: "test_model",
		Features: map[string]interface{}{
			"feature_a": 1.0,
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp.Predictions)
}

func TestSageMakerConnector_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	// Missing region
	connector := NewSageMakerConnector(SageMakerConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "sagemaker-test",
			Endpoint: "http://localhost:8080",
		},
		EndpointName: "test-endpoint",
	})
	err := connector.Connect(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "region")

	// Missing endpoint
	connector = NewSageMakerConnector(SageMakerConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "sagemaker-test",
			Endpoint: "http://localhost:8080",
		},
		Region: "us-east-1",
	})
	err = connector.Connect(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestConnectorRegistry(t *testing.T) {
	registry := NewConnectorRegistry()

	// Create mock server for connectors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	}))
	defer server.Close()

	// Test register
	tf := NewTensorFlowConnector(TensorFlowConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "tf",
			Endpoint: server.URL,
		},
	})

	err := registry.Register("tf", tf)
	require.NoError(t, err)

	// Test duplicate registration
	err = registry.Register("tf", tf)
	assert.Error(t, err)

	// Test get
	connector, err := registry.Get("tf")
	require.NoError(t, err)
	assert.Equal(t, "tf", connector.Name())

	// Test get non-existent
	_, err = registry.Get("nonexistent")
	assert.Error(t, err)

	// Test list
	connectors := registry.List()
	assert.Len(t, connectors, 1)

	// Test unregister
	err = registry.Unregister("tf")
	require.NoError(t, err)

	connectors = registry.List()
	assert.Len(t, connectors, 0)

	// Test unregister non-existent
	err = registry.Unregister("tf")
	assert.Error(t, err)
}

func TestConnectorRegistry_ConnectAll(t *testing.T) {
	registry := NewConnectorRegistry()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"model_version_status": []interface{}{},
		})
	}))
	defer server.Close()

	tf := NewTensorFlowConnector(TensorFlowConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "tf",
			Endpoint: server.URL,
		},
	})

	err := registry.Register("tf", tf)
	require.NoError(t, err)

	ctx := context.Background()

	// Connect all
	err = registry.ConnectAll(ctx)
	require.NoError(t, err)

	connector, _ := registry.Get("tf")
	assert.True(t, connector.IsConnected())

	// Disconnect all
	err = registry.DisconnectAll(ctx)
	require.NoError(t, err)
	assert.False(t, connector.IsConnected())
}

func TestPredictRequest_JSON(t *testing.T) {
	req := PredictRequest{
		ModelName:    "test_model",
		ModelVersion: "1",
		EntityID:     "entity:123",
		Features: map[string]interface{}{
			"feature_a": 1.5,
			"feature_b": "value",
		},
		Metadata: map[string]interface{}{
			"request_id": "abc123",
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded PredictRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.ModelName, decoded.ModelName)
	assert.Equal(t, req.ModelVersion, decoded.ModelVersion)
	assert.Equal(t, req.EntityID, decoded.EntityID)
}

func TestConnectorNotConnected(t *testing.T) {
	connector := NewTensorFlowConnector(TensorFlowConfig{
		ConnectorConfig: ConnectorConfig{
			Name:     "tf-test",
			Endpoint: "http://localhost:9999",
		},
	})

	ctx := context.Background()

	// Predict without connecting should fail
	_, err := connector.Predict(ctx, &PredictRequest{
		ModelName: "test",
		Features:  map[string]interface{}{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// BatchPredict without connecting should fail
	_, err = connector.BatchPredict(ctx, &BatchPredictRequest{
		ModelName: "test",
		Features:  []map[string]interface{}{},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}
