package server

import (
	"context"
	"net/http"

	"github.com/feather-store/feather/internal/extensions/benchpub"
)

// BenchPubHandler handles benchmark API requests.
type BenchPubHandler struct {
	suite *benchpub.Suite
}

// NewBenchPubHandler creates a new benchmark handler.
func NewBenchPubHandler(suite *benchpub.Suite) *BenchPubHandler {
	return &BenchPubHandler{suite: suite}
}

// RegisterRoutes registers benchmark API routes.
func (h *BenchPubHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/benchmarks/run", h.handleRunBenchmark)
	mux.HandleFunc("GET /v1/benchmarks/results", h.handleListResults)
	mux.HandleFunc("GET /v1/benchmarks/results/{name}", h.handleGetResult)
	mux.HandleFunc("POST /v1/benchmarks/compare", h.handleCompare)
	mux.HandleFunc("GET /v1/benchmarks/stats", h.handleGetStats)
}

// handleRunBenchmark handles POST /v1/benchmarks/run
func (h *BenchPubHandler) handleRunBenchmark(w http.ResponseWriter, r *http.Request) {
	if h.suite == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "benchmark suite not configured")
		return
	}

	var cfg benchpub.BenchmarkConfig
	if err := strictDecode(r.Body, &cfg); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.suite.Run(cfg)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleListResults handles GET /v1/benchmarks/results
func (h *BenchPubHandler) handleListResults(w http.ResponseWriter, r *http.Request) {
	if h.suite == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "benchmark suite not configured")
		return
	}

	results := h.suite.ListResults()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
	})
}

// handleGetResult handles GET /v1/benchmarks/results/{name}
func (h *BenchPubHandler) handleGetResult(w http.ResponseWriter, r *http.Request) {
	if h.suite == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "benchmark suite not configured")
		return
	}

	name := r.PathValue("name")
	if name == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "benchmark name is required")
		return
	}

	result, err := h.suite.GetResult(name)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, result)
}

// handleCompare handles POST /v1/benchmarks/compare
func (h *BenchPubHandler) handleCompare(w http.ResponseWriter, r *http.Request) {
	if h.suite == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "benchmark suite not configured")
		return
	}

	var req struct {
		Names []string `json:"names"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	report, err := h.suite.Compare(req.Names)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, report)
}

// handleGetStats handles GET /v1/benchmarks/stats
func (h *BenchPubHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.suite == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "benchmark suite not configured")
		return
	}

	stats := h.suite.Stats()

	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *BenchPubHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *BenchPubHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
