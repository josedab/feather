package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/extensions/arrowflight"
	"github.com/feather-store/feather/internal/extensions/computegraph"
	"github.com/feather-store/feather/internal/extensions/consistencyvalidator"
	"github.com/feather-store/feather/internal/extensions/embeddingmgmt"
	"github.com/feather-store/feather/internal/extensions/feastcompat"
	"github.com/feather-store/feather/internal/extensions/gitopsdefs"
	"github.com/feather-store/feather/internal/extensions/prefetch"
	"github.com/feather-store/feather/internal/extensions/sdkcodegen"
	"github.com/feather-store/feather/internal/extensions/streamcompute"
	"github.com/feather-store/feather/internal/platform/transform"
)

// ---------------------------------------------------------------------------
// PythonRuntimeHandler
// ---------------------------------------------------------------------------

// PythonRuntimeHandler exposes Python worker pool management endpoints.
type PythonRuntimeHandler struct {
	executor *transform.PythonExecutor
}

// NewPythonRuntimeHandler creates a new PythonRuntimeHandler.
func NewPythonRuntimeHandler(executor *transform.PythonExecutor) *PythonRuntimeHandler {
	return &PythonRuntimeHandler{executor: executor}
}

// RegisterRoutes registers Python runtime API routes.
func (h *PythonRuntimeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/python/workers/start", h.handleStart)
	mux.HandleFunc("GET /v1/python/workers", h.handleList)
	mux.HandleFunc("POST /v1/python/workers/reload/{transformID}", h.handleReload)
	mux.HandleFunc("GET /v1/python/workers/stats", h.handleStats)
}

func (h *PythonRuntimeHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	if err := h.executor.Start(); err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "starting workers: "+err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"status":  "started",
		"workers": h.executor.WorkerCount(),
	})
}

func (h *PythonRuntimeHandler) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"worker_count": h.executor.WorkerCount(),
	})
}

func (h *PythonRuntimeHandler) handleReload(w http.ResponseWriter, r *http.Request) {
	transformID := r.PathValue("transformID")
	// Stop and restart workers to pick up code changes.
	h.executor.Stop()
	if err := h.executor.Start(); err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, "reloading workers: "+err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"reloaded":     transformID,
		"worker_count": h.executor.WorkerCount(),
	})
}

func (h *PythonRuntimeHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"worker_count": h.executor.WorkerCount(),
	})
}

// ---------------------------------------------------------------------------
// ComputeGraphV2Handler
// ---------------------------------------------------------------------------

// ComputeGraphV2Handler exposes enhanced compute graph endpoints with
// derive statements and memoization.
type ComputeGraphV2Handler struct {
	engine   *computegraph.Engine
	memoizer *computegraph.Memoizer
}

// NewComputeGraphV2Handler creates a new ComputeGraphV2Handler.
func NewComputeGraphV2Handler(engine *computegraph.Engine, memoizer *computegraph.Memoizer) *ComputeGraphV2Handler {
	return &ComputeGraphV2Handler{engine: engine, memoizer: memoizer}
}

// RegisterRoutes registers compute graph v2 API routes.
func (h *ComputeGraphV2Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/graph/derive", h.handleDerive)
	mux.HandleFunc("POST /v1/graph/derive/batch", h.handleDeriveBatch)
	mux.HandleFunc("GET /v1/graph/memoizer/stats", h.handleMemoizerStats)
	mux.HandleFunc("POST /v1/graph/memoizer/invalidate", h.handleMemoizerInvalidate)
}

func (h *ComputeGraphV2Handler) handleDerive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Statement string `json:"statement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	spec, err := computegraph.ParseDeriveStatement(req.Statement)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := computegraph.BuildDeriveGraph(h.engine, []computegraph.DeriveSpec{*spec})
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, result)
}

func (h *ComputeGraphV2Handler) handleDeriveBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Specs []computegraph.DeriveSpec `json:"specs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := computegraph.BuildDeriveGraph(h.engine, req.Specs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, result)
}

func (h *ComputeGraphV2Handler) handleMemoizerStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.memoizer.Stats())
}

func (h *ComputeGraphV2Handler) handleMemoizerInvalidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.memoizer.Invalidate(req.Keys...)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"invalidated": len(req.Keys),
	})
}

// ---------------------------------------------------------------------------
// ConsistencyAdvancedHandler
// ---------------------------------------------------------------------------

// ConsistencyAdvancedHandler exposes advanced statistical consistency tests.
type ConsistencyAdvancedHandler struct{}

// NewConsistencyAdvancedHandler creates a new ConsistencyAdvancedHandler.
func NewConsistencyAdvancedHandler() *ConsistencyAdvancedHandler {
	return &ConsistencyAdvancedHandler{}
}

// RegisterRoutes registers consistency advanced API routes.
func (h *ConsistencyAdvancedHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/consistency/test/ks", h.handleKS)
	mux.HandleFunc("POST /v1/consistency/test/psi", h.handlePSI)
	mux.HandleFunc("POST /v1/consistency/test/chi-squared", h.handleChiSquared)
	mux.HandleFunc("POST /v1/consistency/test/jsd", h.handleJSD)
	mux.HandleFunc("POST /v1/consistency/snapshot", h.handleSnapshot)
}

func (h *ConsistencyAdvancedHandler) handleKS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Online    []float64 `json:"online"`
		Offline   []float64 `json:"offline"`
		Threshold float64   `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result := consistencyvalidator.KolmogorovSmirnov(req.Online, req.Offline, req.Threshold)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ConsistencyAdvancedHandler) handlePSI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actual    []float64 `json:"actual"`
		Expected  []float64 `json:"expected"`
		NumBins   int       `json:"num_bins"`
		Threshold float64   `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result := consistencyvalidator.PopulationStabilityIndex(req.Actual, req.Expected, req.NumBins, req.Threshold)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ConsistencyAdvancedHandler) handleChiSquared(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Observed  map[string]int `json:"observed"`
		Expected  map[string]int `json:"expected"`
		Threshold float64        `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result := consistencyvalidator.ChiSquaredTest(req.Observed, req.Expected, req.Threshold)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ConsistencyAdvancedHandler) handleJSD(w http.ResponseWriter, r *http.Request) {
	var req struct {
		P         []float64 `json:"p"`
		Q         []float64 `json:"q"`
		NumBins   int       `json:"num_bins"`
		Threshold float64   `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result := consistencyvalidator.JensenShannonDivergence(req.P, req.Q, req.NumBins, req.Threshold)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *ConsistencyAdvancedHandler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Values []float64 `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	snap := consistencyvalidator.CaptureDistribution(req.Values)
	writeJSONResponse(r.Context(), w, http.StatusOK, snap)
}

// ---------------------------------------------------------------------------
// GitOpsManifestHandler
// ---------------------------------------------------------------------------

// GitOpsManifestHandler exposes manifest loading and CI/CD generation endpoints.
type GitOpsManifestHandler struct {
	loader *gitopsdefs.ManifestLoader
}

// NewGitOpsManifestHandler creates a new GitOpsManifestHandler.
func NewGitOpsManifestHandler(loader *gitopsdefs.ManifestLoader) *GitOpsManifestHandler {
	return &GitOpsManifestHandler{loader: loader}
}

// RegisterRoutes registers GitOps manifest API routes.
func (h *GitOpsManifestHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/gitops/manifest/load", h.handleLoad)
	mux.HandleFunc("POST /v1/gitops/manifest/validate", h.handleValidate)
	mux.HandleFunc("POST /v1/gitops/cicd/generate", h.handleCICDGenerate)
}

func (h *GitOpsManifestHandler) handleLoad(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	manifest, err := h.loader.LoadFile(req.Path)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	validationErrors := h.loader.ValidateManifest(manifest)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"manifest":          manifest,
		"validation_errors": validationErrors,
	})
}

func (h *GitOpsManifestHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	manifest, err := h.loader.LoadFile(req.Path)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	validationErrors := h.loader.ValidateManifest(manifest)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"valid":  len(validationErrors) == 0,
		"errors": validationErrors,
	})
}

func (h *GitOpsManifestHandler) handleCICDGenerate(w http.ResponseWriter, r *http.Request) {
	var req gitopsdefs.CICDConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	output, err := gitopsdefs.GenerateCICD(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"provider": req.Provider,
		"output":   output,
	})
}

// ---------------------------------------------------------------------------
// ArrowBatchHandler
// ---------------------------------------------------------------------------

// ArrowBatchHandler exposes batch feature serving via columnar format.
type ArrowBatchHandler struct {
	batchServer *arrowflight.BatchServer
	converter   *arrowflight.BatchConverter
}

// NewArrowBatchHandler creates a new ArrowBatchHandler.
func NewArrowBatchHandler(bs *arrowflight.BatchServer, conv *arrowflight.BatchConverter) *ArrowBatchHandler {
	return &ArrowBatchHandler{batchServer: bs, converter: conv}
}

// RegisterRoutes registers Arrow batch API routes.
func (h *ArrowBatchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/flight/batch", h.handleBatch)
	mux.HandleFunc("POST /v1/flight/convert/columnar", h.handleConvert)
	mux.HandleFunc("GET /v1/flight/batch/stats", h.handleStats)
}

func (h *ArrowBatchHandler) handleBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entities []string `json:"entities"`
		Features []string `json:"features"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	batch, err := h.batchServer.ServeBatch(r.Context(), req.Entities, req.Features)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, batch)
}

func (h *ArrowBatchHandler) handleConvert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entities []string                          `json:"entities"`
		Features map[string]map[string]interface{} `json:"features"`
		Schema   []arrowflight.ColumnSchema        `json:"schema"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	batch, err := h.converter.FromRows(req.Entities, req.Features, req.Schema)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, batch)
}

func (h *ArrowBatchHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.batchServer.Stats())
}

// ---------------------------------------------------------------------------
// StreamAdvancedHandler
// ---------------------------------------------------------------------------

// StreamAdvancedHandler exposes exactly-once transactions, temporal joins,
// pattern matching, and late data handling endpoints.
type StreamAdvancedHandler struct {
	processor *streamcompute.ExactlyOnceProcessor
}

// NewStreamAdvancedHandler creates a new StreamAdvancedHandler.
func NewStreamAdvancedHandler(proc *streamcompute.ExactlyOnceProcessor) *StreamAdvancedHandler {
	return &StreamAdvancedHandler{processor: proc}
}

// RegisterRoutes registers stream advanced API routes.
func (h *StreamAdvancedHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/stream/transaction/begin", h.handleBegin)
	mux.HandleFunc("POST /v1/stream/transaction/{id}/commit", h.handleCommit)
	mux.HandleFunc("POST /v1/stream/transaction/{id}/rollback", h.handleRollback)
	mux.HandleFunc("POST /v1/stream/join/temporal", h.handleTemporalJoin)
	mux.HandleFunc("GET /v1/stream/patterns", h.handleListPatterns)
	mux.HandleFunc("POST /v1/stream/patterns", h.handleCreatePattern)
	mux.HandleFunc("GET /v1/stream/latedata/stats", h.handleLateDataStats)
}

func (h *StreamAdvancedHandler) handleBegin(w http.ResponseWriter, r *http.Request) {
	tx, err := h.processor.BeginTransaction()
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, tx)
}

func (h *StreamAdvancedHandler) handleCommit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tx := &streamcompute.Transaction{ID: id}
	if err := h.processor.Commit(tx); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "committed", "id": id})
}

func (h *StreamAdvancedHandler) handleRollback(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tx := &streamcompute.Transaction{ID: id}
	if err := h.processor.Rollback(tx); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "rolled_back", "id": id})
}

func (h *StreamAdvancedHandler) handleTemporalJoin(w http.ResponseWriter, r *http.Request) {
	var req streamcompute.TemporalJoinConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	join := streamcompute.NewTemporalJoin(req)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"config": req,
		"stats":  join.Stats(),
	})
}

func (h *StreamAdvancedHandler) handleListPatterns(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"patterns": []interface{}{},
	})
}

func (h *StreamAdvancedHandler) handleCreatePattern(w http.ResponseWriter, r *http.Request) {
	var req streamcompute.PatternSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	matcher := streamcompute.NewPatternMatcher(req)
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"name":  req.Name,
		"stats": matcher.Stats(),
	})
}

func (h *StreamAdvancedHandler) handleLateDataStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.processor.Stats())
}

// ---------------------------------------------------------------------------
// FeastEnhancedHandler
// ---------------------------------------------------------------------------

// FeastEnhancedHandler exposes feature service management, compatibility
// testing, and migration endpoints.
type FeastEnhancedHandler struct {
	svcMgr    *feastcompat.FeatureServiceManager
	suite     *feastcompat.CompatTestSuite
	migration *feastcompat.MigrationCLI
}

// NewFeastEnhancedHandler creates a new FeastEnhancedHandler.
func NewFeastEnhancedHandler(
	svcMgr *feastcompat.FeatureServiceManager,
	suite *feastcompat.CompatTestSuite,
	migration *feastcompat.MigrationCLI,
) *FeastEnhancedHandler {
	return &FeastEnhancedHandler{svcMgr: svcMgr, suite: suite, migration: migration}
}

// RegisterRoutes registers Feast enhanced API routes.
func (h *FeastEnhancedHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/feast/services", h.handleListServices)
	mux.HandleFunc("POST /v1/feast/services", h.handleCreateService)
	mux.HandleFunc("GET /v1/feast/services/{name}", h.handleGetService)
	mux.HandleFunc("PUT /v1/feast/services/{name}", h.handleUpdateService)
	mux.HandleFunc("DELETE /v1/feast/services/{name}", h.handleDeleteService)
	mux.HandleFunc("POST /v1/feast/services/{name}/rollback/{version}", h.handleRollback)
	mux.HandleFunc("POST /v1/feast/migrate/plan", h.handleMigratePlan)
	mux.HandleFunc("POST /v1/feast/migrate/execute", h.handleMigrateExecute)
	mux.HandleFunc("GET /v1/feast/compat/tests", h.handleListTests)
	mux.HandleFunc("POST /v1/feast/compat/run", h.handleRunTests)
}

func (h *FeastEnhancedHandler) handleListServices(w http.ResponseWriter, r *http.Request) {
	services := h.svcMgr.List()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"services": services,
		"total":    len(services),
	})
}

func (h *FeastEnhancedHandler) handleCreateService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Views       []string `json:"views"`
		Description string   `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	svc, err := h.svcMgr.Create(req.Name, req.Views, req.Description)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, svc)
}

func (h *FeastEnhancedHandler) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc, err := h.svcMgr.Get(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, svc)
}

func (h *FeastEnhancedHandler) handleUpdateService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Views []string `json:"views"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	svc, err := h.svcMgr.Update(name, req.Views)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, svc)
}

func (h *FeastEnhancedHandler) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.svcMgr.Delete(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *FeastEnhancedHandler) handleRollback(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	versionStr := r.PathValue("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid version: "+versionStr)
		return
	}
	if err := h.svcMgr.Rollback(name, version); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"version": version,
		"status":  "rolled_back",
	})
}

func (h *FeastEnhancedHandler) handleMigratePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeastConfig string `json:"feast_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	plan, err := h.migration.Plan(req.FeastConfig)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *FeastEnhancedHandler) handleMigrateExecute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeastConfig string `json:"feast_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	result, err := h.migration.Execute(req.FeastConfig)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *FeastEnhancedHandler) handleListTests(w http.ResponseWriter, r *http.Request) {
	tests := h.suite.ListTests()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tests": tests,
		"total": len(tests),
	})
}

func (h *FeastEnhancedHandler) handleRunTests(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TestName string `json:"test_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.TestName != "" {
		result := h.suite.RunTest(req.TestName)
		writeJSONResponse(r.Context(), w, http.StatusOK, result)
		return
	}
	report := h.suite.Report()
	writeJSONResponse(r.Context(), w, http.StatusOK, report)
}

// ---------------------------------------------------------------------------
// EmbeddingLifecycleHandler
// ---------------------------------------------------------------------------

// EmbeddingLifecycleHandler exposes batch embedding, A/B testing, and drift
// detection endpoints.
type EmbeddingLifecycleHandler struct {
	batch *embeddingmgmt.BatchProcessor
	ab    *embeddingmgmt.ABTester
	drift *embeddingmgmt.VectorDriftDetector
}

// NewEmbeddingLifecycleHandler creates a new EmbeddingLifecycleHandler.
func NewEmbeddingLifecycleHandler(
	batch *embeddingmgmt.BatchProcessor,
	ab *embeddingmgmt.ABTester,
	drift *embeddingmgmt.VectorDriftDetector,
) *EmbeddingLifecycleHandler {
	return &EmbeddingLifecycleHandler{batch: batch, ab: ab, drift: drift}
}

// RegisterRoutes registers embedding lifecycle API routes.
func (h *EmbeddingLifecycleHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/embeddings/batch", h.handleSubmitBatch)
	mux.HandleFunc("GET /v1/embeddings/batch/{id}", h.handleGetBatch)
	mux.HandleFunc("GET /v1/embeddings/batch", h.handleListBatch)
	mux.HandleFunc("POST /v1/embeddings/batch/{id}/cancel", h.handleCancelBatch)
	mux.HandleFunc("POST /v1/embeddings/abtest", h.handleCreateABTest)
	mux.HandleFunc("GET /v1/embeddings/abtest", h.handleListABTests)
	mux.HandleFunc("GET /v1/embeddings/abtest/{name}", h.handleGetABTest)
	mux.HandleFunc("POST /v1/embeddings/abtest/{name}/stop", h.handleStopABTest)
	mux.HandleFunc("POST /v1/embeddings/abtest/{name}/query", h.handleRouteABQuery)
	mux.HandleFunc("GET /v1/embeddings/drift", h.handleListDrift)
	mux.HandleFunc("GET /v1/embeddings/drift/{collection}/{model}", h.handleCheckDrift)
	mux.HandleFunc("POST /v1/embeddings/drift/reference", h.handleSetReference)
}

func (h *EmbeddingLifecycleHandler) handleSubmitBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ModelID    string                    `json:"model_id"`
		Collection string                    `json:"collection"`
		Items      []embeddingmgmt.BatchItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	job, err := h.batch.Submit(req.ModelID, req.Collection, req.Items)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, job)
}

func (h *EmbeddingLifecycleHandler) handleGetBatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.batch.GetJob(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, job)
}

func (h *EmbeddingLifecycleHandler) handleListBatch(w http.ResponseWriter, r *http.Request) {
	jobs := h.batch.ListJobs()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

func (h *EmbeddingLifecycleHandler) handleCancelBatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.batch.CancelJob(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"canceled": id})
}

func (h *EmbeddingLifecycleHandler) handleCreateABTest(w http.ResponseWriter, r *http.Request) {
	var req embeddingmgmt.ABTestConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	test, err := h.ab.CreateTest(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, test)
}

func (h *EmbeddingLifecycleHandler) handleListABTests(w http.ResponseWriter, r *http.Request) {
	tests := h.ab.ListTests()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tests": tests,
		"total": len(tests),
	})
}

func (h *EmbeddingLifecycleHandler) handleGetABTest(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	result, err := h.ab.GetResults(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *EmbeddingLifecycleHandler) handleStopABTest(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.ab.StopTest(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"stopped": name})
}

func (h *EmbeddingLifecycleHandler) handleRouteABQuery(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	modelID := h.ab.RouteQuery(name)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{
		"test":     name,
		"model_id": modelID,
	})
}

func (h *EmbeddingLifecycleHandler) handleListDrift(w http.ResponseWriter, r *http.Request) {
	monitors := h.drift.ListMonitored()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"monitors": monitors,
		"total":    len(monitors),
	})
}

func (h *EmbeddingLifecycleHandler) handleCheckDrift(w http.ResponseWriter, r *http.Request) {
	collection := r.PathValue("collection")
	model := r.PathValue("model")
	status, err := h.drift.CheckDrift(collection, model)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, status)
}

func (h *EmbeddingLifecycleHandler) handleSetReference(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Collection string      `json:"collection"`
		ModelID    string      `json:"model_id"`
		Vectors    [][]float64 `json:"vectors"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := h.drift.SetReference(req.Collection, req.ModelID, req.Vectors); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"collection": req.Collection,
		"model_id":   req.ModelID,
		"vectors":    len(req.Vectors),
	})
}

// ---------------------------------------------------------------------------
// SDKLanguagesHandler
// ---------------------------------------------------------------------------

// SDKLanguagesHandler exposes SDK code generation for multiple languages.
type SDKLanguagesHandler struct {
	registry *sdkcodegen.LanguageRegistry
}

// NewSDKLanguagesHandler creates a new SDKLanguagesHandler.
func NewSDKLanguagesHandler(registry *sdkcodegen.LanguageRegistry) *SDKLanguagesHandler {
	return &SDKLanguagesHandler{registry: registry}
}

// RegisterRoutes registers SDK languages API routes.
func (h *SDKLanguagesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sdk/languages", h.handleList)
	mux.HandleFunc("POST /v1/sdk/generate/{language}", h.handleGenerate)
}

func (h *SDKLanguagesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	langs := h.registry.SupportedLanguages()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"languages": langs,
		"total":     len(langs),
	})
}

func (h *SDKLanguagesHandler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	language := r.PathValue("language")
	var schema sdkcodegen.SchemaDefinition
	if err := json.NewDecoder(r.Body).Decode(&schema); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	code, err := h.registry.Generate(sdkcodegen.Language(language), &schema)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"language": language,
		"code":     code,
	})
}

// ---------------------------------------------------------------------------
// PredictiveWarmingHandler
// ---------------------------------------------------------------------------

// PredictiveWarmingHandler exposes predictive feature warming endpoints.
type PredictiveWarmingHandler struct {
	forecaster *prefetch.Forecaster
	warmer     *prefetch.Warmer
}

// NewPredictiveWarmingHandler creates a new PredictiveWarmingHandler.
func NewPredictiveWarmingHandler(forecaster *prefetch.Forecaster, warmer *prefetch.Warmer) *PredictiveWarmingHandler {
	return &PredictiveWarmingHandler{forecaster: forecaster, warmer: warmer}
}

// RegisterRoutes registers predictive warming API routes.
func (h *PredictiveWarmingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/warming/access", h.handleRecordAccess)
	mux.HandleFunc("GET /v1/warming/forecast/{feature}", h.handleForecast)
	mux.HandleFunc("GET /v1/warming/clusters", h.handleClusters)
	mux.HandleFunc("GET /v1/warming/plan", h.handlePlan)
	mux.HandleFunc("POST /v1/warming/execute", h.handleExecute)
	mux.HandleFunc("GET /v1/warming/stats", h.handleStats)
}

func (h *PredictiveWarmingHandler) handleRecordAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entity  string `json:"entity"`
		Feature string `json:"feature"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	h.forecaster.RecordAccess(req.Entity, req.Feature, time.Now())
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *PredictiveWarmingHandler) handleForecast(w http.ResponseWriter, r *http.Request) {
	feature := r.PathValue("feature")
	horizon := 1 * time.Hour
	if qs := r.URL.Query().Get("horizon"); qs != "" {
		if d, err := time.ParseDuration(qs); err == nil {
			horizon = d
		}
	}
	forecasts := h.forecaster.Forecast(feature, horizon)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"feature":   feature,
		"horizon":   horizon.String(),
		"forecasts": forecasts,
	})
}

func (h *PredictiveWarmingHandler) handleClusters(w http.ResponseWriter, r *http.Request) {
	clusters := h.forecaster.ClusterEntities()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"clusters": clusters,
		"total":    len(clusters),
	})
}

func (h *PredictiveWarmingHandler) handlePlan(w http.ResponseWriter, r *http.Request) {
	budgetBytes := int64(256 * 1024 * 1024) // 256MB default
	plan := h.forecaster.GetWarmingPlan(budgetBytes)
	writeJSONResponse(r.Context(), w, http.StatusOK, plan)
}

func (h *PredictiveWarmingHandler) handleExecute(w http.ResponseWriter, r *http.Request) {
	budgetBytes := int64(256 * 1024 * 1024)
	plan := h.forecaster.GetWarmingPlan(budgetBytes)
	result := h.warmer.ExecutePlan(plan)
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *PredictiveWarmingHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"forecaster": h.forecaster.Stats(),
		"warmer":     h.warmer.Stats(),
	})
}
