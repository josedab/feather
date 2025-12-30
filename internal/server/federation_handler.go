package server

import (
	"encoding/json"
	"net/http"

	"github.com/feather-store/feather/internal/federation"
)

// FederationHandler handles federation API requests.
type FederationHandler struct {
	federation *federation.Federation
}

// NewFederationHandler creates a new federation handler.
func NewFederationHandler(fed *federation.Federation) *FederationHandler {
	return &FederationHandler{
		federation: fed,
	}
}

// RegisterRoutes registers federation API routes.
func (h *FederationHandler) RegisterRoutes(mux *http.ServeMux) {
	// Node management
	mux.HandleFunc("GET /v1/federation/nodes", h.handleListNodes)
	mux.HandleFunc("GET /v1/federation/nodes/{id}", h.handleGetNode)
	mux.HandleFunc("POST /v1/federation/nodes", h.handleJoinNode)
	mux.HandleFunc("DELETE /v1/federation/nodes/{id}", h.handleLeaveNode)

	// Feature catalog
	mux.HandleFunc("GET /v1/federation/catalog", h.handleListCatalog)
	mux.HandleFunc("GET /v1/federation/features/{id}", h.handleGetFeature)
	mux.HandleFunc("POST /v1/federation/features", h.handleShareFeature)
	mux.HandleFunc("PUT /v1/federation/features/{id}", h.handleUpdateFeature)
	mux.HandleFunc("DELETE /v1/federation/features/{id}", h.handleDeleteFeature)

	// Search
	mux.HandleFunc("POST /v1/federation/search", h.handleSearchFeatures)

	// Replication
	mux.HandleFunc("POST /v1/federation/features/{id}/replicate", h.handleReplicateFeature)
	mux.HandleFunc("PUT /v1/federation/features/{id}/policy", h.handleSetReplicationPolicy)

	// Stats
	mux.HandleFunc("GET /v1/federation/stats", h.handleGetStats)
	mux.HandleFunc("GET /v1/federation/local", h.handleGetLocalNode)
}

// NodeJSON represents a node in JSON format.
type NodeJSON struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Address     string               `json:"address"`
	Role        string               `json:"role"`
	State       string               `json:"state"`
	Region      string               `json:"region"`
	Tags        []string             `json:"tags,omitempty"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
	JoinedAt    string               `json:"joined_at"`
	LastSeen    string               `json:"last_seen"`
	Version     string               `json:"version,omitempty"`
	Permissions *NodePermissionsJSON `json:"permissions,omitempty"`
}

// NodePermissionsJSON represents node permissions in JSON.
type NodePermissionsJSON struct {
	CanRead      bool     `json:"can_read"`
	CanWrite     bool     `json:"can_write"`
	CanReplicate bool     `json:"can_replicate"`
	AllowedTeams []string `json:"allowed_teams,omitempty"`
	DeniedTeams  []string `json:"denied_teams,omitempty"`
}

// FederatedFeatureJSON represents a federated feature in JSON.
type FederatedFeatureJSON struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	OwnerNode     string             `json:"owner_node"`
	OwnerTeam     string             `json:"owner_team,omitempty"`
	DataType      string             `json:"data_type,omitempty"`
	Tags          []string           `json:"tags,omitempty"`
	Visibility    string             `json:"visibility"`
	AccessControl *AccessControlJSON `json:"access_control,omitempty"`
	Metadata      map[string]string  `json:"metadata,omitempty"`
	Version       int64              `json:"version"`
	CreatedAt     string             `json:"created_at"`
	UpdatedAt     string             `json:"updated_at"`
	ReplicatedTo  []string           `json:"replicated_to,omitempty"`
}

// AccessControlJSON represents access control in JSON.
type AccessControlJSON struct {
	AllowedNodes []string       `json:"allowed_nodes,omitempty"`
	AllowedTeams []string       `json:"allowed_teams,omitempty"`
	AllowedUsers []string       `json:"allowed_users,omitempty"`
	DeniedNodes  []string       `json:"denied_nodes,omitempty"`
	DeniedTeams  []string       `json:"denied_teams,omitempty"`
	DeniedUsers  []string       `json:"denied_users,omitempty"`
	RequireAuth  bool           `json:"require_auth"`
	RateLimits   map[string]int `json:"rate_limits,omitempty"`
}

// CatalogEntryJSON represents a catalog entry in JSON.
type CatalogEntryJSON struct {
	Feature      *FederatedFeatureJSON `json:"feature"`
	SourceNode   string                `json:"source_node"`
	LocalCopy    bool                  `json:"local_copy"`
	CacheExpiry  string                `json:"cache_expiry,omitempty"`
	AccessCount  int64                 `json:"access_count"`
	LastAccessed string                `json:"last_accessed,omitempty"`
}

// handleListNodes handles GET /v1/federation/nodes
func (h *FederationHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	nodes := h.federation.ListNodes()
	response := make([]NodeJSON, len(nodes))

	for i, node := range nodes {
		response[i] = h.nodeToJSON(node)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes": response,
		"count": len(response),
	})
}

// handleGetNode handles GET /v1/federation/nodes/{id}
func (h *FederationHandler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	nodeID := r.PathValue("id")
	if nodeID == "" {
		h.writeError(w, http.StatusBadRequest, "node ID required")
		return
	}

	node, err := h.federation.GetNode(nodeID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.nodeToJSON(node))
}

// JoinNodeRequest represents a request to join a node.
type JoinNodeRequest struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Address     string               `json:"address"`
	Role        string               `json:"role"`
	Region      string               `json:"region"`
	Tags        []string             `json:"tags,omitempty"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
	Permissions *NodePermissionsJSON `json:"permissions,omitempty"`
}

// handleJoinNode handles POST /v1/federation/nodes
func (h *FederationHandler) handleJoinNode(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	var req JoinNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	if req.Address == "" {
		h.writeError(w, http.StatusBadRequest, "address is required")
		return
	}

	node := &federation.Node{
		ID:       req.ID,
		Name:     req.Name,
		Address:  req.Address,
		Role:     federation.NodeRole(req.Role),
		Region:   req.Region,
		Tags:     req.Tags,
		Metadata: req.Metadata,
	}

	if req.Permissions != nil {
		node.Permissions = &federation.NodePermissions{
			CanRead:      req.Permissions.CanRead,
			CanWrite:     req.Permissions.CanWrite,
			CanReplicate: req.Permissions.CanReplicate,
			AllowedTeams: req.Permissions.AllowedTeams,
			DeniedTeams:  req.Permissions.DeniedTeams,
		}
	} else {
		node.Permissions = &federation.NodePermissions{
			CanRead:      true,
			CanWrite:     true,
			CanReplicate: true,
		}
	}

	if err := h.federation.JoinNode(node); err != nil {
		h.writeError(w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"node_id": req.ID,
	})
}

// handleLeaveNode handles DELETE /v1/federation/nodes/{id}
func (h *FederationHandler) handleLeaveNode(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	nodeID := r.PathValue("id")
	if nodeID == "" {
		h.writeError(w, http.StatusBadRequest, "node ID required")
		return
	}

	if err := h.federation.LeaveNode(nodeID); err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleListCatalog handles GET /v1/federation/catalog
func (h *FederationHandler) handleListCatalog(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	entries := h.federation.ListCatalog()
	response := make([]CatalogEntryJSON, len(entries))

	for i, entry := range entries {
		response[i] = h.catalogEntryToJSON(entry)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"catalog": response,
		"count":   len(response),
	})
}

// handleGetFeature handles GET /v1/federation/features/{id}
func (h *FederationHandler) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	feature, err := h.federation.GetFeature(featureID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.featureToJSON(feature))
}

// ShareFeatureRequest represents a request to share a feature.
type ShareFeatureRequest struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Description   string             `json:"description,omitempty"`
	OwnerTeam     string             `json:"owner_team,omitempty"`
	DataType      string             `json:"data_type,omitempty"`
	Tags          []string           `json:"tags,omitempty"`
	Visibility    string             `json:"visibility"`
	AccessControl *AccessControlJSON `json:"access_control,omitempty"`
	Metadata      map[string]string  `json:"metadata,omitempty"`
}

// handleShareFeature handles POST /v1/federation/features
func (h *FederationHandler) handleShareFeature(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	var req ShareFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" {
		h.writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	visibility := federation.VisibilityFederation
	if req.Visibility != "" {
		visibility = federation.Visibility(req.Visibility)
	}

	feature := &federation.FederatedFeature{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		OwnerTeam:   req.OwnerTeam,
		DataType:    req.DataType,
		Tags:        req.Tags,
		Visibility:  visibility,
		Metadata:    req.Metadata,
	}

	if req.AccessControl != nil {
		feature.AccessControl = &federation.AccessControl{
			AllowedNodes: req.AccessControl.AllowedNodes,
			AllowedTeams: req.AccessControl.AllowedTeams,
			AllowedUsers: req.AccessControl.AllowedUsers,
			DeniedNodes:  req.AccessControl.DeniedNodes,
			DeniedTeams:  req.AccessControl.DeniedTeams,
			DeniedUsers:  req.AccessControl.DeniedUsers,
			RequireAuth:  req.AccessControl.RequireAuth,
			RateLimits:   req.AccessControl.RateLimits,
		}
	}

	if err := h.federation.ShareFeature(feature); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success":    true,
		"feature_id": req.ID,
	})
}

// handleUpdateFeature handles PUT /v1/federation/features/{id}
func (h *FederationHandler) handleUpdateFeature(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	var req ShareFeatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	feature := &federation.FederatedFeature{
		ID:          featureID,
		Name:        req.Name,
		Description: req.Description,
		OwnerTeam:   req.OwnerTeam,
		DataType:    req.DataType,
		Tags:        req.Tags,
		Visibility:  federation.Visibility(req.Visibility),
		Metadata:    req.Metadata,
	}

	if req.AccessControl != nil {
		feature.AccessControl = &federation.AccessControl{
			AllowedNodes: req.AccessControl.AllowedNodes,
			AllowedTeams: req.AccessControl.AllowedTeams,
			AllowedUsers: req.AccessControl.AllowedUsers,
			DeniedNodes:  req.AccessControl.DeniedNodes,
			DeniedTeams:  req.AccessControl.DeniedTeams,
			DeniedUsers:  req.AccessControl.DeniedUsers,
			RequireAuth:  req.AccessControl.RequireAuth,
			RateLimits:   req.AccessControl.RateLimits,
		}
	}

	if err := h.federation.UpdateFeature(feature); err != nil {
		if err == federation.ErrFeatureNotFound {
			h.writeError(w, http.StatusNotFound, err.Error())
		} else if err == federation.ErrNotFeatureOwner {
			h.writeError(w, http.StatusForbidden, err.Error())
		} else {
			h.writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleDeleteFeature handles DELETE /v1/federation/features/{id}
func (h *FederationHandler) handleDeleteFeature(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	if err := h.federation.DeleteFeature(featureID); err != nil {
		if err == federation.ErrFeatureNotFound {
			h.writeError(w, http.StatusNotFound, err.Error())
		} else if err == federation.ErrNotFeatureOwner {
			h.writeError(w, http.StatusForbidden, err.Error())
		} else {
			h.writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// SearchFeaturesRequest represents a search request.
type SearchFeaturesRequest struct {
	Name       string   `json:"name,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Teams      []string `json:"teams,omitempty"`
	Regions    []string `json:"regions,omitempty"`
	Visibility []string `json:"visibility,omitempty"`
	DataTypes  []string `json:"data_types,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// handleSearchFeatures handles POST /v1/federation/search
func (h *FederationHandler) handleSearchFeatures(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	var req SearchFeaturesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	visibility := make([]federation.Visibility, len(req.Visibility))
	for i, v := range req.Visibility {
		visibility[i] = federation.Visibility(v)
	}

	query := federation.SearchQuery{
		Name:       req.Name,
		Tags:       req.Tags,
		Teams:      req.Teams,
		Regions:    req.Regions,
		Visibility: visibility,
		DataTypes:  req.DataTypes,
		Limit:      req.Limit,
	}

	results, err := h.federation.SearchFeatures(query)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]CatalogEntryJSON, len(results))
	for i, entry := range results {
		response[i] = h.catalogEntryToJSON(entry)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": response,
		"count":   len(response),
	})
}

// ReplicateRequest represents a replication request.
type ReplicateRequest struct {
	TargetNodes []string `json:"target_nodes"`
}

// handleReplicateFeature handles POST /v1/federation/features/{id}/replicate
func (h *FederationHandler) handleReplicateFeature(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	var req ReplicateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.TargetNodes) == 0 {
		h.writeError(w, http.StatusBadRequest, "target_nodes required")
		return
	}

	if err := h.federation.ReplicateFeature(r.Context(), featureID, req.TargetNodes); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"replicated_to": req.TargetNodes,
	})
}

// ReplicationPolicyRequest represents a replication policy request.
type ReplicationPolicyRequest struct {
	Mode               string   `json:"mode"`
	TargetNodes        []string `json:"target_nodes,omitempty"`
	TargetRegions      []string `json:"target_regions,omitempty"`
	MinReplicas        int      `json:"min_replicas"`
	MaxReplicas        int      `json:"max_replicas"`
	SyncIntervalSecs   int      `json:"sync_interval_secs"`
	ConflictResolution string   `json:"conflict_resolution"`
}

// handleSetReplicationPolicy handles PUT /v1/federation/features/{id}/policy
func (h *FederationHandler) handleSetReplicationPolicy(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	featureID := r.PathValue("id")
	if featureID == "" {
		h.writeError(w, http.StatusBadRequest, "feature ID required")
		return
	}

	var req ReplicationPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	policy := &federation.ReplicationPolicy{
		Mode:               federation.ReplicationMode(req.Mode),
		TargetNodes:        req.TargetNodes,
		TargetRegions:      req.TargetRegions,
		MinReplicas:        req.MinReplicas,
		MaxReplicas:        req.MaxReplicas,
		ConflictResolution: req.ConflictResolution,
	}

	if err := h.federation.SetReplicationPolicy(featureID, policy); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleGetStats handles GET /v1/federation/stats
func (h *FederationHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	stats := h.federation.GetStats()
	h.writeJSON(w, http.StatusOK, stats)
}

// handleGetLocalNode handles GET /v1/federation/local
func (h *FederationHandler) handleGetLocalNode(w http.ResponseWriter, r *http.Request) {
	if h.federation == nil {
		h.writeError(w, http.StatusServiceUnavailable, "federation not configured")
		return
	}

	node := h.federation.GetLocalNode()
	h.writeJSON(w, http.StatusOK, h.nodeToJSON(node))
}

func (h *FederationHandler) nodeToJSON(node *federation.Node) NodeJSON {
	result := NodeJSON{
		ID:       node.ID,
		Name:     node.Name,
		Address:  node.Address,
		Role:     string(node.Role),
		State:    string(node.State),
		Region:   node.Region,
		Tags:     node.Tags,
		Metadata: node.Metadata,
		JoinedAt: node.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastSeen: node.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
		Version:  node.Version,
	}

	if node.Permissions != nil {
		result.Permissions = &NodePermissionsJSON{
			CanRead:      node.Permissions.CanRead,
			CanWrite:     node.Permissions.CanWrite,
			CanReplicate: node.Permissions.CanReplicate,
			AllowedTeams: node.Permissions.AllowedTeams,
			DeniedTeams:  node.Permissions.DeniedTeams,
		}
	}

	return result
}

func (h *FederationHandler) featureToJSON(feature *federation.FederatedFeature) FederatedFeatureJSON {
	result := FederatedFeatureJSON{
		ID:           feature.ID,
		Name:         feature.Name,
		Description:  feature.Description,
		OwnerNode:    feature.OwnerNode,
		OwnerTeam:    feature.OwnerTeam,
		DataType:     feature.DataType,
		Tags:         feature.Tags,
		Visibility:   string(feature.Visibility),
		Metadata:     feature.Metadata,
		Version:      feature.Version,
		CreatedAt:    feature.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    feature.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ReplicatedTo: feature.ReplicatedTo,
	}

	if feature.AccessControl != nil {
		result.AccessControl = &AccessControlJSON{
			AllowedNodes: feature.AccessControl.AllowedNodes,
			AllowedTeams: feature.AccessControl.AllowedTeams,
			AllowedUsers: feature.AccessControl.AllowedUsers,
			DeniedNodes:  feature.AccessControl.DeniedNodes,
			DeniedTeams:  feature.AccessControl.DeniedTeams,
			DeniedUsers:  feature.AccessControl.DeniedUsers,
			RequireAuth:  feature.AccessControl.RequireAuth,
			RateLimits:   feature.AccessControl.RateLimits,
		}
	}

	return result
}

func (h *FederationHandler) catalogEntryToJSON(entry *federation.CatalogEntry) CatalogEntryJSON {
	result := CatalogEntryJSON{
		Feature:     nil,
		SourceNode:  entry.SourceNode,
		LocalCopy:   entry.LocalCopy,
		AccessCount: entry.AccessCount,
	}

	if entry.Feature != nil {
		feature := h.featureToJSON(entry.Feature)
		result.Feature = &feature
	}

	if !entry.CacheExpiry.IsZero() {
		result.CacheExpiry = entry.CacheExpiry.Format("2006-01-02T15:04:05Z07:00")
	}

	if !entry.LastAccessed.IsZero() {
		result.LastAccessed = entry.LastAccessed.Format("2006-01-02T15:04:05Z07:00")
	}

	return result
}

func (h *FederationHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(w, status, data)
}

func (h *FederationHandler) writeError(w http.ResponseWriter, status int, message string) {
	writeJSONError(w, status, message)
}
