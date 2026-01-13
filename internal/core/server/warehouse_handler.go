package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/core/storage"
	"github.com/feather-store/feather/internal/integrations/warehouse"
)

// WarehouseHandler handles warehouse sync API requests.
type WarehouseHandler struct {
	engine *warehouse.SyncEngine
	store  *storage.Store
	schema storage.SchemaRegistry
	logger *slog.Logger
}

// WarehouseHandlerConfig configures the warehouse handler.
type WarehouseHandlerConfig struct {
	Engine *warehouse.SyncEngine
	Store  *storage.Store
	Schema storage.SchemaRegistry
	Logger *slog.Logger
}

// NewWarehouseHandler creates a new warehouse handler.
func NewWarehouseHandler(cfg WarehouseHandlerConfig) *WarehouseHandler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &WarehouseHandler{
		engine: cfg.Engine,
		store:  cfg.Store,
		schema: cfg.Schema,
		logger: logger,
	}
}

// RegisterRoutes registers warehouse API routes.
func (h *WarehouseHandler) RegisterRoutes(mux *http.ServeMux) {
	// Connector management
	mux.HandleFunc("GET /v1/warehouse/connectors", h.handleListConnectors)
	mux.HandleFunc("POST /v1/warehouse/connectors", h.handleRegisterConnector)
	mux.HandleFunc("GET /v1/warehouse/connectors/{id}", h.handleGetConnector)
	mux.HandleFunc("DELETE /v1/warehouse/connectors/{id}", h.handleRemoveConnector)
	mux.HandleFunc("POST /v1/warehouse/connectors/{id}/test", h.handleTestConnection)

	// Sync jobs
	mux.HandleFunc("GET /v1/warehouse/jobs", h.handleListJobs)
	mux.HandleFunc("POST /v1/warehouse/jobs", h.handleCreateJob)
	mux.HandleFunc("GET /v1/warehouse/jobs/{id}", h.handleGetJob)
	mux.HandleFunc("POST /v1/warehouse/jobs/{id}/cancel", h.handleCancelJob)

	// Sync operations
	mux.HandleFunc("POST /v1/warehouse/sync", h.handleSync)
	mux.HandleFunc("POST /v1/warehouse/sync/full", h.handleFullSync)
	mux.HandleFunc("POST /v1/warehouse/sync/incremental", h.handleIncrementalSync)
	mux.HandleFunc("GET /v1/warehouse/sync/status", h.handleSyncStatus)

	// Stats
	mux.HandleFunc("GET /v1/warehouse/stats", h.handleGetStats)
}

// Validation constants for connector credentials.
const (
	maxConnectorIDLength = 64
	maxCredentialLength  = 1024
	maxAccountLength     = 256
	maxProjectIDLength   = 128
	maxDatabaseLength    = 128
	maxSchemaLength      = 128
	maxDescriptionLength = 512
)

// Request/Response types

// WarehouseConnectorRequest represents a connector registration request.
type WarehouseConnectorRequest struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "snowflake" or "bigquery"
	Config      map[string]interface{} `json:"config"`
	Description string                 `json:"description,omitempty"`
}

// validateConnectorRequest validates a connector registration request.
func validateConnectorRequest(req *WarehouseConnectorRequest) error {
	if req.ID == "" {
		return fmt.Errorf("connector id is required")
	}
	if len(req.ID) > maxConnectorIDLength {
		return fmt.Errorf("connector id exceeds maximum length of %d characters", maxConnectorIDLength)
	}
	if req.Type == "" {
		return fmt.Errorf("connector type is required")
	}
	if req.Type != "snowflake" && req.Type != "bigquery" {
		return fmt.Errorf("unsupported connector type: %s", req.Type)
	}
	if len(req.Description) > maxDescriptionLength {
		return fmt.Errorf("description exceeds maximum length of %d characters", maxDescriptionLength)
	}
	return nil
}

// validateSnowflakeConfig validates Snowflake connector configuration.
func validateSnowflakeConfig(config map[string]interface{}) error {
	// Required fields
	account, _ := config["account"].(string)
	if account == "" {
		return fmt.Errorf("snowflake account is required")
	}
	if len(account) > maxAccountLength {
		return fmt.Errorf("account exceeds maximum length of %d characters", maxAccountLength)
	}

	user, _ := config["user"].(string)
	if user == "" {
		return fmt.Errorf("snowflake user is required")
	}
	if len(user) > maxCredentialLength {
		return fmt.Errorf("user exceeds maximum length of %d characters", maxCredentialLength)
	}

	// Password validation (required, length check, no empty string)
	password, _ := config["password"].(string)
	if password == "" {
		return fmt.Errorf("snowflake password is required")
	}
	if len(password) > maxCredentialLength {
		return fmt.Errorf("password exceeds maximum length of %d characters", maxCredentialLength)
	}

	// Optional fields with length validation
	if db, ok := config["database"].(string); ok && len(db) > maxDatabaseLength {
		return fmt.Errorf("database exceeds maximum length of %d characters", maxDatabaseLength)
	}
	if wh, ok := config["warehouse"].(string); ok && len(wh) > maxDatabaseLength {
		return fmt.Errorf("warehouse exceeds maximum length of %d characters", maxDatabaseLength)
	}
	if schema, ok := config["schema"].(string); ok && len(schema) > maxSchemaLength {
		return fmt.Errorf("schema exceeds maximum length of %d characters", maxSchemaLength)
	}
	if role, ok := config["role"].(string); ok && len(role) > maxCredentialLength {
		return fmt.Errorf("role exceeds maximum length of %d characters", maxCredentialLength)
	}

	return nil
}

// validateBigQueryConfig validates BigQuery connector configuration.
func validateBigQueryConfig(config map[string]interface{}) error {
	// Required field
	projectID, _ := config["project_id"].(string)
	if projectID == "" {
		return fmt.Errorf("bigquery project_id is required")
	}
	if len(projectID) > maxProjectIDLength {
		return fmt.Errorf("project_id exceeds maximum length of %d characters", maxProjectIDLength)
	}

	// At least one authentication method must be provided
	credsFile, hasFile := config["credentials_file"].(string)
	credsJSON, hasJSON := config["credentials_json"].(string)

	if !hasFile && !hasJSON {
		return fmt.Errorf("bigquery requires either credentials_file or credentials_json")
	}

	// Length validation for credentials
	if hasFile && len(credsFile) > maxCredentialLength {
		return fmt.Errorf("credentials_file path exceeds maximum length of %d characters", maxCredentialLength)
	}
	// credentials_json can be larger but still bounded
	if hasJSON && len(credsJSON) > maxCredentialLength*10 {
		return fmt.Errorf("credentials_json exceeds maximum length")
	}

	// Optional fields
	if dataset, ok := config["dataset"].(string); ok && len(dataset) > maxDatabaseLength {
		return fmt.Errorf("dataset exceeds maximum length of %d characters", maxDatabaseLength)
	}
	if location, ok := config["location"].(string); ok && len(location) > maxSchemaLength {
		return fmt.Errorf("location exceeds maximum length of %d characters", maxSchemaLength)
	}

	return nil
}

// SyncJobRequest represents a sync job creation request.
type SyncJobRequest struct {
	ConnectorID   string            `json:"connector_id"`
	Direction     string            `json:"direction"` // "export" or "import"
	FeatureGroups []string          `json:"feature_groups,omitempty"`
	Entities      []string          `json:"entities,omitempty"`
	Query         string            `json:"query,omitempty"`
	TargetTable   string            `json:"target_table,omitempty"`
	Mode          string            `json:"mode,omitempty"` // "full", "incremental", "append"
	Options       map[string]string `json:"options,omitempty"`
}

// SyncRequest represents a sync operation request.
type SyncRequest struct {
	ConnectorID string   `json:"connector_id"`
	Table       string   `json:"table"`
	Groups      []string `json:"groups,omitempty"`
	Since       string   `json:"since,omitempty"` // RFC3339 timestamp for incremental
}

// handleListConnectors handles GET /v1/warehouse/connectors
func (h *WarehouseHandler) handleListConnectors(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	connectors := h.engine.ListConnectors()

	response := make([]map[string]interface{}, 0, len(connectors))
	for id, connType := range connectors {
		response = append(response, map[string]interface{}{
			"id":   id,
			"type": connType,
		})
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"connectors": response,
		"count":      len(response),
	})
}

// handleRegisterConnector handles POST /v1/warehouse/connectors
func (h *WarehouseHandler) handleRegisterConnector(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	var req WarehouseConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate base request
	if err := validateConnectorRequest(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate connector-specific configuration
	var configErr error
	switch req.Type {
	case "snowflake":
		configErr = validateSnowflakeConfig(req.Config)
	case "bigquery":
		configErr = validateBigQueryConfig(req.Config)
	}
	if configErr != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, configErr.Error())
		return
	}

	var connector warehouse.Connector
	var err error

	switch req.Type {
	case "snowflake":
		connector, err = h.createSnowflakeConnector(req)
	case "bigquery":
		connector, err = h.createBigQueryConnector(req)
	}

	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.engine.RegisterConnector(req.ID, connector); err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":      true,
		"connector_id": req.ID,
	})
}

// handleGetConnector handles GET /v1/warehouse/connectors/{id}
func (h *WarehouseHandler) handleGetConnector(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	connectorID := r.PathValue("id")
	if connectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector ID required")
		return
	}

	connector, err := h.engine.GetConnector(connectorID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"id":    connectorID,
		"type":  connector.Type(),
		"state": connector.State(),
	})
}

// handleRemoveConnector handles DELETE /v1/warehouse/connectors/{id}
func (h *WarehouseHandler) handleRemoveConnector(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	connectorID := r.PathValue("id")
	if connectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector ID required")
		return
	}

	if err := h.engine.UnregisterConnector(connectorID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleTestConnection handles POST /v1/warehouse/connectors/{id}/test
func (h *WarehouseHandler) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	connectorID := r.PathValue("id")
	if connectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector ID required")
		return
	}

	connector, err := h.engine.GetConnector(connectorID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	if err := connector.Connect(r.Context()); err != nil {
		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	if err := connector.Close(); err != nil {
		h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleListJobs handles GET /v1/warehouse/jobs
func (h *WarehouseHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	status := r.URL.Query().Get("status")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	allJobs := h.engine.ListJobs()

	// Filter by status if specified
	var jobs []*warehouse.SyncJob
	for _, job := range allJobs {
		if status == "" || string(job.Mode) == status {
			jobs = append(jobs, job)
		}
		if len(jobs) >= limit {
			break
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// handleCreateJob handles POST /v1/warehouse/jobs
func (h *WarehouseHandler) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	var req SyncJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ConnectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector_id is required")
		return
	}

	// Create SyncJob from request
	job := &warehouse.SyncJob{
		ID:       "job-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Target:   req.TargetTable,
		Features: req.FeatureGroups,
		Filter:   req.Query,
		Enabled:  true,
	}

	switch req.Direction {
	case "export":
		job.Direction = warehouse.SyncDirectionExport
	case "import":
		job.Direction = warehouse.SyncDirectionImport
	default:
		job.Direction = warehouse.SyncDirectionExport
	}

	switch req.Mode {
	case "full":
		job.Mode = warehouse.SyncModeFull
	case "incremental":
		job.Mode = warehouse.SyncModeIncremental
	case "merge", "append":
		job.Mode = warehouse.SyncModeMerge
	default:
		job.Mode = warehouse.SyncModeIncremental
	}

	if err := h.engine.CreateJob(job); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"job":     job,
	})
}

// handleGetJob handles GET /v1/warehouse/jobs/{id}
func (h *WarehouseHandler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job ID required")
		return
	}

	job, err := h.engine.GetJob(jobID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, job)
}

// handleCancelJob handles POST /v1/warehouse/jobs/{id}/cancel
func (h *WarehouseHandler) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "job ID required")
		return
	}

	// Cancel is implemented by deleting the job
	if err := h.engine.DeleteJob(jobID); err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleSync handles POST /v1/warehouse/sync
func (h *WarehouseHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ConnectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector_id is required")
		return
	}
	if req.Table == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "table is required")
		return
	}

	// Create a temporary job for this sync operation
	job := &warehouse.SyncJob{
		ID:        "sync-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Target:    req.Table,
		Features:  req.Groups,
		Direction: warehouse.SyncDirectionExport,
		Mode:      warehouse.SyncModeIncremental,
		Enabled:   true,
	}

	if err := h.engine.CreateJob(job); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := h.engine.ExecuteJob(r.Context(), job.ID, req.ConnectorID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleFullSync handles POST /v1/warehouse/sync/full
func (h *WarehouseHandler) handleFullSync(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ConnectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector_id is required")
		return
	}
	if req.Table == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "table is required")
		return
	}

	// Create a temporary job for full sync
	job := &warehouse.SyncJob{
		ID:        "fullsync-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Target:    req.Table,
		Features:  req.Groups,
		Direction: warehouse.SyncDirectionExport,
		Mode:      warehouse.SyncModeFull,
		Enabled:   true,
	}

	if err := h.engine.CreateJob(job); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := h.engine.ExecuteJob(r.Context(), job.ID, req.ConnectorID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleIncrementalSync handles POST /v1/warehouse/sync/incremental
func (h *WarehouseHandler) handleIncrementalSync(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ConnectorID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "connector_id is required")
		return
	}
	if req.Table == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "table is required")
		return
	}

	// Create a temporary job for incremental sync
	job := &warehouse.SyncJob{
		ID:        "incsync-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Target:    req.Table,
		Features:  req.Groups,
		Direction: warehouse.SyncDirectionExport,
		Mode:      warehouse.SyncModeIncremental,
		Enabled:   true,
	}

	if err := h.engine.CreateJob(job); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := h.engine.ExecuteJob(r.Context(), job.ID, req.ConnectorID)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleSyncStatus handles GET /v1/warehouse/sync/status
func (h *WarehouseHandler) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	// Return stats as status information
	stats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleGetStats handles GET /v1/warehouse/stats
func (h *WarehouseHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "warehouse engine not configured")
		return
	}

	stats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// Helper methods

func (h *WarehouseHandler) createSnowflakeConnector(req WarehouseConnectorRequest) (*warehouse.SnowflakeConnector, error) {
	config := warehouse.DefaultSnowflakeConfig()

	if account, ok := req.Config["account"].(string); ok {
		config.Account = account
	}
	if user, ok := req.Config["user"].(string); ok {
		config.User = user
	}
	if password, ok := req.Config["password"].(string); ok {
		config.Password = password
	}
	if db, ok := req.Config["database"].(string); ok {
		config.Database = db
	}
	if wh, ok := req.Config["warehouse"].(string); ok {
		config.Warehouse = wh
	}
	if schema, ok := req.Config["schema"].(string); ok {
		config.Schema = schema
	}
	if role, ok := req.Config["role"].(string); ok {
		config.Role = role
	}

	return warehouse.NewSnowflakeConnector(config, h.store, h.schema, h.logger)
}

func (h *WarehouseHandler) createBigQueryConnector(req WarehouseConnectorRequest) (*warehouse.BigQueryConnector, error) {
	config := warehouse.DefaultBigQueryConfig()

	if projectID, ok := req.Config["project_id"].(string); ok {
		config.ProjectID = projectID
	}
	if dataset, ok := req.Config["dataset"].(string); ok {
		config.Dataset = dataset
	}
	if location, ok := req.Config["location"].(string); ok {
		config.Location = location
	}
	if credsFile, ok := req.Config["credentials_file"].(string); ok {
		config.CredentialsFile = credsFile
	}
	if credsJSON, ok := req.Config["credentials_json"].(string); ok {
		config.CredentialsJSON = credsJSON
	}

	return warehouse.NewBigQueryConnector(config, h.store, h.schema, h.logger)
}

func (h *WarehouseHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *WarehouseHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
