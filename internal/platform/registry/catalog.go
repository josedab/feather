package registry

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// FeatureDefinition represents a feature in the catalog.
type FeatureDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	DataType    string            `json:"data_type"`   // int, float, string, bool, vector, etc.
	EntityType  string            `json:"entity_type"` // user, item, transaction, etc.
	Owner       string            `json:"owner"`       // Team or person responsible
	Team        string            `json:"team"`
	Tags        []string          `json:"tags"`
	Category    string            `json:"category"` // demographic, behavioral, derived, etc.
	Source      FeatureSource     `json:"source"`
	Schema      *FeatureSchema    `json:"schema,omitempty"`
	Freshness   FreshnessConfig   `json:"freshness"`
	Metadata    map[string]string `json:"metadata"`

	// Lifecycle
	Status       FeatureStatus `json:"status"`
	Version      int           `json:"version"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	CreatedBy    string        `json:"created_by"`
	UpdatedBy    string        `json:"updated_by"`
	DeprecatedAt *time.Time    `json:"deprecated_at,omitempty"`

	// Lineage
	Upstream        []string `json:"upstream"`                  // Features this depends on
	Downstream      []string `json:"downstream"`                // Features depending on this
	Transformations []string `json:"transformations,omitempty"` // Applied transforms
}

// FeatureSource describes where a feature comes from.
type FeatureSource struct {
	Type     string `json:"type"`   // batch, streaming, derived, external
	System   string `json:"system"` // kafka, snowflake, transform, etc.
	Table    string `json:"table,omitempty"`
	Topic    string `json:"topic,omitempty"`
	Query    string `json:"query,omitempty"`
	Schedule string `json:"schedule,omitempty"` // Cron expression for batch
}

// FeatureSchema defines the expected schema for a feature.
type FeatureSchema struct {
	Type        string                 `json:"type"`
	Nullable    bool                   `json:"nullable"`
	Default     interface{}            `json:"default,omitempty"`
	Constraints map[string]interface{} `json:"constraints,omitempty"` // min, max, enum, pattern
	Dimensions  []int                  `json:"dimensions,omitempty"`  // For vectors/arrays
}

// FreshnessConfig defines freshness requirements.
type FreshnessConfig struct {
	MaxAge     time.Duration `json:"max_age"`
	SLA        time.Duration `json:"sla"`
	AlertAfter time.Duration `json:"alert_after"`
}

// FeatureStatus represents the lifecycle status of a feature.
type FeatureStatus string

// FeatureStatus constants for feature lifecycle.
const (
	StatusDraft      FeatureStatus = "draft"
	StatusActive     FeatureStatus = "active"
	StatusDeprecated FeatureStatus = "deprecated"
	StatusArchived   FeatureStatus = "archived"
)

// Catalog is the central feature registry.
type Catalog struct {
	features   map[string]*FeatureDefinition
	byTag      map[string][]string             // tag -> feature names
	byOwner    map[string][]string             // owner -> feature names
	byTeam     map[string][]string             // team -> feature names
	byCategory map[string][]string             // category -> feature names
	byEntity   map[string][]string             // entity_type -> feature names
	versions   map[string][]*FeatureDefinition // feature name -> version history
	popularity map[string]*FeaturePopularity // feature name -> popularity
	mu         sync.RWMutex
}

// FeaturePopularity tracks usage metrics for a feature.
type FeaturePopularity struct {
	FeatureName string    `json:"feature_name"`
	ViewCount   int64     `json:"view_count"`
	QueryCount  int64     `json:"query_count"`
	LastQueried time.Time `json:"last_queried"`
	LastViewed  time.Time `json:"last_viewed"`
	Score       float64   `json:"score"`
}

// NewCatalog creates a new feature catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		features:   make(map[string]*FeatureDefinition),
		byTag:      make(map[string][]string),
		byOwner:    make(map[string][]string),
		byTeam:     make(map[string][]string),
		byCategory: make(map[string][]string),
		byEntity:   make(map[string][]string),
		versions:   make(map[string][]*FeatureDefinition),
		popularity: make(map[string]*FeaturePopularity),
	}
}

// Register adds or updates a feature definition.
func (c *Catalog) Register(def *FeatureDefinition, registeredBy string) error {
	if def.Name == "" {
		return ErrNameRequired
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	existing, exists := c.features[def.Name]
	if exists {
		// Update existing - save old version to history
		oldCopy := *existing
		c.versions[def.Name] = append(c.versions[def.Name], &oldCopy)

		def.Version = existing.Version + 1
		def.CreatedAt = existing.CreatedAt
		def.CreatedBy = existing.CreatedBy
		def.UpdatedAt = now
		def.UpdatedBy = registeredBy

		// Remove old indexes
		c.removeIndexes(existing)
	} else {
		def.Version = 1
		def.CreatedAt = now
		def.CreatedBy = registeredBy
		def.UpdatedAt = now
		def.UpdatedBy = registeredBy
	}

	if def.Status == "" {
		def.Status = StatusDraft
	}

	c.features[def.Name] = def
	c.addIndexes(def)

	return nil
}

func (c *Catalog) addIndexes(def *FeatureDefinition) {
	// Index by tags
	for _, tag := range def.Tags {
		c.byTag[tag] = append(c.byTag[tag], def.Name)
	}

	// Index by owner
	if def.Owner != "" {
		c.byOwner[def.Owner] = append(c.byOwner[def.Owner], def.Name)
	}

	// Index by team
	if def.Team != "" {
		c.byTeam[def.Team] = append(c.byTeam[def.Team], def.Name)
	}

	// Index by category
	if def.Category != "" {
		c.byCategory[def.Category] = append(c.byCategory[def.Category], def.Name)
	}

	// Index by entity type
	if def.EntityType != "" {
		c.byEntity[def.EntityType] = append(c.byEntity[def.EntityType], def.Name)
	}
}

func (c *Catalog) removeIndexes(def *FeatureDefinition) {
	// Remove from tag index
	for _, tag := range def.Tags {
		c.byTag[tag] = removeFromSlice(c.byTag[tag], def.Name)
	}

	// Remove from owner index
	if def.Owner != "" {
		c.byOwner[def.Owner] = removeFromSlice(c.byOwner[def.Owner], def.Name)
	}

	// Remove from team index
	if def.Team != "" {
		c.byTeam[def.Team] = removeFromSlice(c.byTeam[def.Team], def.Name)
	}

	// Remove from category index
	if def.Category != "" {
		c.byCategory[def.Category] = removeFromSlice(c.byCategory[def.Category], def.Name)
	}

	// Remove from entity index
	if def.EntityType != "" {
		c.byEntity[def.EntityType] = removeFromSlice(c.byEntity[def.EntityType], def.Name)
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

// Get retrieves a feature definition by name.
func (c *Catalog) Get(name string) *FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.features[name]
}

// GetVersion retrieves a specific version of a feature.
func (c *Catalog) GetVersion(name string, version int) *FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if version <= 0 {
		return c.features[name]
	}

	history := c.versions[name]
	for _, def := range history {
		if def.Version == version {
			return def
		}
	}

	// Check current version
	if current, ok := c.features[name]; ok && current.Version == version {
		return current
	}

	return nil
}

// GetVersionHistory returns all versions of a feature.
func (c *Catalog) GetVersionHistory(name string) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	history := make([]*FeatureDefinition, 0)
	if versions, ok := c.versions[name]; ok {
		history = append(history, versions...)
	}
	if current, ok := c.features[name]; ok {
		history = append(history, current)
	}
	return history
}

// List returns all features with optional filtering.
func (c *Catalog) List(filter *ListFilter) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*FeatureDefinition

	for _, def := range c.features {
		if filter == nil || filter.Matches(def) {
			result = append(result, def)
		}
	}

	// Sort by name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// ListFilter defines filtering criteria for listing features.
type ListFilter struct {
	Tags       []string      `json:"tags,omitempty"`
	Owner      string        `json:"owner,omitempty"`
	Team       string        `json:"team,omitempty"`
	Category   string        `json:"category,omitempty"`
	EntityType string        `json:"entity_type,omitempty"`
	Status     FeatureStatus `json:"status,omitempty"`
	Search     string        `json:"search,omitempty"` // Text search in name/description
}

// Matches checks if a feature definition matches the filter.
func (f *ListFilter) Matches(def *FeatureDefinition) bool {
	if f.Owner != "" && def.Owner != f.Owner {
		return false
	}

	if f.Team != "" && def.Team != f.Team {
		return false
	}

	if f.Category != "" && def.Category != f.Category {
		return false
	}

	if f.EntityType != "" && def.EntityType != f.EntityType {
		return false
	}

	if f.Status != "" && def.Status != f.Status {
		return false
	}

	if len(f.Tags) > 0 {
		hasTag := false
		for _, filterTag := range f.Tags {
			for _, defTag := range def.Tags {
				if filterTag == defTag {
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

	if f.Search != "" {
		searchLower := strings.ToLower(f.Search)
		if !strings.Contains(strings.ToLower(def.Name), searchLower) &&
			!strings.Contains(strings.ToLower(def.Description), searchLower) {
			return false
		}
	}

	return true
}

// Search performs a text search across features.
func (c *Catalog) Search(query string, limit int) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	queryLower := strings.ToLower(query)
	type scored struct {
		def   *FeatureDefinition
		score int
	}

	var results []scored

	for _, def := range c.features {
		score := 0
		nameLower := strings.ToLower(def.Name)
		descLower := strings.ToLower(def.Description)

		// Exact name match - highest score
		if nameLower == queryLower {
			score = 100
		} else if strings.HasPrefix(nameLower, queryLower) {
			score = 80
		} else if strings.Contains(nameLower, queryLower) {
			score = 60
		} else if strings.Contains(descLower, queryLower) {
			score = 40
		}

		// Check tags
		for _, tag := range def.Tags {
			if strings.Contains(strings.ToLower(tag), queryLower) {
				score += 20
				break
			}
		}

		if score > 0 {
			results = append(results, scored{def, score})
		}
	}

	// Sort by score descending, then by name ascending for deterministic ordering
	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].def.Name < results[j].def.Name
	})

	if len(results) > limit {
		results = results[:limit]
	}

	defs := make([]*FeatureDefinition, len(results))
	for i, r := range results {
		defs[i] = r.def
	}

	return defs
}

// GetByTag returns all features with a specific tag.
func (c *Catalog) GetByTag(tag string) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := c.byTag[tag]
	result := make([]*FeatureDefinition, 0, len(names))
	for _, name := range names {
		if def, ok := c.features[name]; ok {
			result = append(result, def)
		}
	}
	return result
}

// GetByOwner returns all features owned by a specific person/team.
func (c *Catalog) GetByOwner(owner string) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := c.byOwner[owner]
	result := make([]*FeatureDefinition, 0, len(names))
	for _, name := range names {
		if def, ok := c.features[name]; ok {
			result = append(result, def)
		}
	}
	return result
}

// GetByTeam returns all features owned by a specific team.
func (c *Catalog) GetByTeam(team string) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := c.byTeam[team]
	result := make([]*FeatureDefinition, 0, len(names))
	for _, name := range names {
		if def, ok := c.features[name]; ok {
			result = append(result, def)
		}
	}
	return result
}

// GetByCategory returns all features in a specific category.
func (c *Catalog) GetByCategory(category string) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := c.byCategory[category]
	result := make([]*FeatureDefinition, 0, len(names))
	for _, name := range names {
		if def, ok := c.features[name]; ok {
			result = append(result, def)
		}
	}
	return result
}

// GetByEntityType returns all features for a specific entity type.
func (c *Catalog) GetByEntityType(entityType string) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := c.byEntity[entityType]
	result := make([]*FeatureDefinition, 0, len(names))
	for _, name := range names {
		if def, ok := c.features[name]; ok {
			result = append(result, def)
		}
	}
	return result
}

// SetStatus updates the status of a feature.
func (c *Catalog) SetStatus(name string, status FeatureStatus, updatedBy string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	def, ok := c.features[name]
	if !ok {
		return ErrFeatureNotFound
	}

	def.Status = status
	def.UpdatedAt = time.Now()
	def.UpdatedBy = updatedBy

	if status == StatusDeprecated {
		now := time.Now()
		def.DeprecatedAt = &now
	}

	return nil
}

// Delete removes a feature from the catalog.
func (c *Catalog) Delete(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	def, ok := c.features[name]
	if !ok {
		return ErrFeatureNotFound
	}

	c.removeIndexes(def)
	delete(c.features, name)

	return nil
}

// GetLineage returns the full lineage for a feature.
func (c *Catalog) GetLineage(name string) *FeatureLineage {
	c.mu.RLock()
	defer c.mu.RUnlock()

	def, ok := c.features[name]
	if !ok {
		return nil
	}

	return &FeatureLineage{
		Feature:    def,
		Upstream:   c.getUpstreamRecursive(name, make(map[string]bool)),
		Downstream: c.getDownstreamRecursive(name, make(map[string]bool)),
	}
}

func (c *Catalog) getUpstreamRecursive(name string, visited map[string]bool) []*FeatureDefinition {
	if visited[name] {
		return nil
	}
	visited[name] = true

	def, ok := c.features[name]
	if !ok {
		return nil
	}

	var result []*FeatureDefinition
	for _, upstream := range def.Upstream {
		if upDef, ok := c.features[upstream]; ok {
			result = append(result, upDef)
			result = append(result, c.getUpstreamRecursive(upstream, visited)...)
		}
	}
	return result
}

func (c *Catalog) getDownstreamRecursive(name string, visited map[string]bool) []*FeatureDefinition {
	if visited[name] {
		return nil
	}
	visited[name] = true

	def, ok := c.features[name]
	if !ok {
		return nil
	}

	var result []*FeatureDefinition
	for _, downstream := range def.Downstream {
		if downDef, ok := c.features[downstream]; ok {
			result = append(result, downDef)
			result = append(result, c.getDownstreamRecursive(downstream, visited)...)
		}
	}
	return result
}

// FeatureLineage represents the complete lineage of a feature.
type FeatureLineage struct {
	Feature    *FeatureDefinition   `json:"feature"`
	Upstream   []*FeatureDefinition `json:"upstream"`
	Downstream []*FeatureDefinition `json:"downstream"`
}

// GetStats returns catalog statistics.
func (c *Catalog) GetStats() *CatalogStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &CatalogStats{
		TotalFeatures: len(c.features),
		ByStatus:      make(map[FeatureStatus]int),
		ByCategory:    make(map[string]int),
		ByEntityType:  make(map[string]int),
		ByTeam:        make(map[string]int),
		TagCounts:     make(map[string]int),
	}

	for _, def := range c.features {
		stats.ByStatus[def.Status]++
		if def.Category != "" {
			stats.ByCategory[def.Category]++
		}
		if def.EntityType != "" {
			stats.ByEntityType[def.EntityType]++
		}
		if def.Team != "" {
			stats.ByTeam[def.Team]++
		}
		for _, tag := range def.Tags {
			stats.TagCounts[tag]++
		}
	}

	return stats
}

// CatalogStats contains catalog statistics.
type CatalogStats struct {
	TotalFeatures int                   `json:"total_features"`
	ByStatus      map[FeatureStatus]int `json:"by_status"`
	ByCategory    map[string]int        `json:"by_category"`
	ByEntityType  map[string]int        `json:"by_entity_type"`
	ByTeam        map[string]int        `json:"by_team"`
	TagCounts     map[string]int        `json:"tag_counts"`
}

// Export exports the catalog to JSON.
func (c *Catalog) Export() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	features := make([]*FeatureDefinition, 0, len(c.features))
	for _, def := range c.features {
		features = append(features, def)
	}

	return json.Marshal(features)
}

// Import imports features from JSON.
func (c *Catalog) Import(data []byte, importedBy string) error {
	var features []*FeatureDefinition
	if err := json.Unmarshal(data, &features); err != nil {
		return err
	}

	for _, def := range features {
		if err := c.Register(def, importedBy); err != nil {
			return err
		}
	}

	return nil
}

// SchemaChangeType represents the type of schema change.
type SchemaChangeType string

const (
	// SchemaChangeNone indicates no change.
	SchemaChangeNone SchemaChangeType = "none"
	// SchemaChangeBackwardCompatible indicates backward compatible changes (additive).
	SchemaChangeBackwardCompatible SchemaChangeType = "backward_compatible"
	// SchemaChangeForwardCompatible indicates forward compatible changes.
	SchemaChangeForwardCompatible SchemaChangeType = "forward_compatible"
	// SchemaChangeBreaking indicates breaking changes.
	SchemaChangeBreaking SchemaChangeType = "breaking"
)

// SchemaChange describes a specific change between versions.
type SchemaChange struct {
	Field       string           `json:"field"`
	ChangeType  SchemaChangeType `json:"change_type"`
	OldValue    interface{}      `json:"old_value,omitempty"`
	NewValue    interface{}      `json:"new_value,omitempty"`
	Description string           `json:"description"`
}

// SchemaEvolution represents the evolution between two versions.
type SchemaEvolution struct {
	FeatureName    string           `json:"feature_name"`
	FromVersion    int              `json:"from_version"`
	ToVersion      int              `json:"to_version"`
	OverallType    SchemaChangeType `json:"overall_type"`
	Changes        []SchemaChange   `json:"changes"`
	IsBreaking     bool             `json:"is_breaking"`
	MigrationNotes string           `json:"migration_notes,omitempty"`
}

// CompareVersions compares two versions and returns the schema evolution.
func (c *Catalog) CompareVersions(name string, fromVersion, toVersion int) (*SchemaEvolution, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fromDef := c.getVersionLocked(name, fromVersion)
	toDef := c.getVersionLocked(name, toVersion)

	if fromDef == nil {
		return nil, errors.New("from version not found")
	}
	if toDef == nil {
		return nil, errors.New("to version not found")
	}

	return compareDefinitions(fromDef, toDef), nil
}

func (c *Catalog) getVersionLocked(name string, version int) *FeatureDefinition {
	if version <= 0 {
		return c.features[name]
	}

	history := c.versions[name]
	for _, def := range history {
		if def.Version == version {
			return def
		}
	}

	if current, ok := c.features[name]; ok && current.Version == version {
		return current
	}

	return nil
}

func compareDefinitions(from, to *FeatureDefinition) *SchemaEvolution {
	evolution := &SchemaEvolution{
		FeatureName: to.Name,
		FromVersion: from.Version,
		ToVersion:   to.Version,
		Changes:     make([]SchemaChange, 0),
		OverallType: SchemaChangeNone,
	}

	// Check data type change (breaking)
	if from.DataType != to.DataType {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "data_type",
			ChangeType:  SchemaChangeBreaking,
			OldValue:    from.DataType,
			NewValue:    to.DataType,
			Description: "Data type changed",
		})
		evolution.IsBreaking = true
	}

	// Check entity type change (breaking)
	if from.EntityType != to.EntityType {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "entity_type",
			ChangeType:  SchemaChangeBreaking,
			OldValue:    from.EntityType,
			NewValue:    to.EntityType,
			Description: "Entity type changed",
		})
		evolution.IsBreaking = true
	}

	// Check schema changes if both have schemas
	if from.Schema != nil && to.Schema != nil {
		compareSchemas(from.Schema, to.Schema, evolution)
	} else if from.Schema != nil && to.Schema == nil {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "schema",
			ChangeType:  SchemaChangeBreaking,
			OldValue:    from.Schema,
			NewValue:    nil,
			Description: "Schema removed",
		})
		evolution.IsBreaking = true
	} else if from.Schema == nil && to.Schema != nil {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "schema",
			ChangeType:  SchemaChangeBackwardCompatible,
			OldValue:    nil,
			NewValue:    to.Schema,
			Description: "Schema added",
		})
	}

	// Check freshness changes (backward compatible if relaxed, breaking if stricter)
	if from.Freshness.MaxAge != to.Freshness.MaxAge {
		changeType := SchemaChangeBackwardCompatible
		if to.Freshness.MaxAge < from.Freshness.MaxAge {
			changeType = SchemaChangeBreaking
		}
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "freshness.max_age",
			ChangeType:  changeType,
			OldValue:    from.Freshness.MaxAge.String(),
			NewValue:    to.Freshness.MaxAge.String(),
			Description: "Freshness requirement changed",
		})
		if changeType == SchemaChangeBreaking {
			evolution.IsBreaking = true
		}
	}

	// Determine overall change type
	if evolution.IsBreaking {
		evolution.OverallType = SchemaChangeBreaking
		evolution.MigrationNotes = "This version contains breaking changes. Consumer updates may be required."
	} else if len(evolution.Changes) > 0 {
		evolution.OverallType = SchemaChangeBackwardCompatible
	}

	return evolution
}

func compareSchemas(from, to *FeatureSchema, evolution *SchemaEvolution) {
	// Check type change
	if from.Type != to.Type {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "schema.type",
			ChangeType:  SchemaChangeBreaking,
			OldValue:    from.Type,
			NewValue:    to.Type,
			Description: "Schema type changed",
		})
		evolution.IsBreaking = true
	}

	// Check nullable change (making non-nullable is breaking)
	if from.Nullable && !to.Nullable {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "schema.nullable",
			ChangeType:  SchemaChangeBreaking,
			OldValue:    true,
			NewValue:    false,
			Description: "Field made non-nullable",
		})
		evolution.IsBreaking = true
	} else if !from.Nullable && to.Nullable {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "schema.nullable",
			ChangeType:  SchemaChangeBackwardCompatible,
			OldValue:    false,
			NewValue:    true,
			Description: "Field made nullable",
		})
	}

	// Check dimensions change (for vectors)
	if !equalIntSlices(from.Dimensions, to.Dimensions) {
		evolution.Changes = append(evolution.Changes, SchemaChange{
			Field:       "schema.dimensions",
			ChangeType:  SchemaChangeBreaking,
			OldValue:    from.Dimensions,
			NewValue:    to.Dimensions,
			Description: "Vector dimensions changed",
		})
		evolution.IsBreaking = true
	}
}

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// GetBreakingChanges returns all breaking changes between versions.
func (c *Catalog) GetBreakingChanges(name string) []*SchemaEvolution {
	c.mu.RLock()
	defer c.mu.RUnlock()

	history := c.GetVersionHistory(name)
	if len(history) < 2 {
		return nil
	}

	var breakingChanges []*SchemaEvolution
	for i := 1; i < len(history); i++ {
		evolution := compareDefinitions(history[i-1], history[i])
		if evolution.IsBreaking {
			breakingChanges = append(breakingChanges, evolution)
		}
	}

	return breakingChanges
}

// ValidateEvolution checks if a proposed change would be breaking.
func (c *Catalog) ValidateEvolution(name string, proposed *FeatureDefinition) (*SchemaEvolution, error) {
	c.mu.RLock()
	current := c.features[name]
	c.mu.RUnlock()

	if current == nil {
		return nil, nil // New feature, no evolution to check
	}

	evolution := compareDefinitions(current, proposed)
	return evolution, nil
}

// RecordView records a view event for a feature.
func (c *Catalog) RecordView(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p := c.getOrCreatePopularity(name)
	p.ViewCount++
	p.LastViewed = time.Now()
	p.Score = float64(p.ViewCount) + float64(p.QueryCount)*2
}

// RecordQuery records a query event for a feature.
func (c *Catalog) RecordQuery(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p := c.getOrCreatePopularity(name)
	p.QueryCount++
	p.LastQueried = time.Now()
	p.Score = float64(p.ViewCount) + float64(p.QueryCount)*2
}

func (c *Catalog) getOrCreatePopularity(name string) *FeaturePopularity {
	p, ok := c.popularity[name]
	if !ok {
		p = &FeaturePopularity{FeatureName: name}
		c.popularity[name] = p
	}
	return p
}

// GetPopular returns the most popular features by score.
func (c *Catalog) GetPopular(limit int) []*FeaturePopularity {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make([]*FeaturePopularity, 0, len(c.popularity))
	for _, p := range c.popularity {
		items = append(items, p)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}

// GetRecentlyUpdated returns features sorted by update time.
func (c *Catalog) GetRecentlyUpdated(limit int) []*FeatureDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	features := make([]*FeatureDefinition, 0, len(c.features))
	for _, def := range c.features {
		features = append(features, def)
	}
	sort.Slice(features, func(i, j int) bool {
		return features[i].UpdatedAt.After(features[j].UpdatedAt)
	})
	if limit > 0 && limit < len(features) {
		features = features[:limit]
	}
	return features
}

// DashboardSummary provides a consolidated view for the catalog web UI.
type DashboardSummary struct {
	Stats            *CatalogStats        `json:"stats"`
	RecentlyUpdated  []*FeatureDefinition `json:"recently_updated"`
	Popular          []*FeaturePopularity `json:"popular"`
	DeprecatedCount  int                  `json:"deprecated_count"`
	StaleCount       int                  `json:"stale_count"`
	HealthScore      float64              `json:"health_score"`
	TopOwners        []OwnerSummary       `json:"top_owners"`
}

// OwnerSummary summarises feature ownership.
type OwnerSummary struct {
	Owner string `json:"owner"`
	Count int    `json:"count"`
}

// GetDashboardSummary returns a consolidated summary for the UI.
func (c *Catalog) GetDashboardSummary() *DashboardSummary {
	stats := c.GetStats()

	deprecatedCount := stats.ByStatus[StatusDeprecated]
	activeCount := stats.ByStatus[StatusActive]
	total := stats.TotalFeatures

	healthScore := 1.0
	if total > 0 {
		healthScore = float64(activeCount) / float64(total)
	}

	// Build top owners
	type ownerCount struct {
		owner string
		count int
	}
	owners := make([]ownerCount, 0, len(stats.ByTeam))
	for team, count := range stats.ByTeam {
		owners = append(owners, ownerCount{team, count})
	}
	sort.Slice(owners, func(i, j int) bool {
		return owners[i].count > owners[j].count
	})
	topOwners := make([]OwnerSummary, 0, 10)
	for i, o := range owners {
		if i >= 10 {
			break
		}
		topOwners = append(topOwners, OwnerSummary{Owner: o.owner, Count: o.count})
	}

	return &DashboardSummary{
		Stats:           stats,
		RecentlyUpdated: c.GetRecentlyUpdated(10),
		Popular:         c.GetPopular(10),
		DeprecatedCount: deprecatedCount,
		HealthScore:     healthScore,
		TopOwners:       topOwners,
	}
}

// Errors
var (
	ErrNameRequired    = errors.New("feature name is required")
	ErrFeatureNotFound = errors.New("feature not found")
)
