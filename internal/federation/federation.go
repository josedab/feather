package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Federation manages a federated feature store network.
type Federation struct {
	mu            sync.RWMutex
	config        Config
	localNode     *Node
	nodes         map[string]*Node
	catalog       map[string]*CatalogEntry
	policies      map[string]*ReplicationPolicy
	httpClient    *http.Client
	syncTicker    *time.Ticker
	healthTicker  *time.Ticker
	stopCh        chan struct{}
	eventHandlers []EventHandler
}

// NewFederation creates a new federation manager.
func NewFederation(config Config) *Federation {
	localNode := &Node{
		ID:       config.NodeID,
		Name:     config.NodeName,
		Address:  config.NodeAddress,
		Role:     NodeRolePeer,
		State:    NodeStateHealthy,
		Region:   config.Region,
		JoinedAt: time.Now(),
		LastSeen: time.Now(),
		Permissions: &NodePermissions{
			CanRead:      true,
			CanWrite:     true,
			CanReplicate: true,
		},
	}

	return &Federation{
		config:    config,
		localNode: localNode,
		nodes:     make(map[string]*Node),
		catalog:   make(map[string]*CatalogEntry),
		policies:  make(map[string]*ReplicationPolicy),
		httpClient: &http.Client{
			Timeout: config.RequestTimeout,
		},
		stopCh: make(chan struct{}),
	}
}

// Start starts the federation background tasks.
func (f *Federation) Start() {
	f.syncTicker = time.NewTicker(f.config.SyncInterval)
	f.healthTicker = time.NewTicker(f.config.HealthCheckInterval)

	go f.runHealthChecks()
	go f.runSync()
}

// Stop stops the federation background tasks.
func (f *Federation) Stop() {
	close(f.stopCh)
	if f.syncTicker != nil {
		f.syncTicker.Stop()
	}
	if f.healthTicker != nil {
		f.healthTicker.Stop()
	}
}

// JoinNode adds a node to the federation.
func (f *Federation) JoinNode(node *Node) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if node.ID == "" {
		return ErrInvalidNodeID
	}

	if _, exists := f.nodes[node.ID]; exists {
		return ErrNodeAlreadyExists
	}

	node.JoinedAt = time.Now()
	node.LastSeen = time.Now()
	if node.State == "" {
		node.State = NodeStateSyncing
	}

	f.nodes[node.ID] = node

	f.emitEvent(Event{
		Type:      EventNodeJoined,
		NodeID:    node.ID,
		Timestamp: time.Now(),
	})

	return nil
}

// LeaveNode removes a node from the federation.
func (f *Federation) LeaveNode(nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}

	delete(f.nodes, nodeID)

	f.emitEvent(Event{
		Type:      EventNodeLeft,
		NodeID:    nodeID,
		Timestamp: time.Now(),
	})

	return nil
}

// GetNode returns a node by ID.
func (f *Federation) GetNode(nodeID string) (*Node, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	node, exists := f.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	return node, nil
}

// ListNodes returns all nodes in the federation.
func (f *Federation) ListNodes() []*Node {
	f.mu.RLock()
	defer f.mu.RUnlock()

	nodes := make([]*Node, 0, len(f.nodes)+1)
	nodes = append(nodes, f.localNode)

	for _, node := range f.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// ShareFeature shares a feature with the federation.
func (f *Federation) ShareFeature(feature *FederatedFeature) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if feature.ID == "" {
		return ErrInvalidFeatureID
	}

	feature.OwnerNode = f.localNode.ID
	feature.CreatedAt = time.Now()
	feature.UpdatedAt = time.Now()
	feature.Version = 1

	entry := &CatalogEntry{
		Feature:    feature,
		SourceNode: f.localNode.ID,
		LocalCopy:  true,
	}

	f.catalog[feature.ID] = entry

	f.emitEvent(Event{
		Type:      EventFeatureShared,
		NodeID:    f.localNode.ID,
		FeatureID: feature.ID,
		Timestamp: time.Now(),
	})

	return nil
}

// GetFeature returns a feature from the catalog.
func (f *Federation) GetFeature(ctx context.Context, featureID string) (*FederatedFeature, error) {
	f.mu.RLock()
	entry, exists := f.catalog[featureID]
	f.mu.RUnlock()

	if !exists {
		// Try to fetch from remote nodes
		return f.fetchRemoteFeature(ctx, featureID)
	}

	// Update access stats
	f.mu.Lock()
	entry.AccessCount++
	entry.LastAccessed = time.Now()
	f.mu.Unlock()

	return entry.Feature, nil
}

// SearchFeatures searches for features in the catalog.
func (f *Federation) SearchFeatures(query SearchQuery) ([]*CatalogEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var results []*CatalogEntry

	for _, entry := range f.catalog {
		if f.matchesQuery(entry, query) {
			if f.hasAccess(entry.Feature) {
				results = append(results, entry)
			}
		}
	}

	return results, nil
}

func (f *Federation) matchesQuery(entry *CatalogEntry, query SearchQuery) bool {
	feature := entry.Feature

	// Name filter
	if query.Name != "" && feature.Name != query.Name {
		return false
	}

	// Tags filter
	if len(query.Tags) > 0 {
		found := false
		for _, tag := range query.Tags {
			for _, ft := range feature.Tags {
				if tag == ft {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Team filter
	if len(query.Teams) > 0 {
		found := false
		for _, team := range query.Teams {
			if team == feature.OwnerTeam {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Visibility filter
	if len(query.Visibility) > 0 {
		found := false
		for _, v := range query.Visibility {
			if v == feature.Visibility {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Data type filter
	if len(query.DataTypes) > 0 {
		found := false
		for _, dt := range query.DataTypes {
			if dt == feature.DataType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (f *Federation) hasAccess(feature *FederatedFeature) bool {
	// Check visibility
	switch feature.Visibility {
	case VisibilityPublic, VisibilityFederation:
		return true
	case VisibilityPrivate:
		return feature.OwnerNode == f.localNode.ID
	case VisibilityTeam, VisibilityOrg:
		return true
	}

	// Check access control
	if feature.AccessControl != nil {
		ac := feature.AccessControl

		// Check denied lists
		for _, nodeID := range ac.DeniedNodes {
			if nodeID == f.localNode.ID {
				return false
			}
		}

		// Check allowed lists
		if len(ac.AllowedNodes) > 0 {
			allowed := false
			for _, nodeID := range ac.AllowedNodes {
				if nodeID == f.localNode.ID {
					allowed = true
					break
				}
			}
			if !allowed {
				return false
			}
		}
	}

	return true
}

// SetReplicationPolicy sets the replication policy for a feature.
func (f *Federation) SetReplicationPolicy(featureID string, policy *ReplicationPolicy) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.catalog[featureID]; !exists {
		return ErrFeatureNotFound
	}

	f.policies[featureID] = policy

	return nil
}

// ReplicateFeature replicates a feature to target nodes.
func (f *Federation) ReplicateFeature(ctx context.Context, featureID string, targetNodes []string) error {
	f.mu.RLock()
	entry, exists := f.catalog[featureID]
	f.mu.RUnlock()

	if !exists {
		return ErrFeatureNotFound
	}

	for _, nodeID := range targetNodes {
		if err := f.replicateToNode(ctx, entry.Feature, nodeID); err != nil {
			return fmt.Errorf("replication to %s failed: %w", nodeID, err)
		}
	}

	// Update replicated_to list
	f.mu.Lock()
	entry.Feature.ReplicatedTo = append(entry.Feature.ReplicatedTo, targetNodes...)
	f.mu.Unlock()

	f.emitEvent(Event{
		Type:      EventReplicationDone,
		NodeID:    f.localNode.ID,
		FeatureID: featureID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"target_nodes": targetNodes,
		},
	})

	return nil
}

func (f *Federation) replicateToNode(ctx context.Context, feature *FederatedFeature, nodeID string) error {
	f.mu.RLock()
	node, exists := f.nodes[nodeID]
	f.mu.RUnlock()

	if !exists {
		return ErrNodeNotFound
	}

	if !node.Permissions.CanReplicate {
		return ErrReplicationDenied
	}

	// In a real implementation, this would send the feature to the target node
	// For now, we simulate the replication
	_ = node.Address

	return nil
}

func (f *Federation) fetchRemoteFeature(ctx context.Context, featureID string) (*FederatedFeature, error) {
	f.mu.RLock()
	nodes := make([]*Node, 0, len(f.nodes))
	for _, node := range f.nodes {
		if node.State == NodeStateHealthy {
			nodes = append(nodes, node)
		}
	}
	f.mu.RUnlock()

	for _, node := range nodes {
		feature, err := f.fetchFromNode(ctx, node, featureID)
		if err == nil {
			// Cache locally
			f.mu.Lock()
			f.catalog[featureID] = &CatalogEntry{
				Feature:     feature,
				SourceNode:  node.ID,
				LocalCopy:   false,
				CacheExpiry: time.Now().Add(5 * time.Minute),
			}
			f.mu.Unlock()

			return feature, nil
		}
	}

	return nil, ErrFeatureNotFound
}

func (f *Federation) fetchFromNode(ctx context.Context, node *Node, featureID string) (*FederatedFeature, error) {
	url := fmt.Sprintf("%s/v1/federation/features/%s", node.Address, featureID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var feature FederatedFeature
	if err := json.Unmarshal(body, &feature); err != nil {
		return nil, err
	}

	return &feature, nil
}

func (f *Federation) runHealthChecks() {
	for {
		select {
		case <-f.stopCh:
			return
		case <-f.healthTicker.C:
			f.checkNodeHealth()
		}
	}
}

func (f *Federation) checkNodeHealth() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, node := range f.nodes {
		healthy := f.pingNode(node)
		if healthy {
			node.State = NodeStateHealthy
			node.LastSeen = time.Now()
		} else {
			if node.State == NodeStateHealthy {
				node.State = NodeStateUnhealthy
				f.emitEvent(Event{
					Type:      EventNodeUnhealthy,
					NodeID:    node.ID,
					Timestamp: time.Now(),
				})
			} else if time.Since(node.LastSeen) > 5*f.config.HealthCheckInterval {
				node.State = NodeStateUnreachable
			}
		}
	}
}

func (f *Federation) pingNode(node *Node) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/v1/health", node.Address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return resp.StatusCode == http.StatusOK
}

func (f *Federation) runSync() {
	for {
		select {
		case <-f.stopCh:
			return
		case <-f.syncTicker.C:
			f.syncCatalog()
		}
	}
}

func (f *Federation) syncCatalog() {
	f.mu.RLock()
	nodes := make([]*Node, 0, len(f.nodes))
	for _, node := range f.nodes {
		if node.State == NodeStateHealthy {
			nodes = append(nodes, node)
		}
	}
	f.mu.RUnlock()

	for _, node := range nodes {
		f.syncWithNode(context.Background(), node)
	}

	f.emitEvent(Event{
		Type:      EventSyncCompleted,
		NodeID:    f.localNode.ID,
		Timestamp: time.Now(),
	})
}

func (f *Federation) syncWithNode(ctx context.Context, node *Node) {
	url := fmt.Sprintf("%s/v1/federation/catalog", node.Address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var catalog struct {
		Features []*FederatedFeature `json:"features"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, feature := range catalog.Features {
		if f.hasAccess(feature) {
			existing, exists := f.catalog[feature.ID]
			if !exists || existing.Feature.Version < feature.Version {
				f.catalog[feature.ID] = &CatalogEntry{
					Feature:    feature,
					SourceNode: node.ID,
					LocalCopy:  false,
				}
			}
		}
	}
}

// OnEvent registers an event handler.
func (f *Federation) OnEvent(handler EventHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventHandlers = append(f.eventHandlers, handler)
}

func (f *Federation) emitEvent(event Event) {
	for _, handler := range f.eventHandlers {
		go handler(event)
	}
}

// GetStats returns federation statistics.
func (f *Federation) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	healthyNodes := 0
	for _, node := range f.nodes {
		if node.State == NodeStateHealthy {
			healthyNodes++
		}
	}

	localFeatures := 0
	remoteFeatures := 0
	for _, entry := range f.catalog {
		if entry.LocalCopy {
			localFeatures++
		} else {
			remoteFeatures++
		}
	}

	regions := make(map[string]int)
	for _, node := range f.nodes {
		regions[node.Region]++
	}

	return map[string]interface{}{
		"local_node":      f.localNode.ID,
		"total_nodes":     len(f.nodes) + 1,
		"healthy_nodes":   healthyNodes + 1,
		"total_features":  len(f.catalog),
		"local_features":  localFeatures,
		"remote_features": remoteFeatures,
		"regions":         regions,
	}
}

// GetLocalNode returns the local node information.
func (f *Federation) GetLocalNode() *Node {
	return f.localNode
}

// UpdateFeature updates a shared feature.
func (f *Federation) UpdateFeature(feature *FederatedFeature) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	entry, exists := f.catalog[feature.ID]
	if !exists {
		return ErrFeatureNotFound
	}

	if entry.Feature.OwnerNode != f.localNode.ID {
		return ErrNotFeatureOwner
	}

	feature.UpdatedAt = time.Now()
	feature.Version = entry.Feature.Version + 1
	entry.Feature = feature

	f.emitEvent(Event{
		Type:      EventFeatureUpdated,
		NodeID:    f.localNode.ID,
		FeatureID: feature.ID,
		Timestamp: time.Now(),
	})

	return nil
}

// DeleteFeature removes a feature from the catalog.
func (f *Federation) DeleteFeature(featureID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	entry, exists := f.catalog[featureID]
	if !exists {
		return ErrFeatureNotFound
	}

	if entry.Feature.OwnerNode != f.localNode.ID {
		return ErrNotFeatureOwner
	}

	delete(f.catalog, featureID)
	delete(f.policies, featureID)

	f.emitEvent(Event{
		Type:      EventFeatureDeleted,
		NodeID:    f.localNode.ID,
		FeatureID: featureID,
		Timestamp: time.Now(),
	})

	return nil
}

// ListCatalog returns all features in the catalog.
func (f *Federation) ListCatalog() []*CatalogEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries := make([]*CatalogEntry, 0, len(f.catalog))
	for _, entry := range f.catalog {
		if f.hasAccess(entry.Feature) {
			entries = append(entries, entry)
		}
	}

	return entries
}
