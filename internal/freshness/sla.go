package freshness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Errors returned by the SLA system.
var (
	ErrSLANotFound        = errors.New("SLA not found")
	ErrSLAAlreadyExists   = errors.New("SLA already exists")
	ErrInvalidSLASpec     = errors.New("invalid SLA specification")
	ErrFeatureNotTracked  = errors.New("feature not tracked")
	ErrWebhookFailed      = errors.New("webhook delivery failed")
)

// SLASeverity represents the severity level of an SLA state.
type SLASeverity string

const (
	SeverityOK       SLASeverity = "ok"
	SeverityWarning  SLASeverity = "warning"
	SeverityCritical SLASeverity = "critical"
	SeverityBreach   SLASeverity = "breach"
)

// AlertChannel represents where alerts are sent.
type AlertChannel string

const (
	AlertChannelWebhook   AlertChannel = "webhook"
	AlertChannelSlack     AlertChannel = "slack"
	AlertChannelPagerDuty AlertChannel = "pagerduty"
	AlertChannelEmail     AlertChannel = "email"
	AlertChannelPrometheus AlertChannel = "prometheus"
)

// RemediationAction represents an auto-remediation action type.
type RemediationAction string

const (
	RemediationNone      RemediationAction = "none"
	RemediationBackfill  RemediationAction = "backfill"
	RemediationRecompute RemediationAction = "recompute"
	RemediationFallback  RemediationAction = "fallback"
	RemediationNotify    RemediationAction = "notify"
)

// SLASpec defines a freshness SLA specification.
type SLASpec struct {
	// ID is the unique SLA identifier.
	ID string `json:"id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Description describes the SLA.
	Description string `json:"description,omitempty"`

	// FeaturePattern is a glob pattern for matching features.
	FeaturePattern string `json:"feature_pattern"`

	// Features is an explicit list of features (alternative to pattern).
	Features []string `json:"features,omitempty"`

	// Thresholds define staleness thresholds.
	Thresholds SLAThresholds `json:"thresholds"`

	// AlertConfig configures alerting behavior.
	AlertConfig SLAAlertConfig `json:"alert_config"`

	// RemediationConfig configures auto-remediation.
	RemediationConfig SLARemediationConfig `json:"remediation_config,omitempty"`

	// Enabled indicates if this SLA is active.
	Enabled bool `json:"enabled"`

	// Priority for evaluation order (higher = more important).
	Priority int `json:"priority"`

	// Tags for organization.
	Tags []string `json:"tags,omitempty"`

	// Owner is the SLA owner.
	Owner string `json:"owner,omitempty"`

	// CreatedAt is when the SLA was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the SLA was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// SLAThresholds defines staleness thresholds.
type SLAThresholds struct {
	// WarningAge triggers a warning alert.
	WarningAge time.Duration `json:"warning_age"`

	// CriticalAge triggers a critical alert.
	CriticalAge time.Duration `json:"critical_age"`

	// BreachAge triggers a breach (SLA violation).
	BreachAge time.Duration `json:"breach_age"`

	// MaxStalePercent is the max percentage of stale serves before alert.
	MaxStalePercent float64 `json:"max_stale_percent,omitempty"`

	// MinFreshnessScore is the minimum acceptable freshness score (0-100).
	MinFreshnessScore float64 `json:"min_freshness_score,omitempty"`
}

// SLAAlertConfig configures alerting behavior.
type SLAAlertConfig struct {
	// Channels to send alerts to.
	Channels []AlertChannelConfig `json:"channels"`

	// CooldownPeriod prevents alert spam.
	CooldownPeriod time.Duration `json:"cooldown_period"`

	// EscalationAfter escalates if not resolved within this time.
	EscalationAfter time.Duration `json:"escalation_after,omitempty"`

	// EscalationChannels for escalated alerts.
	EscalationChannels []AlertChannelConfig `json:"escalation_channels,omitempty"`

	// IncludeMetrics includes freshness metrics in alerts.
	IncludeMetrics bool `json:"include_metrics"`

	// GroupBy groups alerts by this field (e.g., "feature_group").
	GroupBy string `json:"group_by,omitempty"`
}

// AlertChannelConfig configures a single alert channel.
type AlertChannelConfig struct {
	// Type of alert channel.
	Type AlertChannel `json:"type"`

	// URL is the webhook/API endpoint.
	URL string `json:"url,omitempty"`

	// Headers for the request.
	Headers map[string]string `json:"headers,omitempty"`

	// Severities to send to this channel.
	Severities []SLASeverity `json:"severities,omitempty"`

	// Template is the message template.
	Template string `json:"template,omitempty"`
}

// SLARemediationConfig configures auto-remediation.
type SLARemediationConfig struct {
	// Enabled enables auto-remediation.
	Enabled bool `json:"enabled"`

	// Actions to take for each severity level.
	Actions map[SLASeverity]RemediationAction `json:"actions"`

	// BackfillConfig for backfill remediation.
	BackfillConfig *BackfillRemediationConfig `json:"backfill_config,omitempty"`

	// FallbackConfig for fallback remediation.
	FallbackConfig *FallbackRemediationConfig `json:"fallback_config,omitempty"`

	// MaxRetries for remediation attempts.
	MaxRetries int `json:"max_retries"`

	// RetryBackoff between retries.
	RetryBackoff time.Duration `json:"retry_backoff"`
}

// BackfillRemediationConfig configures backfill auto-remediation.
type BackfillRemediationConfig struct {
	// SourcePath is the backfill data source.
	SourcePath string `json:"source_path"`

	// MaxAge is the maximum age of data to backfill.
	MaxAge time.Duration `json:"max_age"`

	// Priority for backfill jobs.
	Priority int `json:"priority"`
}

// FallbackRemediationConfig configures fallback auto-remediation.
type FallbackRemediationConfig struct {
	// FallbackFeature is the feature to use as fallback.
	FallbackFeature string `json:"fallback_feature,omitempty"`

	// DefaultValue is the default value to use.
	DefaultValue interface{} `json:"default_value,omitempty"`

	// UseCachedValue uses the last cached value.
	UseCachedValue bool `json:"use_cached_value"`
}

// SLAState represents the current state of an SLA for a feature.
type SLAState struct {
	// SLAID is the SLA identifier.
	SLAID string `json:"sla_id"`

	// Feature is the feature name.
	Feature string `json:"feature"`

	// Severity is the current severity level.
	Severity SLASeverity `json:"severity"`

	// LastValue is the timestamp of the last feature value.
	LastValue time.Time `json:"last_value"`

	// StaleDuration is how long the feature has been stale.
	StaleDuration time.Duration `json:"stale_duration"`

	// FreshnessScore is the computed freshness score (0-100).
	FreshnessScore float64 `json:"freshness_score"`

	// StaleServePercent is the percentage of stale serves.
	StaleServePercent float64 `json:"stale_serve_percent"`

	// LastChecked is when the state was last evaluated.
	LastChecked time.Time `json:"last_checked"`

	// AlertFired indicates if an alert was fired.
	AlertFired bool `json:"alert_fired"`

	// LastAlertAt is when the last alert was fired.
	LastAlertAt *time.Time `json:"last_alert_at,omitempty"`

	// RemediationTriggered indicates if remediation was triggered.
	RemediationTriggered bool `json:"remediation_triggered"`

	// RemediationStatus is the current remediation status.
	RemediationStatus string `json:"remediation_status,omitempty"`
}

// SLAAlert represents an alert event.
type SLAAlert struct {
	// ID is the unique alert identifier.
	ID string `json:"id"`

	// SLAID is the SLA identifier.
	SLAID string `json:"sla_id"`

	// Feature is the feature name.
	Feature string `json:"feature"`

	// Severity is the alert severity.
	Severity SLASeverity `json:"severity"`

	// Message is the alert message.
	Message string `json:"message"`

	// Metrics contains freshness metrics.
	Metrics *AlertMetrics `json:"metrics,omitempty"`

	// FiredAt is when the alert was fired.
	FiredAt time.Time `json:"fired_at"`

	// ResolvedAt is when the alert was resolved.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// Acknowledged indicates if the alert was acknowledged.
	Acknowledged bool `json:"acknowledged"`

	// AcknowledgedBy is who acknowledged the alert.
	AcknowledgedBy string `json:"acknowledged_by,omitempty"`

	// AcknowledgedAt is when the alert was acknowledged.
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}

// AlertMetrics contains metrics included in alerts.
type AlertMetrics struct {
	StaleDuration     time.Duration `json:"stale_duration"`
	FreshnessScore    float64       `json:"freshness_score"`
	StaleServePercent float64       `json:"stale_serve_percent"`
	AccessRate        float64       `json:"access_rate"`
	LastValue         time.Time     `json:"last_value"`
}

// SLAManager manages feature freshness SLAs.
type SLAManager struct {
	mu sync.RWMutex

	// SLA registry
	slas map[string]*SLASpec

	// Feature to SLA mapping
	featureSLAs map[string][]string

	// Current SLA states
	states map[string]map[string]*SLAState // slaID -> feature -> state

	// Alert history
	alerts []*SLAAlert

	// Active alerts
	activeAlerts map[string]*SLAAlert // alertID -> alert

	// Freshness manager for metrics
	freshnessManager *Manager

	// Configuration
	config SLAManagerConfig

	// Logger
	logger *slog.Logger

	// Metrics
	metrics SLAManagerMetrics

	// Background worker
	stopCh    chan struct{}
	stoppedCh chan struct{}
	started   bool

	// HTTP client for webhooks
	httpClient *http.Client

	// Remediation callback
	remediationCallback RemediationCallback
}

// RemediationCallback is called when auto-remediation is triggered.
type RemediationCallback func(ctx context.Context, feature string, action RemediationAction, config *SLARemediationConfig) error

// SLAManagerConfig configures the SLA manager.
type SLAManagerConfig struct {
	// CheckInterval is how often to check SLA states.
	CheckInterval time.Duration `json:"check_interval"`

	// AlertRetention is how long to retain alert history.
	AlertRetention time.Duration `json:"alert_retention"`

	// MaxAlerts is the maximum number of alerts to retain.
	MaxAlerts int `json:"max_alerts"`

	// DefaultCooldown is the default alert cooldown period.
	DefaultCooldown time.Duration `json:"default_cooldown"`

	// EnableRemediation globally enables/disables remediation.
	EnableRemediation bool `json:"enable_remediation"`

	// WebhookTimeout is the timeout for webhook delivery.
	WebhookTimeout time.Duration `json:"webhook_timeout"`
}

// DefaultSLAManagerConfig returns default configuration.
func DefaultSLAManagerConfig() SLAManagerConfig {
	return SLAManagerConfig{
		CheckInterval:     time.Minute,
		AlertRetention:    24 * time.Hour,
		MaxAlerts:         1000,
		DefaultCooldown:   5 * time.Minute,
		EnableRemediation: true,
		WebhookTimeout:    10 * time.Second,
	}
}

// SLAManagerMetrics tracks SLA manager performance.
type SLAManagerMetrics struct {
	SLAsRegistered      int64   `json:"slas_registered"`
	FeaturesTracked     int64   `json:"features_tracked"`
	ChecksPerformed     int64   `json:"checks_performed"`
	AlertsFired         int64   `json:"alerts_fired"`
	AlertsResolved      int64   `json:"alerts_resolved"`
	RemediationsTriggered int64 `json:"remediations_triggered"`
	RemediationsSucceeded int64 `json:"remediations_succeeded"`
	RemediationsFailed  int64   `json:"remediations_failed"`
	CurrentBreaches     int64   `json:"current_breaches"`
	OverallCompliance   float64 `json:"overall_compliance"`
}

// NewSLAManager creates a new SLA manager.
func NewSLAManager(freshnessManager *Manager, config SLAManagerConfig, logger *slog.Logger) *SLAManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &SLAManager{
		slas:             make(map[string]*SLASpec),
		featureSLAs:      make(map[string][]string),
		states:           make(map[string]map[string]*SLAState),
		alerts:           make([]*SLAAlert, 0),
		activeAlerts:     make(map[string]*SLAAlert),
		freshnessManager: freshnessManager,
		config:           config,
		logger:           logger,
		stopCh:           make(chan struct{}),
		stoppedCh:        make(chan struct{}),
		httpClient: &http.Client{
			Timeout: config.WebhookTimeout,
		},
	}
}

// Start starts the SLA monitoring loop.
func (m *SLAManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = true
	m.mu.Unlock()

	go m.monitorLoop(ctx)

	m.logger.Info("SLA manager started",
		"check_interval", m.config.CheckInterval,
	)

	return nil
}

// Stop stops the SLA manager.
func (m *SLAManager) Stop() error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return nil
	}
	m.started = false
	m.mu.Unlock()

	close(m.stopCh)

	select {
	case <-m.stoppedCh:
	case <-time.After(30 * time.Second):
		m.logger.Warn("force stopping SLA manager after timeout")
	}

	m.logger.Info("SLA manager stopped")
	return nil
}

func (m *SLAManager) monitorLoop(ctx context.Context) {
	defer close(m.stoppedCh)

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAllSLAs(ctx)
		}
	}
}

func (m *SLAManager) checkAllSLAs(ctx context.Context) {
	m.mu.RLock()
	slas := make([]*SLASpec, 0, len(m.slas))
	for _, sla := range m.slas {
		if sla.Enabled {
			slas = append(slas, sla)
		}
	}
	m.mu.RUnlock()

	for _, sla := range slas {
		m.checkSLA(ctx, sla)
	}

	atomic.AddInt64(&m.metrics.ChecksPerformed, 1)
	m.updateComplianceMetrics()
}

func (m *SLAManager) checkSLA(ctx context.Context, sla *SLASpec) {
	features := m.getFeaturesForSLA(sla)

	for _, feature := range features {
		state := m.evaluateFeature(sla, feature)

		m.mu.Lock()
		if m.states[sla.ID] == nil {
			m.states[sla.ID] = make(map[string]*SLAState)
		}
		oldState := m.states[sla.ID][feature]
		m.states[sla.ID][feature] = state
		m.mu.Unlock()

		// Check if severity changed
		if oldState == nil || oldState.Severity != state.Severity {
			m.handleSeverityChange(ctx, sla, feature, oldState, state)
		}
	}
}

func (m *SLAManager) getFeaturesForSLA(sla *SLASpec) []string {
	if len(sla.Features) > 0 {
		return sla.Features
	}

	// Match features by pattern
	if m.freshnessManager == nil {
		return nil
	}

	allMetrics := m.freshnessManager.GetAllMetrics()
	features := make([]string, 0)

	for featureName := range allMetrics {
		if matchPattern(sla.FeaturePattern, featureName) {
			features = append(features, featureName)
		}
	}

	return features
}

func (m *SLAManager) evaluateFeature(sla *SLASpec, feature string) *SLAState {
	state := &SLAState{
		SLAID:       sla.ID,
		Feature:     feature,
		Severity:    SeverityOK,
		LastChecked: time.Now(),
	}

	// Get freshness metrics
	if m.freshnessManager != nil {
		if metrics, ok := m.freshnessManager.GetAccessMetrics(feature); ok {
			state.LastValue = metrics.LastAccess
			state.StaleDuration = time.Since(metrics.LastAccess)

			// Calculate freshness score (0-100)
			maxAge := sla.Thresholds.BreachAge
			if maxAge > 0 {
				freshness := 100 * (1 - float64(state.StaleDuration)/float64(maxAge))
				if freshness < 0 {
					freshness = 0
				}
				state.FreshnessScore = freshness
			}

			// Calculate stale serve percentage
			if metrics.TotalAccesses > 0 {
				state.StaleServePercent = float64(metrics.StaleServes) / float64(metrics.TotalAccesses) * 100
			}
		}
	}

	// Determine severity based on thresholds
	if sla.Thresholds.BreachAge > 0 && state.StaleDuration >= sla.Thresholds.BreachAge {
		state.Severity = SeverityBreach
	} else if sla.Thresholds.CriticalAge > 0 && state.StaleDuration >= sla.Thresholds.CriticalAge {
		state.Severity = SeverityCritical
	} else if sla.Thresholds.WarningAge > 0 && state.StaleDuration >= sla.Thresholds.WarningAge {
		state.Severity = SeverityWarning
	}

	// Check stale serve percentage
	if sla.Thresholds.MaxStalePercent > 0 && state.StaleServePercent > sla.Thresholds.MaxStalePercent {
		if state.Severity < SeverityWarning {
			state.Severity = SeverityWarning
		}
	}

	// Check freshness score
	if sla.Thresholds.MinFreshnessScore > 0 && state.FreshnessScore < sla.Thresholds.MinFreshnessScore {
		if state.Severity < SeverityWarning {
			state.Severity = SeverityWarning
		}
	}

	return state
}

func (m *SLAManager) handleSeverityChange(ctx context.Context, sla *SLASpec, feature string, oldState, newState *SLAState) {
	// Check if we should fire an alert
	if newState.Severity != SeverityOK {
		// Check cooldown
		if oldState != nil && oldState.LastAlertAt != nil {
			if time.Since(*oldState.LastAlertAt) < sla.AlertConfig.CooldownPeriod {
				return // In cooldown
			}
		}

		alert := m.createAlert(sla, feature, newState)
		m.fireAlert(ctx, sla, alert)

		now := time.Now()
		newState.AlertFired = true
		newState.LastAlertAt = &now

		// Trigger remediation if configured
		if m.config.EnableRemediation && sla.RemediationConfig.Enabled {
			go m.triggerRemediation(ctx, sla, feature, newState)
		}
	} else if oldState != nil && oldState.Severity != SeverityOK {
		// Severity improved to OK - resolve alerts
		m.resolveAlerts(sla.ID, feature)
	}
}

func (m *SLAManager) createAlert(sla *SLASpec, feature string, state *SLAState) *SLAAlert {
	alert := &SLAAlert{
		ID:       fmt.Sprintf("%s-%s-%d", sla.ID, feature, time.Now().UnixNano()),
		SLAID:    sla.ID,
		Feature:  feature,
		Severity: state.Severity,
		FiredAt:  time.Now(),
		Message:  fmt.Sprintf("Feature %s freshness SLA violation: %s (stale for %v)", feature, state.Severity, state.StaleDuration),
	}

	if sla.AlertConfig.IncludeMetrics {
		alert.Metrics = &AlertMetrics{
			StaleDuration:     state.StaleDuration,
			FreshnessScore:    state.FreshnessScore,
			StaleServePercent: state.StaleServePercent,
			LastValue:         state.LastValue,
		}
	}

	return alert
}

func (m *SLAManager) fireAlert(ctx context.Context, sla *SLASpec, alert *SLAAlert) {
	m.mu.Lock()
	m.alerts = append(m.alerts, alert)
	m.activeAlerts[alert.ID] = alert
	m.mu.Unlock()

	atomic.AddInt64(&m.metrics.AlertsFired, 1)

	if alert.Severity == SeverityBreach {
		atomic.AddInt64(&m.metrics.CurrentBreaches, 1)
	}

	// Send to all configured channels
	for _, channel := range sla.AlertConfig.Channels {
		if len(channel.Severities) > 0 {
			found := false
			for _, s := range channel.Severities {
				if s == alert.Severity {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		go m.sendAlert(ctx, channel, alert)
	}

	m.logger.Warn("SLA alert fired",
		"sla_id", alert.SLAID,
		"feature", alert.Feature,
		"severity", alert.Severity,
	)
}

func (m *SLAManager) sendAlert(ctx context.Context, channel AlertChannelConfig, alert *SLAAlert) {
	switch channel.Type {
	case AlertChannelWebhook:
		m.sendWebhookAlert(ctx, channel, alert)
	case AlertChannelSlack:
		m.sendSlackAlert(ctx, channel, alert)
	case AlertChannelPagerDuty:
		m.sendPagerDutyAlert(ctx, channel, alert)
	default:
		m.logger.Debug("unsupported alert channel", "type", channel.Type)
	}
}

func (m *SLAManager) sendWebhookAlert(ctx context.Context, channel AlertChannelConfig, alert *SLAAlert) {
	if channel.URL == "" {
		return
	}

	payload, err := json.Marshal(alert)
	if err != nil {
		m.logger.Error("failed to marshal alert", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", channel.URL, strings.NewReader(string(payload)))
	if err != nil {
		m.logger.Error("failed to create webhook request", "error", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range channel.Headers {
		req.Header.Set(k, v)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		m.logger.Error("webhook delivery failed", "error", err, "url", channel.URL)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		m.logger.Error("webhook returned error", "status", resp.StatusCode, "url", channel.URL)
	}
}

func (m *SLAManager) sendSlackAlert(ctx context.Context, channel AlertChannelConfig, alert *SLAAlert) {
	if channel.URL == "" {
		return
	}

	// Format as Slack message
	color := "#36a64f" // green
	switch alert.Severity {
	case SeverityWarning:
		color = "#ffcc00"
	case SeverityCritical:
		color = "#ff9900"
	case SeverityBreach:
		color = "#ff0000"
	}

	payload := map[string]interface{}{
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": fmt.Sprintf("Freshness SLA Alert: %s", alert.Severity),
				"text":  alert.Message,
				"fields": []map[string]interface{}{
					{"title": "Feature", "value": alert.Feature, "short": true},
					{"title": "SLA", "value": alert.SLAID, "short": true},
				},
				"ts": alert.FiredAt.Unix(),
			},
		},
	}

	if alert.Metrics != nil {
		payload["attachments"].([]map[string]interface{})[0]["fields"] = append(
			payload["attachments"].([]map[string]interface{})[0]["fields"].([]map[string]interface{}),
			map[string]interface{}{"title": "Stale Duration", "value": alert.Metrics.StaleDuration.String(), "short": true},
			map[string]interface{}{"title": "Freshness Score", "value": fmt.Sprintf("%.1f%%", alert.Metrics.FreshnessScore), "short": true},
		)
	}

	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", channel.URL, strings.NewReader(string(data)))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	m.httpClient.Do(req)
}

func (m *SLAManager) sendPagerDutyAlert(ctx context.Context, channel AlertChannelConfig, alert *SLAAlert) {
	if channel.URL == "" {
		return
	}

	severity := "info"
	switch alert.Severity {
	case SeverityWarning:
		severity = "warning"
	case SeverityCritical:
		severity = "error"
	case SeverityBreach:
		severity = "critical"
	}

	payload := map[string]interface{}{
		"routing_key":  channel.Headers["routing_key"],
		"event_action": "trigger",
		"payload": map[string]interface{}{
			"summary":  alert.Message,
			"severity": severity,
			"source":   "feather-freshness-sla",
			"custom_details": map[string]interface{}{
				"sla_id":  alert.SLAID,
				"feature": alert.Feature,
			},
		},
	}

	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", channel.URL, strings.NewReader(string(data)))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	m.httpClient.Do(req)
}

func (m *SLAManager) resolveAlerts(slaID, feature string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, alert := range m.activeAlerts {
		if alert.SLAID == slaID && alert.Feature == feature {
			alert.ResolvedAt = &now
			delete(m.activeAlerts, id)
			atomic.AddInt64(&m.metrics.AlertsResolved, 1)

			if alert.Severity == SeverityBreach {
				atomic.AddInt64(&m.metrics.CurrentBreaches, -1)
			}

			m.logger.Info("SLA alert resolved",
				"alert_id", alert.ID,
				"feature", feature,
			)
		}
	}
}

func (m *SLAManager) triggerRemediation(ctx context.Context, sla *SLASpec, feature string, state *SLAState) {
	action, ok := sla.RemediationConfig.Actions[state.Severity]
	if !ok || action == RemediationNone {
		return
	}

	atomic.AddInt64(&m.metrics.RemediationsTriggered, 1)

	m.mu.Lock()
	if slaState, ok := m.states[sla.ID][feature]; ok {
		slaState.RemediationTriggered = true
		slaState.RemediationStatus = "running"
	}
	m.mu.Unlock()

	var err error
	if m.remediationCallback != nil {
		err = m.remediationCallback(ctx, feature, action, &sla.RemediationConfig)
	}

	m.mu.Lock()
	if slaState, ok := m.states[sla.ID][feature]; ok {
		if err != nil {
			slaState.RemediationStatus = fmt.Sprintf("failed: %v", err)
			atomic.AddInt64(&m.metrics.RemediationsFailed, 1)
		} else {
			slaState.RemediationStatus = "completed"
			atomic.AddInt64(&m.metrics.RemediationsSucceeded, 1)
		}
	}
	m.mu.Unlock()

	if err != nil {
		m.logger.Error("remediation failed",
			"feature", feature,
			"action", action,
			"error", err,
		)
	} else {
		m.logger.Info("remediation completed",
			"feature", feature,
			"action", action,
		)
	}
}

func (m *SLAManager) updateComplianceMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalFeatures := 0
	compliantFeatures := 0

	for _, slaStates := range m.states {
		for _, state := range slaStates {
			totalFeatures++
			if state.Severity == SeverityOK {
				compliantFeatures++
			}
		}
	}

	if totalFeatures > 0 {
		m.metrics.OverallCompliance = float64(compliantFeatures) / float64(totalFeatures) * 100
	}

	atomic.StoreInt64(&m.metrics.FeaturesTracked, int64(totalFeatures))
}

// RegisterSLA registers a new SLA.
func (m *SLAManager) RegisterSLA(spec *SLASpec) error {
	if err := m.validateSLASpec(spec); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.slas[spec.ID]; exists {
		return ErrSLAAlreadyExists
	}

	now := time.Now()
	spec.CreatedAt = now
	spec.UpdatedAt = now

	m.slas[spec.ID] = spec
	m.states[spec.ID] = make(map[string]*SLAState)

	// Update feature mapping
	for _, feature := range spec.Features {
		m.featureSLAs[feature] = append(m.featureSLAs[feature], spec.ID)
	}

	atomic.AddInt64(&m.metrics.SLAsRegistered, 1)

	m.logger.Info("SLA registered",
		"sla_id", spec.ID,
		"name", spec.Name,
	)

	return nil
}

func (m *SLAManager) validateSLASpec(spec *SLASpec) error {
	if spec.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidSLASpec)
	}
	if spec.FeaturePattern == "" && len(spec.Features) == 0 {
		return fmt.Errorf("%w: feature_pattern or features is required", ErrInvalidSLASpec)
	}
	if spec.Thresholds.BreachAge <= 0 {
		return fmt.Errorf("%w: breach_age threshold is required", ErrInvalidSLASpec)
	}
	if spec.AlertConfig.CooldownPeriod <= 0 {
		spec.AlertConfig.CooldownPeriod = m.config.DefaultCooldown
	}
	return nil
}

// UpdateSLA updates an existing SLA.
func (m *SLAManager) UpdateSLA(spec *SLASpec) error {
	if err := m.validateSLASpec(spec); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.slas[spec.ID]; !exists {
		return ErrSLANotFound
	}

	spec.UpdatedAt = time.Now()
	m.slas[spec.ID] = spec

	m.logger.Info("SLA updated", "sla_id", spec.ID)
	return nil
}

// DeleteSLA deletes an SLA.
func (m *SLAManager) DeleteSLA(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.slas[id]; !exists {
		return ErrSLANotFound
	}

	delete(m.slas, id)
	delete(m.states, id)

	m.logger.Info("SLA deleted", "sla_id", id)
	return nil
}

// GetSLA returns an SLA by ID.
func (m *SLAManager) GetSLA(id string) (*SLASpec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sla, exists := m.slas[id]
	if !exists {
		return nil, ErrSLANotFound
	}
	return sla, nil
}

// ListSLAs returns all SLAs.
func (m *SLAManager) ListSLAs() []*SLASpec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slas := make([]*SLASpec, 0, len(m.slas))
	for _, sla := range m.slas {
		slas = append(slas, sla)
	}
	return slas
}

// GetSLAState returns the current state for a feature's SLA.
func (m *SLAManager) GetSLAState(slaID, feature string) (*SLAState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if slaStates, ok := m.states[slaID]; ok {
		if state, ok := slaStates[feature]; ok {
			return state, nil
		}
	}
	return nil, ErrFeatureNotTracked
}

// GetAllStates returns all current SLA states.
func (m *SLAManager) GetAllStates() map[string]map[string]*SLAState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[string]*SLAState)
	for slaID, states := range m.states {
		result[slaID] = make(map[string]*SLAState)
		for feature, state := range states {
			result[slaID][feature] = state
		}
	}
	return result
}

// GetAlerts returns alert history.
func (m *SLAManager) GetAlerts(since time.Time) []*SLAAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*SLAAlert, 0)
	for _, alert := range m.alerts {
		if alert.FiredAt.After(since) {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// GetActiveAlerts returns currently active alerts.
func (m *SLAManager) GetActiveAlerts() []*SLAAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*SLAAlert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		alerts = append(alerts, alert)
	}
	return alerts
}

// AcknowledgeAlert acknowledges an alert.
func (m *SLAManager) AcknowledgeAlert(alertID, acknowledgedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	now := time.Now()
	alert.Acknowledged = true
	alert.AcknowledgedBy = acknowledgedBy
	alert.AcknowledgedAt = &now

	return nil
}

// SetRemediationCallback sets the callback for remediation actions.
func (m *SLAManager) SetRemediationCallback(callback RemediationCallback) {
	m.remediationCallback = callback
}

// Metrics returns SLA manager metrics.
func (m *SLAManager) Metrics() SLAManagerMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return SLAManagerMetrics{
		SLAsRegistered:        atomic.LoadInt64(&m.metrics.SLAsRegistered),
		FeaturesTracked:       atomic.LoadInt64(&m.metrics.FeaturesTracked),
		ChecksPerformed:       atomic.LoadInt64(&m.metrics.ChecksPerformed),
		AlertsFired:           atomic.LoadInt64(&m.metrics.AlertsFired),
		AlertsResolved:        atomic.LoadInt64(&m.metrics.AlertsResolved),
		RemediationsTriggered: atomic.LoadInt64(&m.metrics.RemediationsTriggered),
		RemediationsSucceeded: atomic.LoadInt64(&m.metrics.RemediationsSucceeded),
		RemediationsFailed:    atomic.LoadInt64(&m.metrics.RemediationsFailed),
		CurrentBreaches:       atomic.LoadInt64(&m.metrics.CurrentBreaches),
		OverallCompliance:     m.metrics.OverallCompliance,
	}
}

// Note: matchPattern is defined in policy.go and shared across the package
