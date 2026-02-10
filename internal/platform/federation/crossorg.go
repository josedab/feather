package federation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// CrossOrgConfig configures cross-organization federation.
type CrossOrgConfig struct {
	OrgID              string  `json:"org_id"`
	TLSRequired        bool    `json:"tls_required"`
	MaxEpsilonBudget   float64 `json:"max_epsilon_budget"`
	AuditEnabled       bool    `json:"audit_enabled"`
	MaxRequestsPerMin  int     `json:"max_requests_per_min"`
	SigningKey          string  `json:"-"` // never serialized
}

// DefaultCrossOrgConfig returns sensible defaults.
func DefaultCrossOrgConfig() CrossOrgConfig {
	return CrossOrgConfig{
		TLSRequired:       true,
		MaxEpsilonBudget:  10.0,
		AuditEnabled:      true,
		MaxRequestsPerMin: 100,
	}
}

// CrossOrgFederation manages secure feature sharing across organizations.
type CrossOrgFederation struct {
	mu          sync.RWMutex
	config      CrossOrgConfig
	orgs        map[string]*Organization
	agreements  map[string]*SharingAgreement
	auditLog    []AuditEntry
	privacyLedger map[string]*CrossOrgBudget
}

// Organization represents a federated organization.
type Organization struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TrustLevel  string    `json:"trust_level"` // "full", "verified", "limited"
	JoinedAt    time.Time `json:"joined_at"`
	LastRequest time.Time `json:"last_request"`
	RequestCount int64    `json:"request_count"`
}

// SharingAgreement defines terms for feature sharing between orgs.
type SharingAgreement struct {
	ID           string    `json:"id"`
	SourceOrg    string    `json:"source_org"`
	TargetOrg    string    `json:"target_org"`
	Features     []string  `json:"features"`
	Privacy      PrivacyPolicy `json:"privacy"`
	ExpiresAt    time.Time `json:"expires_at"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

// PrivacyPolicy defines differential privacy parameters for sharing.
type PrivacyPolicy struct {
	Mechanism   string  `json:"mechanism"` // "laplace", "gaussian"
	Epsilon     float64 `json:"epsilon"`
	Delta       float64 `json:"delta,omitempty"` // only for gaussian
	Sensitivity float64 `json:"sensitivity"`
}

// CrossOrgBudget tracks epsilon consumption per org.
type CrossOrgBudget struct {
	OrgID         string  `json:"org_id"`
	TotalBudget   float64 `json:"total_budget"`
	Consumed      float64 `json:"consumed"`
	Remaining     float64 `json:"remaining"`
	QueryCount    int64   `json:"query_count"`
}

// AuditEntry records a cross-org operation.
type AuditEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Operation   string    `json:"operation"`
	SourceOrg   string    `json:"source_org"`
	TargetOrg   string    `json:"target_org"`
	Features    []string  `json:"features,omitempty"`
	Epsilon     float64   `json:"epsilon,omitempty"`
	Success     bool      `json:"success"`
	Message     string    `json:"message,omitempty"`
	RequestHash string    `json:"request_hash,omitempty"`
}

// CrossOrgRequest represents a feature request from another org.
type CrossOrgRequest struct {
	SourceOrg    string                 `json:"source_org"`
	Features     []string               `json:"features"`
	EntityKeys   []string               `json:"entity_keys"`
	Signature    string                 `json:"signature"`
	Timestamp    time.Time              `json:"timestamp"`
}

// CrossOrgResponse is the response to a cross-org feature request.
type CrossOrgResponse struct {
	Features    map[string]interface{} `json:"features"`
	NoiseAdded  bool                   `json:"noise_added"`
	Epsilon     float64                `json:"epsilon_used"`
	RequestID   string                 `json:"request_id"`
}

// NewCrossOrgFederation creates a new cross-org federation manager.
func NewCrossOrgFederation(cfg CrossOrgConfig) *CrossOrgFederation {
	return &CrossOrgFederation{
		config:        cfg,
		orgs:          make(map[string]*Organization),
		agreements:    make(map[string]*SharingAgreement),
		auditLog:      make([]AuditEntry, 0),
		privacyLedger: make(map[string]*CrossOrgBudget),
	}
}

// RegisterOrg adds an organization to the federation.
func (f *CrossOrgFederation) RegisterOrg(org Organization) error {
	if org.ID == "" || org.Name == "" {
		return fmt.Errorf("org id and name are required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	org.JoinedAt = time.Now()
	if org.TrustLevel == "" {
		org.TrustLevel = "limited"
	}
	f.orgs[org.ID] = &org

	f.privacyLedger[org.ID] = &CrossOrgBudget{
		OrgID:       org.ID,
		TotalBudget: f.config.MaxEpsilonBudget,
		Remaining:   f.config.MaxEpsilonBudget,
	}

	f.appendAudit(AuditEntry{
		Operation: "register_org",
		SourceOrg: org.ID,
		Success:   true,
		Message:   fmt.Sprintf("organization %q registered with trust level %q", org.Name, org.TrustLevel),
	})

	return nil
}

// ListOrgs returns all registered organizations.
func (f *CrossOrgFederation) ListOrgs() []Organization {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]Organization, 0, len(f.orgs))
	for _, org := range f.orgs {
		result = append(result, *org)
	}
	return result
}

// CreateAgreement establishes a feature sharing agreement between orgs.
func (f *CrossOrgFederation) CreateAgreement(agreement SharingAgreement) (*SharingAgreement, error) {
	if agreement.SourceOrg == "" || agreement.TargetOrg == "" {
		return nil, fmt.Errorf("source and target orgs are required")
	}
	if len(agreement.Features) == 0 {
		return nil, fmt.Errorf("at least one feature is required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.orgs[agreement.SourceOrg]; !ok {
		return nil, fmt.Errorf("source org %q not registered", agreement.SourceOrg)
	}
	if _, ok := f.orgs[agreement.TargetOrg]; !ok {
		return nil, fmt.Errorf("target org %q not registered", agreement.TargetOrg)
	}

	if agreement.ID == "" {
		agreement.ID = fmt.Sprintf("agr_%s_%s_%d", agreement.SourceOrg, agreement.TargetOrg, time.Now().UnixNano())
	}
	agreement.Active = true
	agreement.CreatedAt = time.Now()

	if agreement.Privacy.Epsilon <= 0 {
		agreement.Privacy.Epsilon = 1.0
	}
	if agreement.Privacy.Sensitivity <= 0 {
		agreement.Privacy.Sensitivity = 1.0
	}
	if agreement.Privacy.Mechanism == "" {
		agreement.Privacy.Mechanism = "laplace"
	}

	f.agreements[agreement.ID] = &agreement

	f.appendAudit(AuditEntry{
		Operation: "create_agreement",
		SourceOrg: agreement.SourceOrg,
		TargetOrg: agreement.TargetOrg,
		Features:  agreement.Features,
		Success:   true,
	})

	return &agreement, nil
}

// ListAgreements returns all sharing agreements.
func (f *CrossOrgFederation) ListAgreements() []SharingAgreement {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]SharingAgreement, 0, len(f.agreements))
	for _, a := range f.agreements {
		result = append(result, *a)
	}
	return result
}

// ProcessRequest handles a cross-org feature request with privacy protection.
func (f *CrossOrgFederation) ProcessRequest(req CrossOrgRequest, featureValues map[string]interface{}) (*CrossOrgResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Validate source org
	org, ok := f.orgs[req.SourceOrg]
	if !ok {
		f.appendAudit(AuditEntry{
			Operation: "feature_request",
			SourceOrg: req.SourceOrg,
			Success:   false,
			Message:   "unknown organization",
		})
		return nil, fmt.Errorf("unknown organization %q", req.SourceOrg)
	}

	// Verify signature if signing key is set
	if f.config.SigningKey != "" {
		if !f.verifySignature(req) {
			f.appendAudit(AuditEntry{
				Operation: "feature_request",
				SourceOrg: req.SourceOrg,
				Success:   false,
				Message:   "invalid signature",
			})
			return nil, fmt.Errorf("invalid request signature")
		}
	}

	// Find applicable agreement
	var agreement *SharingAgreement
	for _, a := range f.agreements {
		if a.Active && a.TargetOrg == req.SourceOrg {
			agreement = a
			break
		}
	}
	if agreement == nil {
		return nil, fmt.Errorf("no active sharing agreement for org %q", req.SourceOrg)
	}

	// Check privacy budget
	budget := f.privacyLedger[req.SourceOrg]
	if budget.Remaining < agreement.Privacy.Epsilon {
		return nil, fmt.Errorf("privacy budget exhausted for org %q (remaining: %.2f, required: %.2f)",
			req.SourceOrg, budget.Remaining, agreement.Privacy.Epsilon)
	}

	// Apply differential privacy noise
	noisyFeatures := make(map[string]interface{}, len(featureValues))
	for k, v := range featureValues {
		noisyFeatures[k] = addNoise(v, agreement.Privacy)
	}

	// Consume privacy budget
	budget.Consumed += agreement.Privacy.Epsilon
	budget.Remaining -= agreement.Privacy.Epsilon
	budget.QueryCount++

	// Update org stats
	org.LastRequest = time.Now()
	org.RequestCount++

	f.appendAudit(AuditEntry{
		Operation: "feature_request",
		SourceOrg: req.SourceOrg,
		TargetOrg: f.config.OrgID,
		Features:  req.Features,
		Epsilon:   agreement.Privacy.Epsilon,
		Success:   true,
	})

	return &CrossOrgResponse{
		Features:   noisyFeatures,
		NoiseAdded: true,
		Epsilon:    agreement.Privacy.Epsilon,
		RequestID:  fmt.Sprintf("req_%d", time.Now().UnixNano()),
	}, nil
}

// addNoise adds differential privacy noise to a value.
func addNoise(value interface{}, policy PrivacyPolicy) interface{} {
	fVal, ok := toFedFloat(value)
	if !ok {
		return value // non-numeric values returned as-is
	}

	var noise float64
	switch policy.Mechanism {
	case "laplace":
		noise = laplaceNoise(policy.Sensitivity / policy.Epsilon)
	case "gaussian":
		sigma := policy.Sensitivity * math.Sqrt(2*math.Log(1.25/policy.Delta)) / policy.Epsilon
		noise = gaussianNoise(sigma)
	default:
		noise = laplaceNoise(policy.Sensitivity / policy.Epsilon)
	}

	return fVal + noise
}

// laplaceNoise generates noise from a Laplace distribution with scale b.
func laplaceNoise(b float64) float64 {
	u := rand.Float64() - 0.5
	return -b * sign(u) * math.Log(1-2*math.Abs(u))
}

// gaussianNoise generates noise from a Gaussian distribution with std dev sigma.
func gaussianNoise(sigma float64) float64 {
	return rand.NormFloat64() * sigma
}

func sign(x float64) float64 {
	if x < 0 {
		return -1
	}
	return 1
}

func toFedFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

// GetCrossOrgBudget returns the privacy budget for an org.
func (f *CrossOrgFederation) GetCrossOrgBudget(orgID string) (*CrossOrgBudget, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	budget, ok := f.privacyLedger[orgID]
	if !ok {
		return nil, fmt.Errorf("org %q not found", orgID)
	}
	cp := *budget
	return &cp, nil
}

// GetAuditLog returns recent audit entries.
func (f *CrossOrgFederation) GetAuditLog(limit int) []AuditEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if limit <= 0 || limit > len(f.auditLog) {
		limit = len(f.auditLog)
	}
	start := len(f.auditLog) - limit
	result := make([]AuditEntry, limit)
	copy(result, f.auditLog[start:])
	return result
}

func (f *CrossOrgFederation) appendAudit(entry AuditEntry) {
	entry.Timestamp = time.Now()
	f.auditLog = append(f.auditLog, entry)
	if len(f.auditLog) > 10000 {
		f.auditLog = f.auditLog[len(f.auditLog)-10000:]
	}
}

func (f *CrossOrgFederation) verifySignature(req CrossOrgRequest) bool {
	mac := hmac.New(sha256.New, []byte(f.config.SigningKey))
	mac.Write([]byte(req.SourceOrg + "|" + req.Timestamp.Format(time.RFC3339)))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(req.Signature))
}

// CrossOrgStats returns federation statistics.
type CrossOrgStats struct {
	TotalOrgs       int `json:"total_orgs"`
	ActiveAgreements int `json:"active_agreements"`
	TotalAuditEntries int `json:"total_audit_entries"`
	TotalQueries    int64 `json:"total_queries"`
}

// Stats returns federation statistics.
func (f *CrossOrgFederation) Stats() CrossOrgStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	active := 0
	for _, a := range f.agreements {
		if a.Active {
			active++
		}
	}

	var totalQueries int64
	for _, b := range f.privacyLedger {
		totalQueries += b.QueryCount
	}

	return CrossOrgStats{
		TotalOrgs:         len(f.orgs),
		ActiveAgreements:  active,
		TotalAuditEntries: len(f.auditLog),
		TotalQueries:      totalQueries,
	}
}
