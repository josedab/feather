package groups

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// FeatureGroup represents a collection of related features.
type FeatureGroup struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	EntityType  string            `json:"entity_type"`
	Features    []GroupFeature    `json:"features"`
	Tags        []string          `json:"tags"`
	Owner       string            `json:"owner"`
	Team        string            `json:"team"`
	Version     int               `json:"version"`
	Status      GroupStatus       `json:"status"`
	TTL         time.Duration     `json:"ttl,omitempty"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CreatedBy   string            `json:"created_by"`
	UpdatedBy   string            `json:"updated_by"`
}

// GroupFeature represents a feature within a group.
type GroupFeature struct {
	Name        string      `json:"name"`
	DataType    string      `json:"data_type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Validator   string      `json:"validator,omitempty"` // Validation rule
}

// GroupStatus represents the status of a feature group.
type GroupStatus string

const (
	GroupStatusDraft      GroupStatus = "draft"
	GroupStatusActive     GroupStatus = "active"
	GroupStatusDeprecated GroupStatus = "deprecated"
	GroupStatusArchived   GroupStatus = "archived"
)

// GroupView represents a view/projection of a feature group.
type GroupView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	GroupID     string    `json:"group_id"`
	Features    []string  `json:"features"` // Subset of group features
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Manager manages feature groups.
type Manager struct {
	groups   map[string]*FeatureGroup
	views    map[string]*GroupView
	byEntity map[string][]string        // entity_type -> group IDs
	byTag    map[string][]string        // tag -> group IDs
	versions map[string][]*FeatureGroup // group ID -> version history
	mu       sync.RWMutex
}

// NewManager creates a new feature group manager.
func NewManager() *Manager {
	return &Manager{
		groups:   make(map[string]*FeatureGroup),
		views:    make(map[string]*GroupView),
		byEntity: make(map[string][]string),
		byTag:    make(map[string][]string),
		versions: make(map[string][]*FeatureGroup),
	}
}

// CreateGroup creates a new feature group.
func (m *Manager) CreateGroup(group *FeatureGroup, createdBy string) error {
	if group.ID == "" {
		return ErrGroupIDRequired
	}
	if group.Name == "" {
		return ErrGroupNameRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[group.ID]; ok {
		return ErrGroupExists
	}

	now := time.Now()
	group.Version = 1
	group.Status = GroupStatusDraft
	group.CreatedAt = now
	group.UpdatedAt = now
	group.CreatedBy = createdBy
	group.UpdatedBy = createdBy

	if group.Metadata == nil {
		group.Metadata = make(map[string]string)
	}

	m.groups[group.ID] = group
	m.indexGroup(group)

	return nil
}

// UpdateGroup updates an existing feature group.
func (m *Manager) UpdateGroup(group *FeatureGroup, updatedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.groups[group.ID]
	if !ok {
		return ErrGroupNotFound
	}

	// Save old version
	oldCopy := *existing
	m.versions[group.ID] = append(m.versions[group.ID], &oldCopy)

	// Remove old indexes
	m.removeGroupIndex(existing)

	// Update
	group.Version = existing.Version + 1
	group.CreatedAt = existing.CreatedAt
	group.CreatedBy = existing.CreatedBy
	group.UpdatedAt = time.Now()
	group.UpdatedBy = updatedBy

	m.groups[group.ID] = group
	m.indexGroup(group)

	return nil
}

func (m *Manager) indexGroup(group *FeatureGroup) {
	if group.EntityType != "" {
		m.byEntity[group.EntityType] = append(m.byEntity[group.EntityType], group.ID)
	}

	for _, tag := range group.Tags {
		m.byTag[tag] = append(m.byTag[tag], group.ID)
	}
}

func (m *Manager) removeGroupIndex(group *FeatureGroup) {
	if group.EntityType != "" {
		m.byEntity[group.EntityType] = removeFromSlice(m.byEntity[group.EntityType], group.ID)
	}

	for _, tag := range group.Tags {
		m.byTag[tag] = removeFromSlice(m.byTag[tag], group.ID)
	}
}

func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// GetGroup retrieves a feature group by ID.
func (m *Manager) GetGroup(id string) *FeatureGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.groups[id]
}

// GetGroupVersion retrieves a specific version of a group.
func (m *Manager) GetGroupVersion(id string, version int) *FeatureGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check current version
	if current, ok := m.groups[id]; ok && current.Version == version {
		return current
	}

	// Check version history
	for _, g := range m.versions[id] {
		if g.Version == version {
			return g
		}
	}

	return nil
}

// ListGroups lists all feature groups.
func (m *Manager) ListGroups(filter *GroupFilter) []*FeatureGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*FeatureGroup
	for _, group := range m.groups {
		if filter == nil || filter.Matches(group) {
			result = append(result, group)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// GroupFilter defines filtering criteria for groups.
type GroupFilter struct {
	EntityType string      `json:"entity_type,omitempty"`
	Status     GroupStatus `json:"status,omitempty"`
	Owner      string      `json:"owner,omitempty"`
	Team       string      `json:"team,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
}

// Matches checks if a group matches the filter.
func (f *GroupFilter) Matches(group *FeatureGroup) bool {
	if f.EntityType != "" && group.EntityType != f.EntityType {
		return false
	}

	if f.Status != "" && group.Status != f.Status {
		return false
	}

	if f.Owner != "" && group.Owner != f.Owner {
		return false
	}

	if f.Team != "" && group.Team != f.Team {
		return false
	}

	if len(f.Tags) > 0 {
		hasTag := false
		for _, filterTag := range f.Tags {
			for _, groupTag := range group.Tags {
				if filterTag == groupTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	return true
}

// DeleteGroup deletes a feature group.
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return ErrGroupNotFound
	}

	m.removeGroupIndex(group)
	delete(m.groups, id)

	// Delete associated views
	for viewID, view := range m.views {
		if view.GroupID == id {
			delete(m.views, viewID)
		}
	}

	return nil
}

// SetStatus updates the status of a feature group.
func (m *Manager) SetStatus(id string, status GroupStatus, updatedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[id]
	if !ok {
		return ErrGroupNotFound
	}

	group.Status = status
	group.UpdatedAt = time.Now()
	group.UpdatedBy = updatedBy

	return nil
}

// AddFeature adds a feature to a group.
func (m *Manager) AddFeature(groupID string, feature GroupFeature, updatedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}

	// Check for duplicate
	for _, f := range group.Features {
		if f.Name == feature.Name {
			return ErrFeatureExists
		}
	}

	group.Features = append(group.Features, feature)
	group.UpdatedAt = time.Now()
	group.UpdatedBy = updatedBy
	group.Version++

	return nil
}

// RemoveFeature removes a feature from a group.
func (m *Manager) RemoveFeature(groupID, featureName, updatedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return ErrGroupNotFound
	}

	found := false
	features := make([]GroupFeature, 0, len(group.Features))
	for _, f := range group.Features {
		if f.Name == featureName {
			found = true
		} else {
			features = append(features, f)
		}
	}

	if !found {
		return ErrFeatureNotInGroup
	}

	group.Features = features
	group.UpdatedAt = time.Now()
	group.UpdatedBy = updatedBy
	group.Version++

	return nil
}

// GetFeatureNames returns all feature names in a group.
func (m *Manager) GetFeatureNames(groupID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.groups[groupID]
	if !ok {
		return nil
	}

	names := make([]string, len(group.Features))
	for i, f := range group.Features {
		names[i] = f.Name
	}

	return names
}

// GetGroupsByEntity returns all groups for an entity type.
func (m *Manager) GetGroupsByEntity(entityType string) []*FeatureGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byEntity[entityType]
	groups := make([]*FeatureGroup, 0, len(ids))
	for _, id := range ids {
		if g, ok := m.groups[id]; ok {
			groups = append(groups, g)
		}
	}

	return groups
}

// GetGroupsByTag returns all groups with a specific tag.
func (m *Manager) GetGroupsByTag(tag string) []*FeatureGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.byTag[tag]
	groups := make([]*FeatureGroup, 0, len(ids))
	for _, id := range ids {
		if g, ok := m.groups[id]; ok {
			groups = append(groups, g)
		}
	}

	return groups
}

// CreateView creates a view/projection of a group.
func (m *Manager) CreateView(view *GroupView) error {
	if view.ID == "" {
		return ErrViewIDRequired
	}
	if view.GroupID == "" {
		return ErrGroupIDRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[view.GroupID]; !ok {
		return ErrGroupNotFound
	}

	if _, ok := m.views[view.ID]; ok {
		return ErrViewExists
	}

	view.CreatedAt = time.Now()
	m.views[view.ID] = view

	return nil
}

// GetView retrieves a view by ID.
func (m *Manager) GetView(id string) *GroupView {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.views[id]
}

// ListViews lists all views for a group.
func (m *Manager) ListViews(groupID string) []*GroupView {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var views []*GroupView
	for _, view := range m.views {
		if groupID == "" || view.GroupID == groupID {
			views = append(views, view)
		}
	}

	return views
}

// DeleteView deletes a view.
func (m *Manager) DeleteView(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.views[id]; !ok {
		return ErrViewNotFound
	}

	delete(m.views, id)
	return nil
}

// GetViewFeatures returns the feature names for a view.
func (m *Manager) GetViewFeatures(viewID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	view, ok := m.views[viewID]
	if !ok {
		return nil
	}

	return view.Features
}

// GetStats returns group manager statistics.
func (m *Manager) GetStats() GroupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := GroupStats{
		TotalGroups:  len(m.groups),
		TotalViews:   len(m.views),
		ByStatus:     make(map[GroupStatus]int),
		ByEntityType: make(map[string]int),
	}

	totalFeatures := 0
	for _, group := range m.groups {
		stats.ByStatus[group.Status]++
		if group.EntityType != "" {
			stats.ByEntityType[group.EntityType]++
		}
		totalFeatures += len(group.Features)
	}
	stats.TotalFeatures = totalFeatures

	return stats
}

// GroupStats contains group manager statistics.
type GroupStats struct {
	TotalGroups   int                 `json:"total_groups"`
	TotalViews    int                 `json:"total_views"`
	TotalFeatures int                 `json:"total_features"`
	ByStatus      map[GroupStatus]int `json:"by_status"`
	ByEntityType  map[string]int      `json:"by_entity_type"`
}

// Errors
var (
	ErrGroupIDRequired   = errors.New("group ID is required")
	ErrGroupNameRequired = errors.New("group name is required")
	ErrGroupExists       = errors.New("group already exists")
	ErrGroupNotFound     = errors.New("group not found")
	ErrFeatureExists     = errors.New("feature already exists in group")
	ErrFeatureNotInGroup = errors.New("feature not found in group")
	ErrViewIDRequired    = errors.New("view ID is required")
	ErrViewExists        = errors.New("view already exists")
	ErrViewNotFound      = errors.New("view not found")
)
