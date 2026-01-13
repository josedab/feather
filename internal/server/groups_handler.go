package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/groups"
)

// GroupsHandler handles feature group API requests.
type GroupsHandler struct {
	manager *groups.Manager
}

// NewGroupsHandler creates a new groups handler.
func NewGroupsHandler() *GroupsHandler {
	return &GroupsHandler{
		manager: groups.NewManager(),
	}
}

// RegisterRoutes registers feature group API routes.
func (h *GroupsHandler) RegisterRoutes(mux *http.ServeMux) {
	// Group CRUD
	mux.HandleFunc("POST /v1/groups", h.handleCreateGroup)
	mux.HandleFunc("GET /v1/groups", h.handleListGroups)
	mux.HandleFunc("GET /v1/groups/{id}", h.handleGetGroup)
	mux.HandleFunc("PUT /v1/groups/{id}", h.handleUpdateGroup)
	mux.HandleFunc("DELETE /v1/groups/{id}", h.handleDeleteGroup)

	// Group status
	mux.HandleFunc("PUT /v1/groups/{id}/status", h.handleSetStatus)

	// Group features
	mux.HandleFunc("POST /v1/groups/{id}/features", h.handleAddFeature)
	mux.HandleFunc("DELETE /v1/groups/{id}/features/{feature}", h.handleRemoveFeature)
	mux.HandleFunc("GET /v1/groups/{id}/features", h.handleGetFeatures)

	// Group versions
	mux.HandleFunc("GET /v1/groups/{id}/versions/{version}", h.handleGetVersion)

	// Views - use /v1/feature-views to avoid route conflict with /v1/groups/{id}/features
	mux.HandleFunc("POST /v1/feature-views", h.handleCreateView)
	mux.HandleFunc("GET /v1/feature-views", h.handleListViews)
	mux.HandleFunc("GET /v1/feature-views/{id}", h.handleGetView)
	mux.HandleFunc("DELETE /v1/feature-views/{id}", h.handleDeleteView)

	// Query by entity/tag - use different paths to avoid route conflicts
	mux.HandleFunc("GET /v1/entities/{entity}/groups", h.handleGetByEntity)
	mux.HandleFunc("GET /v1/tags/{tag}/groups", h.handleGetByTag)

	// Stats
	mux.HandleFunc("GET /v1/group-stats", h.handleGetStats)
}

// GetManager returns the groups manager for integration.
func (h *GroupsHandler) GetManager() *groups.Manager {
	return h.manager
}

// CreateGroupRequest represents a request to create a feature group.
type CreateGroupRequest struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	EntityType  string                `json:"entity_type"`
	Features    []groups.GroupFeature `json:"features"`
	Tags        []string              `json:"tags"`
	Owner       string                `json:"owner"`
	Team        string                `json:"team"`
	Metadata    map[string]string     `json:"metadata"`
}

// handleCreateGroup handles POST /v1/groups
func (h *GroupsHandler) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	createdBy := r.Header.Get("X-User-ID")
	if createdBy == "" {
		createdBy = "anonymous"
	}

	group := &groups.FeatureGroup{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		EntityType:  req.EntityType,
		Features:    req.Features,
		Tags:        req.Tags,
		Owner:       req.Owner,
		Team:        req.Team,
		Metadata:    req.Metadata,
	}

	if err := h.manager.CreateGroup(group, createdBy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"group":   group,
	})
}

// handleListGroups handles GET /v1/groups
func (h *GroupsHandler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	var filter *groups.GroupFilter
	if query.Get("entity_type") != "" || query.Get("status") != "" ||
		query.Get("owner") != "" || query.Get("team") != "" || len(query["tag"]) > 0 {
		filter = &groups.GroupFilter{
			EntityType: query.Get("entity_type"),
			Status:     groups.GroupStatus(query.Get("status")),
			Owner:      query.Get("owner"),
			Team:       query.Get("team"),
			Tags:       query["tag"],
		}
	}

	groupList := h.manager.ListGroups(filter)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"groups": groupList,
		"count":  len(groupList),
	})
}

// handleGetGroup handles GET /v1/groups/{id}
func (h *GroupsHandler) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id required")
		return
	}

	group := h.manager.GetGroup(id)
	if group == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "group not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, group)
}

// handleUpdateGroup handles PUT /v1/groups/{id}
func (h *GroupsHandler) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id required")
		return
	}

	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	updatedBy := r.Header.Get("X-User-ID")
	if updatedBy == "" {
		updatedBy = "anonymous"
	}

	group := &groups.FeatureGroup{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		EntityType:  req.EntityType,
		Features:    req.Features,
		Tags:        req.Tags,
		Owner:       req.Owner,
		Team:        req.Team,
		Metadata:    req.Metadata,
	}

	if err := h.manager.UpdateGroup(group, updatedBy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"group":   h.manager.GetGroup(id),
	})
}

// handleDeleteGroup handles DELETE /v1/groups/{id}
func (h *GroupsHandler) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id required")
		return
	}

	if err := h.manager.DeleteGroup(id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// GroupSetStatusRequest represents a status update request for groups.
type GroupSetStatusRequest struct {
	Status groups.GroupStatus `json:"status"`
}

// handleSetStatus handles PUT /v1/groups/{id}/status
func (h *GroupsHandler) handleSetStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id required")
		return
	}

	var req GroupSetStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	updatedBy := r.Header.Get("X-User-ID")
	if updatedBy == "" {
		updatedBy = "anonymous"
	}

	if err := h.manager.SetStatus(id, req.Status, updatedBy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"status":  req.Status,
	})
}

// handleAddFeature handles POST /v1/groups/{id}/features
func (h *GroupsHandler) handleAddFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id required")
		return
	}

	var feature groups.GroupFeature
	if err := json.NewDecoder(r.Body).Decode(&feature); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	updatedBy := r.Header.Get("X-User-ID")
	if updatedBy == "" {
		updatedBy = "anonymous"
	}

	if err := h.manager.AddFeature(id, feature, updatedBy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"group":   h.manager.GetGroup(id),
	})
}

// handleRemoveFeature handles DELETE /v1/groups/{id}/features/{feature}
func (h *GroupsHandler) handleRemoveFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	featureName := r.PathValue("feature")
	if id == "" || featureName == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id and feature name required")
		return
	}

	updatedBy := r.Header.Get("X-User-ID")
	if updatedBy == "" {
		updatedBy = "anonymous"
	}

	if err := h.manager.RemoveFeature(id, featureName, updatedBy); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
		"feature": featureName,
	})
}

// handleGetFeatures handles GET /v1/groups/{id}/features
func (h *GroupsHandler) handleGetFeatures(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id required")
		return
	}

	features := h.manager.GetFeatureNames(id)
	if features == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "group not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"group_id": id,
		"features": features,
		"count":    len(features),
	})
}

// handleGetVersion handles GET /v1/groups/{id}/versions/{version}
func (h *GroupsHandler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versionStr := r.PathValue("version")
	if id == "" || versionStr == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "group id and version required")
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid version number")
		return
	}

	group := h.manager.GetGroupVersion(id, version)
	if group == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "version not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, group)
}

// CreateViewRequest represents a view creation request.
type CreateViewRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	GroupID     string   `json:"group_id"`
	Features    []string `json:"features"`
	Description string   `json:"description"`
}

// handleCreateView handles POST /v1/groups/views
func (h *GroupsHandler) handleCreateView(w http.ResponseWriter, r *http.Request) {
	var req CreateViewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	view := &groups.GroupView{
		ID:          req.ID,
		Name:        req.Name,
		GroupID:     req.GroupID,
		Features:    req.Features,
		Description: req.Description,
	}

	if err := h.manager.CreateView(view); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"view":    view,
	})
}

// handleListViews handles GET /v1/groups/views
func (h *GroupsHandler) handleListViews(w http.ResponseWriter, r *http.Request) {
	groupID := r.URL.Query().Get("group_id")
	views := h.manager.ListViews(groupID)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"views": views,
		"count": len(views),
	})
}

// handleGetView handles GET /v1/groups/views/{id}
func (h *GroupsHandler) handleGetView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "view id required")
		return
	}

	view := h.manager.GetView(id)
	if view == nil {
		h.writeError(r.Context(), w, http.StatusNotFound, "view not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, view)
}

// handleDeleteView handles DELETE /v1/groups/views/{id}
func (h *GroupsHandler) handleDeleteView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "view id required")
		return
	}

	if err := h.manager.DeleteView(id); err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
		"id":      id,
	})
}

// handleGetByEntity handles GET /v1/groups/by-entity/{entity}
func (h *GroupsHandler) handleGetByEntity(w http.ResponseWriter, r *http.Request) {
	entity := r.PathValue("entity")
	if entity == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "entity type required")
		return
	}

	groupList := h.manager.GetGroupsByEntity(entity)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"entity_type": entity,
		"groups":      groupList,
		"count":       len(groupList),
	})
}

// handleGetByTag handles GET /v1/groups/by-tag/{tag}
func (h *GroupsHandler) handleGetByTag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	if tag == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "tag required")
		return
	}

	groupList := h.manager.GetGroupsByTag(tag)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"tag":    tag,
		"groups": groupList,
		"count":  len(groupList),
	})
}

// handleGetStats handles GET /v1/groups/stats
func (h *GroupsHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

func (h *GroupsHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *GroupsHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	writeJSONError(ctx, w, status, message)
}
