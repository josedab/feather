package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/ml"
	"github.com/feather-store/feather/internal/storage"
)

// MLHandler handles ML connector API requests.
type MLHandler struct {
	registry *ml.ConnectorRegistry
	store    *storage.Store
}

// NewMLHandler creates a new ML handler.
func NewMLHandler(store *storage.Store) *MLHandler {
	return &MLHandler{
		registry: ml.NewConnectorRegistry(),
		store:    store,
	}
}

// RegisterRoutes registers ML API routes.
func (h *MLHandler) RegisterRoutes(mux *http.ServeMux) {
	// Connector management
	mux.HandleFunc("GET /v1/ml/connectors", h.handleListConnectors)
	mux.HandleFunc("POST /v1/ml/connectors", h.handleRegisterConnector)
	mux.HandleFunc("GET /v1/ml/connectors/{name}", h.handleGetConnector)
	mux.HandleFunc("DELETE /v1/ml/connectors/{name}", h.handleUnregisterConnector)
	mux.HandleFunc("POST /v1/ml/connectors/{name}/connect", h.handleConnect)
	mux.HandleFunc("POST /v1/ml/connectors/{name}/disconnect", h.handleDisconnect)

	// Predictions
	mux.HandleFunc("POST /v1/ml/predict", h.handlePredict)
	mux.HandleFunc("POST /v1/ml/predict/batch", h.handleBatchPredict)
}

// ConnectorRequest represents a connector registration request.
type ConnectorRequest struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"` // tensorflow, mlflow, sagemaker
	Endpoint     string            `json:"endpoint"`
	Region       string            `json:"region,omitempty"`       // for sagemaker
	EndpointName string            `json:"endpoint_name,omitempty"` // for sagemaker
	TrackingURI  string            `json:"tracking_uri,omitempty"`  // for mlflow
	Headers      map[string]string `json:"headers,omitempty"`
}

// handleListConnectors handles GET /v1/ml/connectors
func (h *MLHandler) handleListConnectors(w http.ResponseWriter, r *http.Request) {
	connectors := h.registry.List()

	result := make([]map[string]interface{}, len(connectors))
	for i, c := range connectors {
		result[i] = map[string]interface{}{
			"name":      c.Name(),
			"type":      c.Type(),
			"connected": c.IsConnected(),
		}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"connectors": result,
		"count":      len(result),
	})
}

// handleRegisterConnector handles POST /v1/ml/connectors
func (h *MLHandler) handleRegisterConnector(w http.ResponseWriter, r *http.Request) {
	var req ConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Type == "" {
		h.writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	var connector ml.Connector
	baseConfig := ml.ConnectorConfig{
		Name:     req.Name,
		Endpoint: req.Endpoint,
		Headers:  req.Headers,
		Store:    h.store,
	}

	switch req.Type {
	case "tensorflow":
		connector = ml.NewTensorFlowConnector(ml.TensorFlowConfig{
			ConnectorConfig: baseConfig,
		})

	case "mlflow":
		connector = ml.NewMLflowConnector(ml.MLflowConfig{
			ConnectorConfig: baseConfig,
			TrackingURI:     req.TrackingURI,
		})

	case "sagemaker":
		connector = ml.NewSageMakerConnector(ml.SageMakerConfig{
			ConnectorConfig: baseConfig,
			Region:          req.Region,
			EndpointName:    req.EndpointName,
		})

	default:
		h.writeError(w, http.StatusBadRequest, "unsupported connector type: "+req.Type)
		return
	}

	if err := h.registry.Register(req.Name, connector); err != nil {
		h.writeError(w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"name":    req.Name,
		"type":    req.Type,
	})
}

// handleGetConnector handles GET /v1/ml/connectors/{name}
func (h *MLHandler) handleGetConnector(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "connector name required")
		return
	}

	connector, err := h.registry.Get(name)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":      connector.Name(),
		"type":      connector.Type(),
		"connected": connector.IsConnected(),
	})
}

// handleUnregisterConnector handles DELETE /v1/ml/connectors/{name}
func (h *MLHandler) handleUnregisterConnector(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "connector name required")
		return
	}

	// Disconnect first if connected
	connector, err := h.registry.Get(name)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if connector.IsConnected() {
		connector.Disconnect(r.Context())
	}

	if err := h.registry.Unregister(name); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleConnect handles POST /v1/ml/connectors/{name}/connect
func (h *MLHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "connector name required")
		return
	}

	connector, err := h.registry.Get(name)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := connector.Connect(r.Context()); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"connected": true,
	})
}

// handleDisconnect handles POST /v1/ml/connectors/{name}/disconnect
func (h *MLHandler) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		h.writeError(w, http.StatusBadRequest, "connector name required")
		return
	}

	connector, err := h.registry.Get(name)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := connector.Disconnect(r.Context()); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"connected": false,
	})
}

// PredictAPIRequest represents an API prediction request.
type PredictAPIRequest struct {
	Connector    string                 `json:"connector"`
	ModelName    string                 `json:"model_name"`
	ModelVersion string                 `json:"model_version,omitempty"`
	EntityID     string                 `json:"entity_id,omitempty"`
	FeatureNames []string               `json:"feature_names,omitempty"`
	Features     map[string]interface{} `json:"features,omitempty"`
}

// handlePredict handles POST /v1/ml/predict
func (h *MLHandler) handlePredict(w http.ResponseWriter, r *http.Request) {
	var req PredictAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Connector == "" {
		h.writeError(w, http.StatusBadRequest, "connector is required")
		return
	}

	if req.ModelName == "" {
		h.writeError(w, http.StatusBadRequest, "model_name is required")
		return
	}

	connector, err := h.registry.Get(req.Connector)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Get features from store if entity ID is provided
	features := req.Features
	if features == nil && req.EntityID != "" && len(req.FeatureNames) > 0 {
		features, err = connector.GetFeatures(r.Context(), req.EntityID, req.FeatureNames)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to get features: "+err.Error())
			return
		}
	}

	resp, err := connector.Predict(r.Context(), &ml.PredictRequest{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		EntityID:     req.EntityID,
		Features:     features,
	})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"model_name":    resp.ModelName,
		"model_version": resp.ModelVersion,
		"predictions":   resp.Predictions,
		"latency_ms":    resp.Latency.Milliseconds(),
	})
}

// BatchPredictAPIRequest represents a batch prediction API request.
type BatchPredictAPIRequest struct {
	Connector    string                   `json:"connector"`
	ModelName    string                   `json:"model_name"`
	ModelVersion string                   `json:"model_version,omitempty"`
	EntityIDs    []string                 `json:"entity_ids,omitempty"`
	FeatureNames []string                 `json:"feature_names,omitempty"`
	Features     []map[string]interface{} `json:"features,omitempty"`
}

// handleBatchPredict handles POST /v1/ml/predict/batch
func (h *MLHandler) handleBatchPredict(w http.ResponseWriter, r *http.Request) {
	var req BatchPredictAPIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Connector == "" {
		h.writeError(w, http.StatusBadRequest, "connector is required")
		return
	}

	if req.ModelName == "" {
		h.writeError(w, http.StatusBadRequest, "model_name is required")
		return
	}

	connector, err := h.registry.Get(req.Connector)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Get features from store if entity IDs are provided
	features := req.Features
	if features == nil && len(req.EntityIDs) > 0 && len(req.FeatureNames) > 0 {
		features, err = connector.BatchGetFeatures(r.Context(), req.EntityIDs, req.FeatureNames)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to get features: "+err.Error())
			return
		}
	}

	resp, err := connector.BatchPredict(r.Context(), &ml.BatchPredictRequest{
		ModelName:    req.ModelName,
		ModelVersion: req.ModelVersion,
		EntityIDs:    req.EntityIDs,
		Features:     features,
	})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"model_name":    resp.ModelName,
		"model_version": resp.ModelVersion,
		"predictions":   resp.Predictions,
		"count":         len(resp.Predictions),
		"latency_ms":    resp.Latency.Milliseconds(),
	})
}

func (h *MLHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(w, status, data)
}

func (h *MLHandler) writeError(w http.ResponseWriter, status int, message string) {
	writeJSONError(w, status, message)
}
