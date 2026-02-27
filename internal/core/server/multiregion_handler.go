package server

import (
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/platform/multiregion"
)

// MultiRegionHandler provides HTTP endpoints for multi-region federation.
type MultiRegionHandler struct {
	federation *multiregion.Federation
}

// NewMultiRegionHandler creates a new multi-region handler.
func NewMultiRegionHandler(federation *multiregion.Federation) *MultiRegionHandler {
	return &MultiRegionHandler{federation: federation}
}

// RegisterRoutes registers federation API routes.
func (h *MultiRegionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/federation/regions", h.handleAddRegion)
	mux.HandleFunc("GET /v1/federation/regions", h.handleListRegions)
	mux.HandleFunc("GET /v1/federation/regions/{name}", h.handleGetRegion)
	mux.HandleFunc("DELETE /v1/federation/regions/{name}", h.handleRemoveRegion)
	mux.HandleFunc("GET /v1/federation/route", h.handleRoute)
	mux.HandleFunc("POST /v1/federation/residency", h.handleSetResidency)
	mux.HandleFunc("GET /v1/federation/residency", h.handleGetResidencyRules)
	mux.HandleFunc("POST /v1/federation/replicate", h.handleReplicate)
	mux.HandleFunc("GET /v1/federation/replication-log", h.handleReplicationLog)
	mux.HandleFunc("GET /v1/federation/clock/{entity}", h.handleGetClock)
	mux.HandleFunc("GET /v1/federation/stats", h.handleStats)
}

func (h *MultiRegionHandler) handleAddRegion(w http.ResponseWriter, r *http.Request) {
	var region multiregion.Region
	if err := strictDecode(r.Body, &region); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.federation.AddRegion(region); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	added, _ := h.federation.GetRegion(region.Name)
	writeJSONResponse(r.Context(), w, http.StatusCreated, added)
}

func (h *MultiRegionHandler) handleListRegions(w http.ResponseWriter, r *http.Request) {
	regions := h.federation.ListRegions()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"regions": regions,
		"total":   len(regions),
	})
}

func (h *MultiRegionHandler) handleGetRegion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	region, err := h.federation.GetRegion(name)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, region)
}

func (h *MultiRegionHandler) handleRemoveRegion(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.federation.RemoveRegion(name); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"deleted": name})
}

func (h *MultiRegionHandler) handleRoute(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("entity")
	if entity == "" {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "entity query parameter is required")
		return
	}

	result, err := h.federation.Route(entity)
	if err != nil {
		writeJSONError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, result)
}

func (h *MultiRegionHandler) handleSetResidency(w http.ResponseWriter, r *http.Request) {
	var rule multiregion.ResidencyRule
	if err := strictDecode(r.Body, &rule); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.federation.SetResidencyRule(rule); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusCreated, rule)
}

func (h *MultiRegionHandler) handleGetResidencyRules(w http.ResponseWriter, r *http.Request) {
	rules := h.federation.GetResidencyRules()
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"rules": rules,
		"total": len(rules),
	})
}

func (h *MultiRegionHandler) handleReplicate(w http.ResponseWriter, r *http.Request) {
	var event multiregion.ReplicationEvent
	if err := strictDecode(r.Body, &event); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := h.federation.ReplicateEvent(event); err != nil {
		writeJSONError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]string{"status": "replicated"})
}

func (h *MultiRegionHandler) handleReplicationLog(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events := h.federation.GetReplicationLog(limit)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"events": events,
		"total":  len(events),
	})
}

func (h *MultiRegionHandler) handleGetClock(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	clock := h.federation.GetVectorClock(entity)
	writeJSONResponse(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entity": entity,
		"clock":  clock,
	})
}

func (h *MultiRegionHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.federation.Stats()
	writeJSONResponse(r.Context(), w, http.StatusOK, stats)
}
