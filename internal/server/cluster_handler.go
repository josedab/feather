package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/feather-store/feather/internal/cluster"
)

// ClusterHandler handles cluster management API requests.
type ClusterHandler struct {
	membership   *cluster.MembershipManager
	ring         *cluster.HashRing
	partitionMap *cluster.PartitionMap
	rebalancer   *cluster.Rebalancer
}

// NewClusterHandler creates a new cluster handler.
func NewClusterHandler(
	membership *cluster.MembershipManager,
	ring *cluster.HashRing,
	partitionMap *cluster.PartitionMap,
	rebalancer *cluster.Rebalancer,
) *ClusterHandler {
	return &ClusterHandler{
		membership:   membership,
		ring:         ring,
		partitionMap: partitionMap,
		rebalancer:   rebalancer,
	}
}

// RegisterRoutes registers cluster API routes.
func (h *ClusterHandler) RegisterRoutes(mux *http.ServeMux) {
	// Cluster membership
	mux.HandleFunc("GET /v1/cluster/nodes", h.handleListNodes)
	mux.HandleFunc("GET /v1/cluster/nodes/{id}", h.handleGetNode)
	mux.HandleFunc("GET /v1/cluster/local", h.handleGetLocalNode)
	mux.HandleFunc("GET /v1/cluster/stats", h.handleGetStats)

	// Hash ring
	mux.HandleFunc("GET /v1/cluster/ring", h.handleGetRing)
	mux.HandleFunc("GET /v1/cluster/ring/lookup", h.handleRingLookup)

	// Partitions
	mux.HandleFunc("GET /v1/cluster/partitions", h.handleListPartitions)
	mux.HandleFunc("GET /v1/cluster/partitions/{id}", h.handleGetPartition)
	mux.HandleFunc("GET /v1/cluster/partitions/key/{key}", h.handleGetPartitionForKey)

	// Rebalancing
	mux.HandleFunc("GET /v1/cluster/rebalance", h.handleGetRebalanceStatus)
	mux.HandleFunc("POST /v1/cluster/rebalance", h.handleTriggerRebalance)
	mux.HandleFunc("DELETE /v1/cluster/rebalance", h.handleCancelRebalance)
	mux.HandleFunc("GET /v1/cluster/rebalance/history", h.handleGetRebalanceHistory)
}

// ClusterNodeJSON represents a cluster node in JSON format.
type ClusterNodeJSON struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	GossipPort    int               `json:"gossip_port"`
	DataPort      int               `json:"data_port"`
	Status        string            `json:"status"`
	Role          string            `json:"role"`
	Zone          string            `json:"zone"`
	Region        string            `json:"region"`
	Weight        int               `json:"weight"`
	VirtualNodes  int               `json:"virtual_nodes"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	JoinedAt      string            `json:"joined_at"`
	LastHeartbeat string            `json:"last_heartbeat"`
	Generation    uint64            `json:"generation"`
}

// HashRingJSON represents the hash ring state.
type HashRingJSON struct {
	NodeCount        int            `json:"node_count"`
	VirtualNodeCount int            `json:"virtual_node_count"`
	Distribution     map[string]int `json:"distribution,omitempty"`
}

// PartitionJSON represents a partition.
type PartitionJSON struct {
	ID          int               `json:"id"`
	Owners      []ClusterNodeJSON `json:"owners"`
	Replication int               `json:"replication"`
}

// RebalancePlanJSON represents a rebalance plan.
type RebalancePlanJSON struct {
	ID          string              `json:"id"`
	Reason      string              `json:"reason"`
	State       string              `json:"state"`
	Tasks       []RebalanceTaskJSON `json:"tasks"`
	CreatedAt   string              `json:"created_at"`
	StartedAt   string              `json:"started_at,omitempty"`
	CompletedAt string              `json:"completed_at,omitempty"`
	TotalBytes  int64               `json:"total_bytes"`
	TotalKeys   int64               `json:"total_keys"`
	Progress    float64             `json:"progress"`
}

// RebalanceTaskJSON represents a rebalance task.
type RebalanceTaskJSON struct {
	ID          string  `json:"id"`
	Partition   int     `json:"partition"`
	FromNode    string  `json:"from_node"`
	ToNode      string  `json:"to_node"`
	State       string  `json:"state"`
	BytesMoved  int64   `json:"bytes_moved"`
	KeysMoved   int64   `json:"keys_moved"`
	StartedAt   string  `json:"started_at,omitempty"`
	CompletedAt string  `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
	Progress    float64 `json:"progress"`
}

// handleListNodes handles GET /v1/cluster/nodes
func (h *ClusterHandler) handleListNodes(w http.ResponseWriter, r *http.Request) {
	if h.membership == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	// Filter by status if provided
	statusFilter := r.URL.Query().Get("status")
	var nodes []*cluster.Node

	if statusFilter == "alive" {
		nodes = h.membership.AliveMembers()
	} else {
		nodes = h.membership.Members()
	}

	response := make([]ClusterNodeJSON, len(nodes))
	for i, node := range nodes {
		response[i] = h.nodeToJSON(node)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"nodes": response,
		"count": len(response),
	})
}

// handleGetNode handles GET /v1/cluster/nodes/{id}
func (h *ClusterHandler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	if h.membership == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	nodeID := r.PathValue("id")
	if nodeID == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "node ID required")
		return
	}

	node, ok := h.membership.GetMember(nodeID)
	if !ok {
		h.writeError(r.Context(), w, http.StatusNotFound, "node not found")
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, h.nodeToJSON(node))
}

// handleGetLocalNode handles GET /v1/cluster/local
func (h *ClusterHandler) handleGetLocalNode(w http.ResponseWriter, r *http.Request) {
	if h.membership == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	node := h.membership.LocalNode()
	h.writeJSON(r.Context(), w, http.StatusOK, h.nodeToJSON(node))
}

// handleGetStats handles GET /v1/cluster/stats
func (h *ClusterHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.membership == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	stats := h.membership.Stats()
	h.writeJSON(r.Context(), w, http.StatusOK, stats)
}

// handleGetRing handles GET /v1/cluster/ring
func (h *ClusterHandler) handleGetRing(w http.ResponseWriter, r *http.Request) {
	if h.ring == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	includeDistribution := r.URL.Query().Get("distribution") == "true"

	response := HashRingJSON{
		NodeCount:        h.ring.NodeCount(),
		VirtualNodeCount: h.ring.VirtualNodeCount(),
	}

	if includeDistribution {
		// Generate sample keys to calculate distribution
		sampleKeys := make([]string, 1000)
		for i := range sampleKeys {
			sampleKeys[i] = "sample-key-" + strconv.Itoa(i)
		}
		response.Distribution = h.ring.GetKeyDistribution(sampleKeys)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, response)
}

// handleRingLookup handles GET /v1/cluster/ring/lookup
func (h *ClusterHandler) handleRingLookup(w http.ResponseWriter, r *http.Request) {
	if h.ring == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "key parameter required")
		return
	}

	countStr := r.URL.Query().Get("count")
	count := 1
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count < 1 {
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid count parameter")
			return
		}
	}

	nodes, err := h.ring.GetNodes(key, count)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ClusterNodeJSON, len(nodes))
	for i, node := range nodes {
		response[i] = h.nodeToJSON(node)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"key":   key,
		"nodes": response,
	})
}

// handleListPartitions handles GET /v1/cluster/partitions
func (h *ClusterHandler) handleListPartitions(w http.ResponseWriter, r *http.Request) {
	if h.partitionMap == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	// Optional filter by node
	nodeFilter := r.URL.Query().Get("node")

	totalPartitions := h.partitionMap.TotalPartitions()
	replicationFactor := h.partitionMap.ReplicationFactor()

	var partitions []PartitionJSON

	if nodeFilter != "" {
		// Get partitions for specific node
		partitionIDs := h.partitionMap.GetLocalPartitions(nodeFilter)
		partitions = make([]PartitionJSON, len(partitionIDs))
		for i, pid := range partitionIDs {
			owners := h.partitionMap.GetOwners(pid)
			partitions[i] = h.partitionToJSON(pid, owners, replicationFactor)
		}
	} else {
		// Get all partitions
		partitions = make([]PartitionJSON, totalPartitions)
		for p := 0; p < totalPartitions; p++ {
			owners := h.partitionMap.GetOwners(p)
			partitions[p] = h.partitionToJSON(p, owners, replicationFactor)
		}
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"partitions":         partitions,
		"total":              totalPartitions,
		"replication_factor": replicationFactor,
		"distribution":       h.partitionMap.GetPartitionDistribution(),
	})
}

// handleGetPartition handles GET /v1/cluster/partitions/{id}
func (h *ClusterHandler) handleGetPartition(w http.ResponseWriter, r *http.Request) {
	if h.partitionMap == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "partition ID required")
		return
	}

	partitionID, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeError(r.Context(), w, http.StatusBadRequest, "invalid partition ID")
		return
	}

	totalPartitions := h.partitionMap.TotalPartitions()
	if partitionID < 0 || partitionID >= totalPartitions {
		h.writeError(r.Context(), w, http.StatusNotFound, "partition not found")
		return
	}

	owners := h.partitionMap.GetOwners(partitionID)
	response := h.partitionToJSON(partitionID, owners, h.partitionMap.ReplicationFactor())

	h.writeJSON(r.Context(), w, http.StatusOK, response)
}

// handleGetPartitionForKey handles GET /v1/cluster/partitions/key/{key}
func (h *ClusterHandler) handleGetPartitionForKey(w http.ResponseWriter, r *http.Request) {
	if h.partitionMap == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	key := r.PathValue("key")
	if key == "" {
		h.writeError(r.Context(), w, http.StatusBadRequest, "key required")
		return
	}

	partitionID := h.partitionMap.GetPartitionForKey(key)
	owners := h.partitionMap.GetOwnersForKey(key)

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"key":       key,
		"partition": h.partitionToJSON(partitionID, owners, h.partitionMap.ReplicationFactor()),
	})
}

// handleGetRebalanceStatus handles GET /v1/cluster/rebalance
func (h *ClusterHandler) handleGetRebalanceStatus(w http.ResponseWriter, r *http.Request) {
	if h.rebalancer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	stats := h.rebalancer.Stats()
	response := map[string]interface{}{
		"total_rebalances":       stats.TotalRebalances,
		"successful_rebalances":  stats.SuccessfulRebalances,
		"failed_rebalances":      stats.FailedRebalances,
		"partition_distribution": stats.PartitionDistribution,
	}

	if !stats.LastRebalance.IsZero() {
		response["last_rebalance"] = stats.LastRebalance.Format("2006-01-02T15:04:05Z07:00")
	}

	if stats.CurrentPlan != nil {
		response["current_plan"] = h.planToJSON(stats.CurrentPlan)
	}

	h.writeJSON(r.Context(), w, http.StatusOK, response)
}

// handleTriggerRebalance handles POST /v1/cluster/rebalance
func (h *ClusterHandler) handleTriggerRebalance(w http.ResponseWriter, r *http.Request) {
	if h.rebalancer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	plan, err := h.rebalancer.TriggerRebalance()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusAccepted, map[string]interface{}{
		"success": true,
		"plan":    h.planToJSON(plan),
	})
}

// handleCancelRebalance handles DELETE /v1/cluster/rebalance
func (h *ClusterHandler) handleCancelRebalance(w http.ResponseWriter, r *http.Request) {
	if h.rebalancer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	err := h.rebalancer.CancelRebalance()
	if err != nil {
		h.writeError(r.Context(), w, http.StatusConflict, err.Error())
		return
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// handleGetRebalanceHistory handles GET /v1/cluster/rebalance/history
func (h *ClusterHandler) handleGetRebalanceHistory(w http.ResponseWriter, r *http.Request) {
	if h.rebalancer == nil {
		h.writeError(r.Context(), w, http.StatusServiceUnavailable, "cluster not configured")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			h.writeError(r.Context(), w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
	}

	history := h.rebalancer.GetPlanHistory()

	// Return most recent first
	start := 0
	if len(history) > limit {
		start = len(history) - limit
	}

	response := make([]RebalancePlanJSON, 0, limit)
	for i := len(history) - 1; i >= start; i-- {
		response = append(response, h.planToJSON(history[i]))
	}

	h.writeJSON(r.Context(), w, http.StatusOK, map[string]interface{}{
		"history": response,
		"count":   len(response),
	})
}

func (h *ClusterHandler) nodeToJSON(node *cluster.Node) ClusterNodeJSON {
	return ClusterNodeJSON{
		ID:            node.ID,
		Name:          node.Name,
		Address:       node.Address,
		GossipPort:    node.GossipPort,
		DataPort:      node.DataPort,
		Status:        string(node.Status),
		Role:          string(node.Role),
		Zone:          node.Zone,
		Region:        node.Region,
		Weight:        node.Weight,
		VirtualNodes:  node.VirtualNodes,
		Metadata:      node.Metadata,
		JoinedAt:      node.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastHeartbeat: node.LastHeartbeat.Format("2006-01-02T15:04:05Z07:00"),
		Generation:    node.Generation,
	}
}

func (h *ClusterHandler) partitionToJSON(id int, owners []*cluster.Node, replication int) PartitionJSON {
	ownerJSON := make([]ClusterNodeJSON, len(owners))
	for i, owner := range owners {
		ownerJSON[i] = h.nodeToJSON(owner)
	}
	return PartitionJSON{
		ID:          id,
		Owners:      ownerJSON,
		Replication: replication,
	}
}

func (h *ClusterHandler) planToJSON(plan *cluster.RebalancePlan) RebalancePlanJSON {
	result := RebalancePlanJSON{
		ID:         plan.ID,
		Reason:     string(plan.Reason),
		State:      string(plan.State),
		CreatedAt:  plan.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		TotalBytes: plan.TotalBytes,
		TotalKeys:  plan.TotalKeys,
		Progress:   plan.Progress(),
	}

	if !plan.StartedAt.IsZero() {
		result.StartedAt = plan.StartedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if !plan.CompletedAt.IsZero() {
		result.CompletedAt = plan.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	result.Tasks = make([]RebalanceTaskJSON, len(plan.Tasks))
	for i, task := range plan.Tasks {
		result.Tasks[i] = h.taskToJSON(task)
	}

	return result
}

func (h *ClusterHandler) taskToJSON(task *cluster.RebalanceTask) RebalanceTaskJSON {
	result := RebalanceTaskJSON{
		ID:         task.ID,
		Partition:  task.Partition,
		FromNode:   task.FromNode,
		ToNode:     task.ToNode,
		State:      string(task.State),
		BytesMoved: task.BytesMoved,
		KeysMoved:  task.KeysMoved,
		Error:      task.Error,
		Progress:   task.Progress,
	}

	if !task.StartedAt.IsZero() {
		result.StartedAt = task.StartedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if !task.CompletedAt.IsZero() {
		result.CompletedAt = task.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	return result
}

func (h *ClusterHandler) writeJSON(ctx context.Context, w http.ResponseWriter, status int, data interface{}) {
	writeJSONResponse(ctx, w, status, data)
}

func (h *ClusterHandler) writeError(ctx context.Context, w http.ResponseWriter, status int, message string) {
	h.writeJSON(ctx, w, status, map[string]interface{}{
		"error": message,
	})
}
