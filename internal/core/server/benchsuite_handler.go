package server

import (
	"net/http"

	"github.com/feather-store/feather/internal/tools/benchsuite"
)

// BenchSuiteHandler provides HTTP endpoints for the benchmark suite.
type BenchSuiteHandler struct {
	suite *benchsuite.Suite
}

// NewBenchSuiteHandler creates a new benchmark suite handler.
func NewBenchSuiteHandler(suite *benchsuite.Suite) *BenchSuiteHandler {
	return &BenchSuiteHandler{suite: suite}
}

// RegisterRoutes registers benchmark suite API routes.
func (h *BenchSuiteHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/benchmarks/runs", h.handleCreateRun)
	mux.HandleFunc("GET /v1/benchmarks/runs", h.handleListRuns)
	mux.HandleFunc("GET /v1/benchmarks/runs/{id}", h.handleGetRun)
	mux.HandleFunc("POST /v1/benchmarks/runs/{id}/execute", h.handleExecuteRun)
	mux.HandleFunc("DELETE /v1/benchmarks/runs/{id}", h.handleDeleteRun)
	mux.HandleFunc("GET /v1/benchmarks/runs/{id}/report", h.handleReport)
	mux.HandleFunc("POST /v1/benchmarks/compare", h.handleCompare)
	mux.HandleFunc("GET /v1/benchmarks/stats", h.handleStats)
}

func (h *BenchSuiteHandler) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Workload string `json:"workload"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	run, err := h.suite.CreateRun(req.Name, benchsuite.WorkloadType(req.Workload))
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSONResponse(r.Context(), w, http.StatusCreated, run)
}

func (h *BenchSuiteHandler) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs := h.suite.ListRuns()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"runs":  runs,
		"total": len(runs),
	})
}

func (h *BenchSuiteHandler) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := h.suite.GetRun(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, run)
}

func (h *BenchSuiteHandler) handleExecuteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	results, err := h.suite.RunBenchmark(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, results)
}

func (h *BenchSuiteHandler) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.suite.DeleteRun(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *BenchSuiteHandler) handleReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	report, err := h.suite.GenerateReport(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"report": report})
}

func (h *BenchSuiteHandler) handleCompare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := strictDecode(r.Body, &req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	report, err := h.suite.Compare(req.IDs)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, report)
}

func (h *BenchSuiteHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.suite.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
