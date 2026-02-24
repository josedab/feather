package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/backfillengine"
)

// ---------------------------------------------------------------------------
// BackfillEngineHandler
// ---------------------------------------------------------------------------

// BackfillEngineHandler exposes unified streaming backfill engine endpoints.
type BackfillEngineHandler struct {
	coordinator *backfillengine.Coordinator
}

// NewBackfillEngineHandler creates a new BackfillEngineHandler.
func NewBackfillEngineHandler(coord *backfillengine.Coordinator) *BackfillEngineHandler {
	return &BackfillEngineHandler{coordinator: coord}
}

// RegisterRoutes registers backfill engine API routes.
func (h *BackfillEngineHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/backfill-engine/sources", h.handleListSources)
	mux.HandleFunc("POST /v1/backfill-engine/sources", h.handleRegisterSource)
	mux.HandleFunc("DELETE /v1/backfill-engine/sources/{name}", h.handleUnregisterSource)
	mux.HandleFunc("POST /v1/backfill-engine/jobs", h.handleCreateJob)
	mux.HandleFunc("GET /v1/backfill-engine/jobs", h.handleListJobs)
	mux.HandleFunc("GET /v1/backfill-engine/jobs/{id}", h.handleGetJob)
	mux.HandleFunc("POST /v1/backfill-engine/jobs/{id}/start", h.handleStartJob)
	mux.HandleFunc("POST /v1/backfill-engine/jobs/{id}/pause", h.handlePauseJob)
	mux.HandleFunc("POST /v1/backfill-engine/jobs/{id}/cancel", h.handleCancelJob)
	mux.HandleFunc("GET /v1/backfill-engine/jobs/{id}/checkpoint", h.handleGetCheckpoint)
	mux.HandleFunc("GET /v1/backfill-engine/stats", h.handleStats)
}

func (h *BackfillEngineHandler) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources := h.coordinator.ListSources()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"sources": sources,
		"total":   len(sources),
	})
}

func (h *BackfillEngineHandler) handleRegisterSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string                        `json:"name"`
		Type backfillengine.SourceType     `json:"type"`
		Config json.RawMessage             `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	var source backfillengine.Source
	switch req.Type {
	case backfillengine.SourceTypeKafka:
		var cfg backfillengine.KafkaSourceConfig
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid kafka config: "+err.Error())
			return
		}
		source = backfillengine.NewKafkaSource(cfg)
	case backfillengine.SourceTypeFlink:
		var cfg backfillengine.FlinkSourceConfig
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid flink config: "+err.Error())
			return
		}
		source = backfillengine.NewFlinkSource(cfg)
	case backfillengine.SourceTypeFile:
		var cfg backfillengine.FileSourceConfig
		if err := json.Unmarshal(req.Config, &cfg); err != nil {
			writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid file config: "+err.Error())
			return
		}
		source = backfillengine.NewFileSource(cfg)
	default:
		writeJSONError(r.Context(), w, http.StatusBadRequest, "unsupported source type: "+string(req.Type))
		return
	}

	if err := h.coordinator.RegisterSource(req.Name, source); err != nil {
		writeJSONError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"name": req.Name,
		"type": req.Type,
	})
}

func (h *BackfillEngineHandler) handleUnregisterSource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.coordinator.UnregisterSource(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *BackfillEngineHandler) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req backfillengine.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	job, err := h.coordinator.CreateJob(req)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, job)
}

func (h *BackfillEngineHandler) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.coordinator.ListJobs()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

func (h *BackfillEngineHandler) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job, err := h.coordinator.GetJob(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, job)
}

func (h *BackfillEngineHandler) handleStartJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writer := &noopFeatureWriter{}
	if err := h.coordinator.StartJob(id, writer); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "started", "job_id": id})
}

func (h *BackfillEngineHandler) handlePauseJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.coordinator.PauseJob(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "paused", "job_id": id})
}

func (h *BackfillEngineHandler) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.coordinator.CancelJob(id); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "cancelled", "job_id": id})
}

func (h *BackfillEngineHandler) handleGetCheckpoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cp, err := h.coordinator.GetCheckpoint(id)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, cp)
}

func (h *BackfillEngineHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(r.Context(), w, http.StatusOK, h.coordinator.Stats())
}

// noopFeatureWriter is a default writer used when no store is configured.
type noopFeatureWriter struct{}

func (w *noopFeatureWriter) WriteFeature(_ context.Context, _ string, _ string, _ interface{}, _ time.Time) error {
	return nil
}
func (w *noopFeatureWriter) Flush(_ context.Context) error { return nil }
