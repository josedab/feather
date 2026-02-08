// Package lineage provides feature lineage tracking and data governance.
// It tracks feature provenance, dependencies, and compliance metadata.
package lineage

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Tracker manages feature lineage and governance metadata.
type Tracker struct {
	mu          sync.RWMutex
	features    map[string]*FeatureLineage
	sources     map[string]*DataSource
	consumers   map[string]*Consumer
	graph       *DependencyGraph
	piiRegistry map[string]*PIIMetadata
	auditLog    []AuditEvent
	auditLimit  int
}

// FeatureLineage represents the complete lineage of a feature.
type FeatureLineage struct {
	FeatureID   string    `json:"feature_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`

	// Source information
	Sources []SourceRef `json:"sources"`

	// Transformation chain
	Transformations []Transformation `json:"transformations"`

	// Downstream consumers
	Consumers []ConsumerRef `json:"consumers"`

	// Dependencies on other features
	Dependencies []string `json:"dependencies"`

	// Features that depend on this one
	Dependents []string `json:"dependents"`

	// Compliance metadata
	PIILevel      PIILevel           `json:"pii_level"`
	DataClass     DataClassification `json:"data_classification"`
	RetentionDays int                `json:"retention_days"`

	// Quality metrics
	Freshness    time.Duration `json:"freshness_sla"`
	LastComputed time.Time     `json:"last_computed"`

	// Tags and metadata
	Tags     []string          `json:"tags"`
	Metadata map[string]string `json:"metadata"`
}

// DataSource represents an upstream data source.
type DataSource struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        SourceType        `json:"type"`
	Connection  string            `json:"connection"`
	Schema      json.RawMessage   `json:"schema,omitempty"`
	Owner       string            `json:"owner"`
	Description string            `json:"description"`
	PIILevel    PIILevel          `json:"pii_level"`
	CreatedAt   time.Time         `json:"created_at"`
	Metadata    map[string]string `json:"metadata"`
}

// SourceType indicates the type of data source.
type SourceType string

// SourceType constants for data sources.
const (
	SourceTypeKafka     SourceType = "kafka"
	SourceTypeDatabase  SourceType = "database"
	SourceTypeAPI       SourceType = "api"
	SourceTypeFile      SourceType = "file"
	SourceTypeFeature   SourceType = "feature"
	SourceTypeStreaming SourceType = "streaming"
)

// SourceRef references a data source in lineage.
type SourceRef struct {
	SourceID string   `json:"source_id"`
	Fields   []string `json:"fields,omitempty"`
	Query    string   `json:"query,omitempty"`
}

// Transformation represents a transformation step.
type Transformation struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        TransformType   `json:"type"`
	Description string          `json:"description"`
	Code        string          `json:"code,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
	InputFields []string        `json:"input_fields"`
	OutputField string          `json:"output_field"`
	Order       int             `json:"order"`
	CreatedAt   time.Time       `json:"created_at"`
}

// TransformType indicates the type of transformation.
type TransformType string

// TransformType constants for transformations.
const (
	TransformTypeAggregation TransformType = "aggregation"
	TransformTypeFilter      TransformType = "filter"
	TransformTypeJoin        TransformType = "join"
	TransformTypeMap         TransformType = "map"
	TransformTypeWindow      TransformType = "window"
	TransformTypeCustom      TransformType = "custom"
	TransformTypeWASM        TransformType = "wasm"
)

// Consumer represents a downstream consumer of a feature.
type Consumer struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        ConsumerType `json:"type"`
	Owner       string       `json:"owner"`
	Description string       `json:"description"`
	Endpoint    string       `json:"endpoint,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
}

// ConsumerType indicates the type of consumer.
type ConsumerType string

// ConsumerType constants for consumers.
const (
	ConsumerTypeModel     ConsumerType = "model"
	ConsumerTypeService   ConsumerType = "service"
	ConsumerTypeDashboard ConsumerType = "dashboard"
	ConsumerTypeExport    ConsumerType = "export"
	ConsumerTypeFeature   ConsumerType = "feature"
)

// ConsumerRef references a consumer in lineage.
type ConsumerRef struct {
	ConsumerID string `json:"consumer_id"`
	Purpose    string `json:"purpose,omitempty"`
}

// PIILevel indicates the level of personally identifiable information.
type PIILevel int

// PIILevel constants for PII classification.
const (
	PIINone     PIILevel = iota
	PIILow               // Indirect identifiers (zip code, age range)
	PIIMedium            // Direct identifiers (name, email)
	PIIHigh              // Sensitive (SSN, health data)
	PIICritical          // Highly sensitive (biometric, financial)
)

func (p PIILevel) String() string {
	switch p {
	case PIINone:
		return "none"
	case PIILow:
		return "low"
	case PIIMedium:
		return "medium"
	case PIIHigh:
		return "high"
	case PIICritical:
		return "critical"
	default:
		return "unknown"
	}
}

// DataClassification indicates the data classification level.
type DataClassification string

// DataClassification constants for data classification.
const (
	ClassPublic       DataClassification = "public"
	ClassInternal     DataClassification = "internal"
	ClassConfidential DataClassification = "confidential"
	ClassRestricted   DataClassification = "restricted"
)

// PIIMetadata tracks PII-related compliance information.
type PIIMetadata struct {
	FeatureID       string    `json:"feature_id"`
	PIILevel        PIILevel  `json:"pii_level"`
	PIITypes        []string  `json:"pii_types"` // email, phone, ssn, etc.
	LegalBasis      string    `json:"legal_basis"`
	RetentionPolicy string    `json:"retention_policy"`
	DataSubjects    []string  `json:"data_subjects"` // user, customer, employee
	CrossBorder     bool      `json:"cross_border"`
	Encrypted       bool      `json:"encrypted"`
	Anonymized      bool      `json:"anonymized"`
	LastAudit       time.Time `json:"last_audit"`
}

// AuditEvent represents an audit log entry.
type AuditEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Details   string    `json:"details"`
	IPAddress string    `json:"ip_address,omitempty"`
}

// NewTracker creates a new lineage tracker.
func NewTracker() *Tracker {
	return &Tracker{
		features:    make(map[string]*FeatureLineage),
		sources:     make(map[string]*DataSource),
		consumers:   make(map[string]*Consumer),
		graph:       NewDependencyGraph(),
		piiRegistry: make(map[string]*PIIMetadata),
		auditLog:    make([]AuditEvent, 0),
		auditLimit:  10000,
	}
}

// RegisterFeature registers a feature for lineage tracking.
func (t *Tracker) RegisterFeature(lineage *FeatureLineage) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if lineage.FeatureID == "" {
		return fmt.Errorf("feature ID is required")
	}

	now := time.Now()
	if lineage.CreatedAt.IsZero() {
		lineage.CreatedAt = now
	}
	lineage.UpdatedAt = now
	lineage.Version = 1

	// Check if updating existing
	if existing, ok := t.features[lineage.FeatureID]; ok {
		lineage.Version = existing.Version + 1
		lineage.CreatedAt = existing.CreatedAt
	}

	t.features[lineage.FeatureID] = lineage

	// Update dependency graph
	t.graph.AddNode(lineage.FeatureID, NodeTypeFeature)
	for _, dep := range lineage.Dependencies {
		t.graph.AddEdge(dep, lineage.FeatureID, EdgeTypeDependsOn)
	}

	// Update dependents
	for _, dep := range lineage.Dependencies {
		if depFeature, ok := t.features[dep]; ok {
			depFeature.Dependents = appendUnique(depFeature.Dependents, lineage.FeatureID)
		}
	}

	t.logAudit("system", "register_feature", lineage.FeatureID,
		fmt.Sprintf("Registered feature version %d", lineage.Version))

	return nil
}

// RegisterSource registers a data source.
func (t *Tracker) RegisterSource(source *DataSource) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if source.ID == "" {
		return fmt.Errorf("source ID is required")
	}

	if source.CreatedAt.IsZero() {
		source.CreatedAt = time.Now()
	}

	t.sources[source.ID] = source
	t.graph.AddNode(source.ID, NodeTypeSource)

	t.logAudit("system", "register_source", source.ID,
		fmt.Sprintf("Registered source: %s", source.Name))

	return nil
}

// RegisterConsumer registers a feature consumer.
func (t *Tracker) RegisterConsumer(consumer *Consumer) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if consumer.ID == "" {
		return fmt.Errorf("consumer ID is required")
	}

	if consumer.CreatedAt.IsZero() {
		consumer.CreatedAt = time.Now()
	}

	t.consumers[consumer.ID] = consumer
	t.graph.AddNode(consumer.ID, NodeTypeConsumer)

	t.logAudit("system", "register_consumer", consumer.ID,
		fmt.Sprintf("Registered consumer: %s", consumer.Name))

	return nil
}

// LinkSourceToFeature links a data source to a feature.
func (t *Tracker) LinkSourceToFeature(sourceID, featureID string, fields []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	feature, ok := t.features[featureID]
	if !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	if _, ok := t.sources[sourceID]; !ok {
		return fmt.Errorf("source not found: %s", sourceID)
	}

	// Add source reference
	feature.Sources = append(feature.Sources, SourceRef{
		SourceID: sourceID,
		Fields:   fields,
	})
	feature.UpdatedAt = time.Now()

	// Update graph
	t.graph.AddEdge(sourceID, featureID, EdgeTypeSourceOf)

	return nil
}

// LinkFeatureToConsumer links a feature to a consumer.
func (t *Tracker) LinkFeatureToConsumer(featureID, consumerID, purpose string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	feature, ok := t.features[featureID]
	if !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	if _, ok := t.consumers[consumerID]; !ok {
		return fmt.Errorf("consumer not found: %s", consumerID)
	}

	// Add consumer reference
	feature.Consumers = append(feature.Consumers, ConsumerRef{
		ConsumerID: consumerID,
		Purpose:    purpose,
	})
	feature.UpdatedAt = time.Now()

	// Update graph
	t.graph.AddEdge(featureID, consumerID, EdgeTypeConsumedBy)

	return nil
}

// AddTransformation adds a transformation to a feature's lineage.
func (t *Tracker) AddTransformation(featureID string, transform Transformation) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	feature, ok := t.features[featureID]
	if !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	if transform.CreatedAt.IsZero() {
		transform.CreatedAt = time.Now()
	}
	transform.Order = len(feature.Transformations)

	feature.Transformations = append(feature.Transformations, transform)
	feature.UpdatedAt = time.Now()

	return nil
}

// SetPIIMetadata sets PII compliance metadata for a feature.
func (t *Tracker) SetPIIMetadata(featureID string, pii *PIIMetadata) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.features[featureID]; !ok {
		return fmt.Errorf("feature not found: %s", featureID)
	}

	pii.FeatureID = featureID
	t.piiRegistry[featureID] = pii

	// Update feature's PII level
	t.features[featureID].PIILevel = pii.PIILevel
	t.features[featureID].UpdatedAt = time.Now()

	t.logAudit("system", "set_pii_metadata", featureID,
		fmt.Sprintf("Set PII level to %s", pii.PIILevel.String()))

	return nil
}

// GetFeatureLineage returns the lineage for a feature.
func (t *Tracker) GetFeatureLineage(featureID string) (*FeatureLineage, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	lineage, ok := t.features[featureID]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", featureID)
	}

	return lineage, nil
}

// GetAllFeatures returns all registered features.
func (t *Tracker) GetAllFeatures() []*FeatureLineage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*FeatureLineage, 0, len(t.features))
	for _, f := range t.features {
		result = append(result, f)
	}
	return result
}

// GetImpactAnalysis returns all features and consumers affected by changing a feature.
func (t *Tracker) GetImpactAnalysis(featureID string) (*ImpactAnalysis, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	feature, ok := t.features[featureID]
	if !ok {
		return nil, fmt.Errorf("feature not found: %s", featureID)
	}

	analysis := &ImpactAnalysis{
		FeatureID:         featureID,
		AffectedFeatures:  make([]string, 0),
		AffectedConsumers: make([]string, 0),
		PIIImpact:         make([]*PIIMetadata, 0),
	}

	// Find all downstream dependencies (BFS)
	visited := make(map[string]bool)
	queue := []string{featureID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		if f, ok := t.features[current]; ok {
			if current != featureID {
				analysis.AffectedFeatures = append(analysis.AffectedFeatures, current)
			}

			// Add consumers
			for _, c := range f.Consumers {
				if !contains(analysis.AffectedConsumers, c.ConsumerID) {
					analysis.AffectedConsumers = append(analysis.AffectedConsumers, c.ConsumerID)
				}
			}

			// Check PII impact
			if pii, ok := t.piiRegistry[current]; ok && pii.PIILevel > PIINone {
				analysis.PIIImpact = append(analysis.PIIImpact, pii)
			}

			// Add dependents to queue
			queue = append(queue, f.Dependents...)
		}
	}

	analysis.TotalImpact = len(analysis.AffectedFeatures) + len(analysis.AffectedConsumers)
	analysis.RiskLevel = t.calculateRiskLevel(feature, analysis)

	return analysis, nil
}

// ImpactAnalysis represents the impact of changing a feature.
type ImpactAnalysis struct {
	FeatureID         string         `json:"feature_id"`
	AffectedFeatures  []string       `json:"affected_features"`
	AffectedConsumers []string       `json:"affected_consumers"`
	PIIImpact         []*PIIMetadata `json:"pii_impact"`
	TotalImpact       int            `json:"total_impact"`
	RiskLevel         string         `json:"risk_level"`
}

func (t *Tracker) calculateRiskLevel(feature *FeatureLineage, analysis *ImpactAnalysis) string {
	// Calculate risk based on PII, impact count, and data classification
	score := 0

	score += analysis.TotalImpact * 10
	score += len(analysis.PIIImpact) * 20
	score += int(feature.PIILevel) * 15

	switch feature.DataClass {
	case ClassRestricted:
		score += 50
	case ClassConfidential:
		score += 30
	case ClassInternal:
		score += 10
	case ClassPublic:
	}

	switch {
	case score >= 100:
		return "critical"
	case score >= 50:
		return "high"
	case score >= 20:
		return "medium"
	default:
		return "low"
	}
}

// GetPIIFeatures returns all features with PII above a certain level.
func (t *Tracker) GetPIIFeatures(minLevel PIILevel) []*FeatureLineage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*FeatureLineage, 0)
	for _, f := range t.features {
		if f.PIILevel >= minLevel {
			result = append(result, f)
		}
	}
	return result
}

// GetDataSubjectFeatures returns all features related to a data subject.
func (t *Tracker) GetDataSubjectFeatures(subject string) []*FeatureLineage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*FeatureLineage, 0)
	for featureID, pii := range t.piiRegistry {
		for _, s := range pii.DataSubjects {
			if s == subject {
				if f, ok := t.features[featureID]; ok {
					result = append(result, f)
				}
				break
			}
		}
	}
	return result
}

// GetAuditLog returns audit events since a given time.
func (t *Tracker) GetAuditLog(since time.Time) []AuditEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]AuditEvent, 0)
	for _, event := range t.auditLog {
		if event.Timestamp.After(since) {
			result = append(result, event)
		}
	}
	return result
}

func (t *Tracker) logAudit(actor, action, resource, details string) {
	event := AuditEvent{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Actor:     actor,
		Action:    action,
		Resource:  resource,
		Details:   details,
	}

	t.auditLog = append(t.auditLog, event)

	// Keep bounded
	if len(t.auditLog) > t.auditLimit {
		t.auditLog = t.auditLog[len(t.auditLog)-t.auditLimit:]
	}
}

// GetDependencyGraph returns the full dependency graph.
func (t *Tracker) GetDependencyGraph() *DependencyGraph {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.graph
}

// Helper functions
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
