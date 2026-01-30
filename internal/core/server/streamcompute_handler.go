package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/feather-store/feather/internal/extensions/streamcompute"
)

// StreamComputeHandler handles stream computation API requests.
type StreamComputeHandler struct {
	engine *streamcompute.Engine
}

// NewStreamComputeHandler creates a new stream compute handler.
func NewStreamComputeHandler(engine *streamcompute.Engine) *StreamComputeHandler {
	return &StreamComputeHandler{engine: engine}
}

// RegisterRoutes registers stream compute API routes.
func (h *StreamComputeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/stream/pipelines", h.handleListPipelines)
	mux.HandleFunc("POST /v1/stream/pipelines", h.handleCreatePipeline)
	mux.HandleFunc("GET /v1/stream/pipelines/{id}", h.handleGetPipeline)
	mux.HandleFunc("DELETE /v1/stream/pipelines/{id}", h.handleDeletePipeline)
	mux.HandleFunc("POST /v1/stream/pipelines/{id}/start", h.handleStartPipeline)
	mux.HandleFunc("POST /v1/stream/pipelines/{id}/stop", h.handleStopPipeline)
	mux.HandleFunc("POST /v1/stream/pipelines/{id}/checkpoint", h.handleCreateCheckpoint)
	mux.HandleFunc("GET /v1/stream/pipelines/{id}/checkpoints", h.handleListCheckpoints)
	mux.HandleFunc("POST /v1/stream/pipelines/{id}/recover", h.handleRecover)
	mux.HandleFunc("POST /v1/stream/ingest", h.handleIngest)
	mux.HandleFunc("GET /v1/stream/results", h.handleGetResults)
	mux.HandleFunc("GET /v1/stream/stats", h.handleGetStats)
}

type createPipelineRequest struct {
	ID            string `json:"id"`
	Description   string `json:"description,omitempty"`
	WindowType    string `json:"window_type"`
	WindowSize    string `json:"window_size"`
	SlideInterval string `json:"slide_interval,omitempty"`
	SessionGap    string `json:"session_gap,omitempty"`
	MaxLate       string `json:"max_late,omitempty"`
	Aggregation   string `json:"aggregation"`
	GroupByKey    bool   `json:"group_by_key"`
	OutputEntity  string `json:"output_entity,omitempty"`
	OutputFeature string `json:"output_feature,omitempty"`
}

type ingestRequest struct {
	Events []ingestEvent `json:"events"`
}

type ingestEvent struct {
	Key       string                 `json:"key"`
	Value     float64                `json:"value"`
	Timestamp string                 `json:"timestamp,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

func (h *StreamComputeHandler) handleListPipelines(w http.ResponseWriter, r *http.Request) {
	pipelines := h.engine.ListPipelines()
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"pipelines": pipelines,
	})
}

func (h *StreamComputeHandler) handleCreatePipeline(w http.ResponseWriter, r *http.Request) {
	var req createPipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "pipeline id required")
		return
	}

	windowType := streamcompute.WindowTumbling
	switch req.WindowType {
	case "sliding":
		windowType = streamcompute.WindowSliding
	case "session":
		windowType = streamcompute.WindowSession
	case "tumbling", "":
		windowType = streamcompute.WindowTumbling
	default:
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid window_type: use tumbling, sliding, or session")
		return
	}

	windowSize, _ := time.ParseDuration(req.WindowSize)
	slide, _ := time.ParseDuration(req.SlideInterval)
	gap, _ := time.ParseDuration(req.SessionGap)
	maxLate, _ := time.ParseDuration(req.MaxLate)

	agg := parseAggregation(req.Aggregation)

	cfg := streamcompute.PipelineConfig{
		ID:          req.ID,
		Description: req.Description,
		Window: streamcompute.WindowConfig{
			Type:    windowType,
			Size:    windowSize,
			Slide:   slide,
			Gap:     gap,
			MaxLate: maxLate,
		},
		Aggregation:   agg,
		GroupByKey:    req.GroupByKey,
		OutputEntity:  req.OutputEntity,
		OutputFeature: req.OutputFeature,
	}

	if err := h.engine.CreatePipeline(cfg); err != nil {
		if errors.Is(err, streamcompute.ErrPipelineExists) {
			h.writeError(r.Context(), w, http.StatusConflict, "pipeline already exists")
			return
		}
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"id":      req.ID,
	})
}

func (h *StreamComputeHandler) handleGetPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	info, err := h.engine.GetPipeline(id)
	if err != nil {
		if errors.Is(err, streamcompute.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, info)
}

func (h *StreamComputeHandler) handleDeletePipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.DeletePipeline(id); err != nil {
		if errors.Is(err, streamcompute.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline deleted"})
}

func (h *StreamComputeHandler) handleStartPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.StartPipeline(id); err != nil {
		if errors.Is(err, streamcompute.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		if errors.Is(err, streamcompute.ErrPipelineRunning) {
			h.writeError(r.Context(), w, http.StatusConflict, "pipeline already running")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline started"})
}

func (h *StreamComputeHandler) handleStopPipeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.StopPipeline(id); err != nil {
		if errors.Is(err, streamcompute.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		if errors.Is(err, streamcompute.ErrPipelineStopped) {
			h.writeError(r.Context(), w, http.StatusConflict, "pipeline not running")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline stopped"})
}

func (h *StreamComputeHandler) handleIngest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	var allResults []streamcompute.WindowResult
	for _, e := range req.Events {
		ts := time.Now()
		if e.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
				ts = parsed
			}
		}

		results := h.engine.Ingest(streamcompute.Event{
			Key:       e.Key,
			Value:     e.Value,
			Timestamp: ts,
			Fields:    e.Fields,
		})
		allResults = append(allResults, results...)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events_processed": len(req.Events),
		"windows_fired":    len(allResults),
		"results":          allResults,
	})
}

func (h *StreamComputeHandler) handleGetResults(w http.ResponseWriter, r *http.Request) {
	pipelineID := r.URL.Query().Get("pipeline_id")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results := h.engine.GetResults(pipelineID, limit)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"results": results,
		"count":   len(results),
	})
}

func (h *StreamComputeHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.engine.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func parseAggregation(s string) streamcompute.AggregationType {
	switch s {
	case "count":
		return streamcompute.AggCount
	case "sum":
		return streamcompute.AggSum
	case "avg":
		return streamcompute.AggAvg
	case "min":
		return streamcompute.AggMin
	case "max":
		return streamcompute.AggMax
	case "first":
		return streamcompute.AggFirst
	case "last":
		return streamcompute.AggLast
	default:
		return streamcompute.AggSum
	}
}

func (h *StreamComputeHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *StreamComputeHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}

func (h *StreamComputeHandler) handleCreateCheckpoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cp, err := h.engine.CreateCheckpoint(id)
	if err != nil {
		if errors.Is(err, streamcompute.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"checkpoint": cp,
	})
}

func (h *StreamComputeHandler) handleListCheckpoints(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cps := h.engine.GetCheckpoints(id)
	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"checkpoints": cps,
		"count":       len(cps),
	})
}

func (h *StreamComputeHandler) handleRecover(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.engine.RestoreFromCheckpoint(id); err != nil {
		if errors.Is(err, streamcompute.ErrPipelineNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "pipeline not found")
			return
		}
		if errors.Is(err, streamcompute.ErrCheckpointNotFound) {
			h.writeError(r.Context(), w, http.StatusNotFound, "no checkpoint available")
			return
		}
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeJSON(r.Context(), w, http.StatusOK, SuccessResponse{Success: true, Message: "pipeline recovered from checkpoint"})
}
