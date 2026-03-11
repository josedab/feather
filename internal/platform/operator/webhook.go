package operator

import (
	"fmt"
	"sync"
	"time"
)

// AdmissionAction is the result of a webhook evaluation.
type AdmissionAction string

const (
	AdmissionAllow AdmissionAction = "allow"
	AdmissionDeny  AdmissionAction = "deny"
	AdmissionWarn  AdmissionAction = "warn"
)

// AdmissionRule defines a validation rule for webhook admission.
type AdmissionRule struct {
	Name        string `json:"name"`
	Resource    string `json:"resource"` // "FeatureStore", "FeatureGroup", "FeaturePipeline"
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// AdmissionRequest represents an incoming admission request.
type AdmissionRequest struct {
	Operation string      `json:"operation"` // "CREATE", "UPDATE", "DELETE"
	Resource  string      `json:"resource"`
	Name      string      `json:"name"`
	Namespace string      `json:"namespace"`
	Object    interface{} `json:"object"`
}

// AdmissionResponse is the webhook's response.
type AdmissionResponse struct {
	Allowed  bool            `json:"allowed"`
	Action   AdmissionAction `json:"action"`
	Reason   string          `json:"reason,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
}

// WebhookStats tracks admission webhook statistics.
type WebhookStats struct {
	TotalRequests int64 `json:"total_requests"`
	Allowed       int64 `json:"allowed"`
	Denied        int64 `json:"denied"`
	Warnings      int64 `json:"warnings"`
}

// WebhookController implements a Kubernetes admission webhook.
type WebhookController struct {
	mu    sync.RWMutex
	rules []AdmissionRule
	stats WebhookStats
	audit []auditEntry
}

type auditEntry struct {
	Request  AdmissionRequest  `json:"request"`
	Response AdmissionResponse `json:"response"`
	Time     time.Time         `json:"time"`
}

// NewWebhookController creates a new webhook controller with default rules.
func NewWebhookController() *WebhookController {
	wc := &WebhookController{
		rules: []AdmissionRule{
			{Name: "require-owner", Resource: "FeatureGroup", Description: "Feature groups must have an owner", Enabled: true},
			{Name: "valid-ttl", Resource: "FeatureGroup", Description: "TTL must be positive", Enabled: true},
			{Name: "name-format", Resource: "*", Description: "Names must be lowercase alphanumeric with hyphens", Enabled: true},
			{Name: "max-replicas", Resource: "FeatureStore", Description: "Max replicas cannot exceed 50", Enabled: true},
			{Name: "require-storage", Resource: "FeatureStore", Description: "FeatureStore must specify storage", Enabled: true},
		},
	}
	return wc
}

// AddRule adds a custom admission rule.
func (w *WebhookController) AddRule(rule AdmissionRule) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rules = append(w.rules, rule)
}

// ListRules returns all admission rules.
func (w *WebhookController) ListRules() []AdmissionRule {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]AdmissionRule, len(w.rules))
	copy(result, w.rules)
	return result
}

// Evaluate evaluates an admission request against all rules.
func (w *WebhookController) Evaluate(req AdmissionRequest) AdmissionResponse {
	w.mu.Lock()
	w.stats.TotalRequests++
	w.mu.Unlock()

	resp := AdmissionResponse{
		Allowed: true,
		Action:  AdmissionAllow,
	}

	w.mu.RLock()
	rules := make([]AdmissionRule, len(w.rules))
	copy(rules, w.rules)
	w.mu.RUnlock()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.Resource != "*" && rule.Resource != req.Resource {
			continue
		}

		switch rule.Name {
		case "name-format":
			if req.Name == "" {
				resp.Allowed = false
				resp.Action = AdmissionDeny
				resp.Reason = "name is required"
			}
		case "require-owner":
			if req.Operation == "CREATE" {
				if fg, ok := req.Object.(*FeatureGroup); ok {
					if fg.Spec.Owner == "" {
						resp.Warnings = append(resp.Warnings, "feature group should have an owner")
						resp.Action = AdmissionWarn
					}
				}
			}
		case "max-replicas":
			if fs, ok := req.Object.(*FeatureStore); ok {
				if fs.Spec.Autoscaling != nil && fs.Spec.Autoscaling.MaxReplicas > 50 {
					resp.Allowed = false
					resp.Action = AdmissionDeny
					resp.Reason = fmt.Sprintf("max_replicas %d exceeds limit of 50", fs.Spec.Autoscaling.MaxReplicas)
				}
			}
		}

		if !resp.Allowed {
			break
		}
	}

	w.mu.Lock()
	if resp.Allowed {
		w.stats.Allowed++
	} else {
		w.stats.Denied++
	}
	if len(resp.Warnings) > 0 {
		w.stats.Warnings++
	}
	w.audit = append(w.audit, auditEntry{Request: req, Response: resp, Time: time.Now()})
	if len(w.audit) > 1000 {
		w.audit = w.audit[len(w.audit)-500:]
	}
	w.mu.Unlock()

	return resp
}

// Stats returns webhook statistics.
func (w *WebhookController) Stats() WebhookStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// GetAuditLog returns recent admission audit entries.
func (w *WebhookController) GetAuditLog(limit int) []auditEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if limit <= 0 || limit > len(w.audit) {
		limit = len(w.audit)
	}
	start := len(w.audit) - limit
	result := make([]auditEntry, limit)
	copy(result, w.audit[start:])
	return result
}
