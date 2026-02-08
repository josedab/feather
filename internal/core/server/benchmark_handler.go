package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/tools/benchmark"
	"github.com/feather-store/feather/internal/core/storage"
)

// BenchmarkHandler handles benchmark API requests.
type BenchmarkHandler struct {
	store *storage.Store
}

// NewBenchmarkHandler creates a new benchmark handler.
func NewBenchmarkHandler(store *storage.Store) *BenchmarkHandler {
	return &BenchmarkHandler{store: store}
}

// RegisterRoutes registers benchmark API routes.
func (h *BenchmarkHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/benchmark/run", h.handleRunBenchmark)
	mux.HandleFunc("GET /v1/benchmark/config", h.handleGetDefaultConfig)
}

// BenchmarkRequest represents a benchmark run request.
type BenchmarkRequest struct {
	NumEntities   int `json:"num_entities,omitempty"`
	NumFeatures   int `json:"num_features,omitempty"`
	NumOperations int `json:"num_operations,omitempty"`
	Concurrency   int `json:"concurrency,omitempty"`
	DataSize      int `json:"data_size,omitempty"`
}

// handleRunBenchmark handles POST /v1/benchmark/run
func (h *BenchmarkHandler) handleRunBenchmark(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "store not configured")
		return
	}

	config := benchmark.DefaultConfig()

	// Parse optional configuration from request body
	var req BenchmarkRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.NumEntities > 0 {
			config.NumEntities = req.NumEntities
		}
		if req.NumFeatures > 0 {
			config.NumFeatures = req.NumFeatures
		}
		if req.NumOperations > 0 {
			config.NumOperations = req.NumOperations
		}
		if req.Concurrency > 0 {
			config.Concurrency = req.Concurrency
		}
		if req.DataSize > 0 {
			config.DataSize = req.DataSize
		}
	}

	// Parse query parameters for quick configuration
	if entities := r.URL.Query().Get("entities"); entities != "" {
		if n, err := strconv.Atoi(entities); err == nil && n > 0 {
			config.NumEntities = n
		}
	}
	if ops := r.URL.Query().Get("operations"); ops != "" {
		if n, err := strconv.Atoi(ops); err == nil && n > 0 {
			config.NumOperations = n
		}
	}
	if concurrency := r.URL.Query().Get("concurrency"); concurrency != "" {
		if n, err := strconv.Atoi(concurrency); err == nil && n > 0 {
			config.Concurrency = n
		}
	}

	// Limit benchmark size for API requests
	if config.NumEntities > 100000 {
		config.NumEntities = 100000
	}
	if config.NumOperations > 1000000 {
		config.NumOperations = 1000000
	}
	if config.Concurrency > 100 {
		config.Concurrency = 100
	}

	// Reduce warmup for API requests
	config.WarmupDuration = 0

	suite := benchmark.NewSuite(h.store, config)

	if err := suite.Run(r.Context()); err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	results := suite.GetResults()

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"config":  config,
		"results": results,
	})
}

// handleGetDefaultConfig handles GET /v1/benchmark/config
func (h *BenchmarkHandler) handleGetDefaultConfig(w http.ResponseWriter, r *http.Request) {
	config := benchmark.DefaultConfig()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"num_entities":       config.NumEntities,
		"num_features":       config.NumFeatures,
		"num_operations":     config.NumOperations,
		"concurrency":        config.Concurrency,
		"warmup_duration_ms": config.WarmupDuration.Milliseconds(),
		"data_size":          config.DataSize,
	})
}

func (h *BenchmarkHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *BenchmarkHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
