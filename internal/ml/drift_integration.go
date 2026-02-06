package ml

import (
	"fmt"
	"sync"
	"time"
)

// ModelDriftAlert represents a drift alert specific to a model.
type ModelDriftAlert struct {
	// AlertID is the unique identifier for this alert
	AlertID string `json:"alert_id"`
	// ModelID is the model experiencing drift
	ModelID string `json:"model_id"`
	// ModelVersion is the version experiencing drift
	ModelVersion string `json:"model_version"`
	// Feature is the feature that drifted
	Feature string `json:"feature"`
	// DriftType describes the kind of drift detected
	DriftType string `json:"drift_type"`
	// Severity indicates the severity level
	Severity string `json:"severity"`
	// Score is the quantified drift score
	Score float64 `json:"score"`
	// Threshold is the threshold that was exceeded
	Threshold float64 `json:"threshold"`
	// Message describes the drift
	Message string `json:"message"`
	// Details contains additional context
	Details map[string]interface{} `json:"details,omitempty"`
	// Timestamp is when the drift was detected
	Timestamp time.Time `json:"timestamp"`
	// Acknowledged indicates if the alert has been reviewed
	Acknowledged bool `json:"acknowledged"`
	// AcknowledgedAt is when the alert was acknowledged
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	// AcknowledgedBy is who acknowledged the alert
	AcknowledgedBy string `json:"acknowledged_by,omitempty"`
}

// ModelDriftMonitor tracks drift alerts for models.
type ModelDriftMonitor struct {
	mu sync.RWMutex

	// alerts stores all alerts by ID
	alerts map[string]*ModelDriftAlert
	// alertsByModel maps modelID:version to alert IDs
	alertsByModel map[string][]string
	// alertsByFeature maps feature name to alert IDs
	alertsByFeature map[string][]string

	// Configuration
	maxAlerts     int
	alertCooldown time.Duration
	lastAlertTime map[string]time.Time

	// Callbacks
	onAlert []func(*ModelDriftAlert)
}

// NewModelDriftMonitor creates a new model drift monitor.
func NewModelDriftMonitor() *ModelDriftMonitor {
	return &ModelDriftMonitor{
		alerts:          make(map[string]*ModelDriftAlert),
		alertsByModel:   make(map[string][]string),
		alertsByFeature: make(map[string][]string),
		maxAlerts:       1000,
		alertCooldown:   5 * time.Minute,
		lastAlertTime:   make(map[string]time.Time),
	}
}

// RecordAlert records a new drift alert.
func (m *ModelDriftMonitor) RecordAlert(alert *ModelDriftAlert) error {
	if alert.AlertID == "" {
		alert.AlertID = fmt.Sprintf("mda_%s_%s_%d", alert.ModelID, alert.Feature, time.Now().UnixNano())
	}
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check cooldown
	cooldownKey := fmt.Sprintf("%s:%s:%s", alert.ModelID, alert.ModelVersion, alert.Feature)
	if lastTime, ok := m.lastAlertTime[cooldownKey]; ok {
		if time.Since(lastTime) < m.alertCooldown {
			return nil // Skip due to cooldown
		}
	}
	m.lastAlertTime[cooldownKey] = time.Now()

	// Store alert
	m.alerts[alert.AlertID] = alert

	// Index by model
	modelKey := fmt.Sprintf("%s:%s", alert.ModelID, alert.ModelVersion)
	m.alertsByModel[modelKey] = append(m.alertsByModel[modelKey], alert.AlertID)

	// Index by feature
	m.alertsByFeature[alert.Feature] = append(m.alertsByFeature[alert.Feature], alert.AlertID)

	// Trim old alerts if over limit
	if len(m.alerts) > m.maxAlerts {
		m.trimOldAlerts()
	}

	// Notify callbacks
	for _, cb := range m.onAlert {
		go cb(alert)
	}

	return nil
}

// RecordValidationResult converts a validation result to drift alerts.
func (m *ModelDriftMonitor) RecordValidationResult(result *ValidationResult) {
	if result.Valid {
		return
	}

	for featureName, fv := range result.Features {
		if fv.Valid {
			continue
		}

		for _, issue := range fv.Issues {
			alert := &ModelDriftAlert{
				ModelID:      result.ModelID,
				ModelVersion: result.ModelVersion,
				Feature:      featureName,
				DriftType:    string(issue.Type),
				Severity:     string(issue.Severity),
				Score:        issue.Score,
				Threshold:    issue.Threshold,
				Message:      issue.Message,
				Timestamp:    result.Timestamp,
				Details: map[string]interface{}{
					"expected": issue.ExpectedValue,
					"actual":   issue.ActualValue,
				},
			}
			_ = m.RecordAlert(alert)
		}
	}
}

// GetAlert retrieves an alert by ID.
func (m *ModelDriftMonitor) GetAlert(alertID string) (*ModelDriftAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return nil, fmt.Errorf("alert not found: %s", alertID)
	}
	return alert, nil
}

// GetAlertsForModel returns all alerts for a model.
func (m *ModelDriftMonitor) GetAlertsForModel(modelID, version string) []*ModelDriftAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", modelID, version)
	alertIDs := m.alertsByModel[key]

	alerts := make([]*ModelDriftAlert, 0, len(alertIDs))
	for _, id := range alertIDs {
		if alert, ok := m.alerts[id]; ok {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// GetAlertsForFeature returns all alerts for a feature.
func (m *ModelDriftMonitor) GetAlertsForFeature(feature string) []*ModelDriftAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alertIDs := m.alertsByFeature[feature]

	alerts := make([]*ModelDriftAlert, 0, len(alertIDs))
	for _, id := range alertIDs {
		if alert, ok := m.alerts[id]; ok {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// GetRecentAlerts returns alerts since a given time.
func (m *ModelDriftMonitor) GetRecentAlerts(since time.Time) []*ModelDriftAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []*ModelDriftAlert
	for _, alert := range m.alerts {
		if alert.Timestamp.After(since) {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// GetUnacknowledgedAlerts returns all unacknowledged alerts.
func (m *ModelDriftMonitor) GetUnacknowledgedAlerts() []*ModelDriftAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []*ModelDriftAlert
	for _, alert := range m.alerts {
		if !alert.Acknowledged {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// AcknowledgeAlert marks an alert as acknowledged.
func (m *ModelDriftMonitor) AcknowledgeAlert(alertID, acknowledgedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	now := time.Now()
	alert.Acknowledged = true
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = acknowledgedBy

	return nil
}

// DeleteAlert removes an alert.
func (m *ModelDriftMonitor) DeleteAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	// Remove from indexes
	modelKey := fmt.Sprintf("%s:%s", alert.ModelID, alert.ModelVersion)
	m.alertsByModel[modelKey] = removeFromSlice(m.alertsByModel[modelKey], alertID)
	m.alertsByFeature[alert.Feature] = removeFromSlice(m.alertsByFeature[alert.Feature], alertID)

	delete(m.alerts, alertID)
	return nil
}

// OnAlert registers a callback for new alerts.
func (m *ModelDriftMonitor) OnAlert(cb func(*ModelDriftAlert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAlert = append(m.onAlert, cb)
}

// Stats returns monitor statistics.
func (m *ModelDriftMonitor) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	unacknowledged := 0
	bySeverity := make(map[string]int)
	byDriftType := make(map[string]int)

	for _, alert := range m.alerts {
		if !alert.Acknowledged {
			unacknowledged++
		}
		bySeverity[alert.Severity]++
		byDriftType[alert.DriftType]++
	}

	return map[string]interface{}{
		"total_alerts":         len(m.alerts),
		"unacknowledged":       unacknowledged,
		"models_affected":      len(m.alertsByModel),
		"features_affected":    len(m.alertsByFeature),
		"alerts_by_severity":   bySeverity,
		"alerts_by_drift_type": byDriftType,
	}
}

func (m *ModelDriftMonitor) trimOldAlerts() {
	// Find oldest alerts to remove
	if len(m.alerts) <= m.maxAlerts {
		return
	}

	toRemove := len(m.alerts) - m.maxAlerts
	oldest := make([]*ModelDriftAlert, 0, len(m.alerts))

	for _, alert := range m.alerts {
		oldest = append(oldest, alert)
	}

	// Sort by timestamp (oldest first)
	for i := 0; i < len(oldest)-1; i++ {
		for j := i + 1; j < len(oldest); j++ {
			if oldest[j].Timestamp.Before(oldest[i].Timestamp) {
				oldest[i], oldest[j] = oldest[j], oldest[i]
			}
		}
	}

	// Remove oldest
	for i := 0; i < toRemove && i < len(oldest); i++ {
		alert := oldest[i]
		modelKey := fmt.Sprintf("%s:%s", alert.ModelID, alert.ModelVersion)
		m.alertsByModel[modelKey] = removeFromSlice(m.alertsByModel[modelKey], alert.AlertID)
		m.alertsByFeature[alert.Feature] = removeFromSlice(m.alertsByFeature[alert.Feature], alert.AlertID)
		delete(m.alerts, alert.AlertID)
	}
}

// ModelServingOrchestrator coordinates model-aware feature serving with validation.
type ModelServingOrchestrator struct {
	registry      *ModelRegistry
	snapshotStore *SnapshotStore
	validator     *ServingValidator
	driftMonitor  *ModelDriftMonitor
}

// NewModelServingOrchestrator creates a new orchestrator.
func NewModelServingOrchestrator(
	registry *ModelRegistry,
	snapshotStore *SnapshotStore,
	validatorConfig ValidatorConfig,
) *ModelServingOrchestrator {
	validator := NewServingValidator(registry, snapshotStore, validatorConfig)
	driftMonitor := NewModelDriftMonitor()

	// Connect validator alerts to drift monitor
	validator.config.AlertCallback = func(result *ValidationResult) {
		driftMonitor.RecordValidationResult(result)
	}

	return &ModelServingOrchestrator{
		registry:      registry,
		snapshotStore: snapshotStore,
		validator:     validator,
		driftMonitor:  driftMonitor,
	}
}

// Registry returns the model registry.
func (o *ModelServingOrchestrator) Registry() *ModelRegistry {
	return o.registry
}

// SnapshotStore returns the snapshot store.
func (o *ModelServingOrchestrator) SnapshotStore() *SnapshotStore {
	return o.snapshotStore
}

// Validator returns the serving validator.
func (o *ModelServingOrchestrator) Validator() *ServingValidator {
	return o.validator
}

// DriftMonitor returns the drift monitor.
func (o *ModelServingOrchestrator) DriftMonitor() *ModelDriftMonitor {
	return o.driftMonitor
}

// Stats returns combined statistics.
func (o *ModelServingOrchestrator) Stats() map[string]interface{} {
	return map[string]interface{}{
		"registry":      o.registry.Stats(),
		"snapshots":     len(o.snapshotStore.ListSnapshots()),
		"validator":     o.validator.Stats(),
		"drift_monitor": o.driftMonitor.Stats(),
	}
}

// DriftDetectorBridge connects the real-time drift detector with model alerts.
// It monitors features used by models and generates alerts when drift is detected.
type DriftDetectorBridge struct {
	mu sync.RWMutex

	registry     *ModelRegistry
	driftMonitor *ModelDriftMonitor

	// featureModels maps feature -> model IDs using that feature
	featureModels map[string]map[string]bool
	// modelFeatures maps model ID -> features used by the model
	modelFeatures map[string]map[string]bool

	// lastDriftCheck tracks when we last generated an alert per feature
	lastDriftCheck map[string]time.Time
	checkCooldown  time.Duration
}

// NewDriftDetectorBridge creates a new drift detector bridge.
func NewDriftDetectorBridge(registry *ModelRegistry, driftMonitor *ModelDriftMonitor) *DriftDetectorBridge {
	bridge := &DriftDetectorBridge{
		registry:       registry,
		driftMonitor:   driftMonitor,
		featureModels:  make(map[string]map[string]bool),
		modelFeatures:  make(map[string]map[string]bool),
		lastDriftCheck: make(map[string]time.Time),
		checkCooldown:  5 * time.Minute,
	}

	// Listen for model registration and version activation
	registry.OnModelRegistered(bridge.onModelRegistered)
	registry.OnVersionActivated(bridge.onVersionActivated)

	return bridge
}

// SetCheckCooldown sets the cooldown between drift checks for the same feature.
func (b *DriftDetectorBridge) SetCheckCooldown(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.checkCooldown = d
}

// RegisterModel registers a model's features for drift monitoring.
func (b *DriftDetectorBridge) RegisterModel(modelID string, features []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Clear old feature mappings for this model
	if oldFeatures, ok := b.modelFeatures[modelID]; ok {
		for feature := range oldFeatures {
			if models, ok := b.featureModels[feature]; ok {
				delete(models, modelID)
			}
		}
	}

	// Set new feature mappings
	b.modelFeatures[modelID] = make(map[string]bool)
	for _, feature := range features {
		b.modelFeatures[modelID][feature] = true

		if b.featureModels[feature] == nil {
			b.featureModels[feature] = make(map[string]bool)
		}
		b.featureModels[feature][modelID] = true
	}
}

// UnregisterModel removes a model from drift monitoring.
func (b *DriftDetectorBridge) UnregisterModel(modelID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if features, ok := b.modelFeatures[modelID]; ok {
		for feature := range features {
			if models, ok := b.featureModels[feature]; ok {
				delete(models, modelID)
			}
		}
	}
	delete(b.modelFeatures, modelID)
}

// OnDriftDetected should be called when drift is detected on a feature.
// It will generate alerts for all models using that feature.
func (b *DriftDetectorBridge) OnDriftDetected(featureName string, driftType string, score float64, threshold float64) {
	b.mu.Lock()
	// Check cooldown
	if lastCheck, ok := b.lastDriftCheck[featureName]; ok {
		if time.Since(lastCheck) < b.checkCooldown {
			b.mu.Unlock()
			return
		}
	}
	b.lastDriftCheck[featureName] = time.Now()

	// Get models using this feature
	models := make(map[string]bool)
	if m, ok := b.featureModels[featureName]; ok {
		for modelID := range m {
			models[modelID] = true
		}
	}
	b.mu.Unlock()

	// Generate alerts for each affected model
	for modelID := range models {
		model, err := b.registry.GetModel(modelID)
		if err != nil {
			continue
		}

		severity := b.scoreToseverity(score)
		alert := &ModelDriftAlert{
			ModelID:      modelID,
			ModelVersion: model.ActiveVersion,
			Feature:      featureName,
			DriftType:    driftType,
			Severity:     severity,
			Score:        score,
			Threshold:    threshold,
			Message:      b.formatMessage(model.Name, featureName, driftType, severity, score),
			Timestamp:    time.Now(),
		}

		_ = b.driftMonitor.RecordAlert(alert)
	}
}

// GetModelsForFeature returns the models that depend on a feature.
func (b *DriftDetectorBridge) GetModelsForFeature(featureName string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if models, ok := b.featureModels[featureName]; ok {
		result := make([]string, 0, len(models))
		for modelID := range models {
			result = append(result, modelID)
		}
		return result
	}
	return nil
}

// GetFeaturesForModel returns the features used by a model.
func (b *DriftDetectorBridge) GetFeaturesForModel(modelID string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if features, ok := b.modelFeatures[modelID]; ok {
		result := make([]string, 0, len(features))
		for feature := range features {
			result = append(result, feature)
		}
		return result
	}
	return nil
}

// Stats returns bridge statistics.
func (b *DriftDetectorBridge) Stats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	featureCount := 0
	for _, models := range b.featureModels {
		if len(models) > 0 {
			featureCount++
		}
	}

	return map[string]interface{}{
		"monitored_models":   len(b.modelFeatures),
		"monitored_features": featureCount,
	}
}

func (b *DriftDetectorBridge) onModelRegistered(model *Model) {
	// Register features from all versions
	for _, version := range model.Versions {
		b.RegisterModel(model.ID, version.Features)
		break // Just register the first version's features for now
	}
}

func (b *DriftDetectorBridge) onVersionActivated(model *Model, version *ModelVersion) {
	b.RegisterModel(model.ID, version.Features)
}

func (b *DriftDetectorBridge) scoreToseverity(score float64) string {
	if score >= 0.3 {
		return "critical"
	} else if score >= 0.2 {
		return "high"
	} else if score >= 0.1 {
		return "medium"
	}
	return "low"
}

func (b *DriftDetectorBridge) formatMessage(modelName, feature, driftType, severity string, score float64) string {
	return fmt.Sprintf(
		"Feature '%s' used by model '%s' has %s drift detected (%s score: %.4f)",
		feature, modelName, severity, driftType, score,
	)
}
