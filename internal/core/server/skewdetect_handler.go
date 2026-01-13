package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/feather-store/feather/internal/extensions/skewdetect"
)

// SkewDetectHandler provides HTTP endpoints for training-serving skew detection.
type SkewDetectHandler struct {
	detector *skewdetect.Detector
}

// NewSkewDetectHandler creates a new skew detect handler.
func NewSkewDetectHandler(detector *skewdetect.Detector) *SkewDetectHandler {
	return &SkewDetectHandler{detector: detector}
}

// RegisterRoutes registers skew detection API routes.
func (h *SkewDetectHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/skew/features", h.handleRegister)
	mux.HandleFunc("POST /v1/skew/features/{name}/online", h.handleRecordOnline)
	mux.HandleFunc("POST /v1/skew/features/{name}/offline", h.handleRecordOffline)
	mux.HandleFunc("GET /v1/skew/features/{name}/check", h.handleCheck)
	mux.HandleFunc("GET /v1/skew/features/{name}", h.handleGetProfile)
	mux.HandleFunc("GET /v1/skew/check", h.handleCheckAll)
	mux.HandleFunc("GET /v1/skew/alerts", h.handleGetAlerts)
	mux.HandleFunc("POST /v1/skew/contracts", h.handleSetContract)
	mux.HandleFunc("GET /v1/skew/contracts/{name}/validate", h.handleValidateContract)
	mux.HandleFunc("GET /v1/skew/profiles", h.handleListProfiles)
	mux.HandleFunc("GET /v1/skew/stats", h.handleStats)
}

func (h *SkewDetectHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "name is required")
		return
	}

	if err := h.detector.RegisterFeature(req.Name); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, map[string]string{"registered": req.Name})
}

func (h *SkewDetectHandler) handleRecordOnline(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Values []float64 `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.detector.RecordOnline(name, req.Values); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *SkewDetectHandler) handleRecordOffline(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Values []float64 `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.detector.RecordOffline(name, req.Values); err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "recorded"})
}

func (h *SkewDetectHandler) handleCheck(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	profile, err := h.detector.Check(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, profile)
}

func (h *SkewDetectHandler) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	profile, err := h.detector.GetProfile(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, profile)
}

func (h *SkewDetectHandler) handleCheckAll(w http.ResponseWriter, r *http.Request) {
	profiles := h.detector.CheckAll()
	writeJSONResponse(r.Context(), w, http.StatusOK, profiles)
}

func (h *SkewDetectHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	if s := r.URL.Query().Get("since"); s != "" {
		parsed, err := time.Parse(time.RFC3339, s)
		if err != nil {
			writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid since parameter: "+err.Error())
			return
		}
		since = parsed
	}

	alerts := h.detector.GetAlerts(since)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (h *SkewDetectHandler) handleSetContract(w http.ResponseWriter, r *http.Request) {
	var contract skewdetect.DataContract
	if err := json.NewDecoder(r.Body).Decode(&contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.detector.SetContract(contract); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, contract)
}

func (h *SkewDetectHandler) handleValidateContract(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	violations, err := h.detector.ValidateContract(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"violations": violations,
		"total":      len(violations),
	})
}

func (h *SkewDetectHandler) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := h.detector.ListProfiles()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"profiles": profiles,
		"total":    len(profiles),
	})
}

func (h *SkewDetectHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.detector.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
