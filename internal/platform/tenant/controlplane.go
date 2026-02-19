package tenant

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AuthProvider defines the SSO authentication provider type.
type AuthProvider string

const (
	AuthOIDC AuthProvider = "oidc"
	AuthSAML AuthProvider = "saml"
	AuthAPIKey AuthProvider = "api_key"
)

// AuthConfig configures tenant authentication.
type AuthConfig struct {
	Provider     AuthProvider `json:"provider" yaml:"provider"`
	IssuerURL    string       `json:"issuer_url,omitempty" yaml:"issuer_url,omitempty"`
	ClientID     string       `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	ClientSecret string       `json:"-" yaml:"client_secret,omitempty"`
	RedirectURL  string       `json:"redirect_url,omitempty" yaml:"redirect_url,omitempty"`
	MetadataURL  string       `json:"metadata_url,omitempty" yaml:"metadata_url,omitempty"` // SAML
	Scopes       []string     `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// TenantAuthInfo contains authentication details for a tenant session.
type TenantAuthInfo struct {
	TenantID    string       `json:"tenant_id"`
	UserID      string       `json:"user_id"`
	Email       string       `json:"email,omitempty"`
	Roles       []string     `json:"roles"`
	Provider    AuthProvider `json:"provider"`
	Token       string       `json:"-"`
	ExpiresAt   time.Time    `json:"expires_at"`
	IssuedAt    time.Time    `json:"issued_at"`
}

// APIKey represents a tenant API key for programmatic access.
type APIKey struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Name        string    `json:"name"`
	KeyPrefix   string    `json:"key_prefix"` // First 8 chars for identification
	KeyHash     string    `json:"-"`           // Full key hash (never exposed)
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	LastUsedAt  time.Time `json:"last_used_at,omitempty"`
	Revoked     bool      `json:"revoked"`
}

// UsageRecord tracks resource usage for a tenant.
type UsageRecord struct {
	TenantID      string    `json:"tenant_id"`
	Period        string    `json:"period"` // "hourly", "daily", "monthly"
	PeriodStart   time.Time `json:"period_start"`
	PeriodEnd     time.Time `json:"period_end"`
	APIRequests   int64     `json:"api_requests"`
	FeaturesRead  int64     `json:"features_read"`
	FeaturesWrite int64     `json:"features_written"`
	StorageBytes  int64     `json:"storage_bytes"`
	ComputeMs     int64     `json:"compute_ms"`
	CostUSD       float64   `json:"cost_usd"`
}

// ControlPlane manages tenant provisioning, authentication, and metering.
type ControlPlane struct {
	mu           sync.RWMutex
	registry     *TenantRegistry
	authConfigs  map[string]*AuthConfig   // tenantID -> auth config
	apiKeys      map[string]*APIKey       // keyID -> API key
	sessions     map[string]*TenantAuthInfo // sessionToken -> auth info
	usageRecords map[string][]UsageRecord // tenantID -> usage records
	currentUsage map[string]*UsageRecord  // tenantID -> current period
}

// NewControlPlane creates a new multi-tenant control plane.
func NewControlPlane(registry *TenantRegistry) *ControlPlane {
	return &ControlPlane{
		registry:     registry,
		authConfigs:  make(map[string]*AuthConfig),
		apiKeys:      make(map[string]*APIKey),
		sessions:     make(map[string]*TenantAuthInfo),
		usageRecords: make(map[string][]UsageRecord),
		currentUsage: make(map[string]*UsageRecord),
	}
}

// ConfigureAuth sets up authentication for a tenant.
func (cp *ControlPlane) ConfigureAuth(tenantID string, config AuthConfig) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if config.Provider == "" {
		return fmt.Errorf("auth provider is required")
	}

	switch config.Provider {
	case AuthOIDC:
		if config.IssuerURL == "" || config.ClientID == "" {
			return fmt.Errorf("OIDC requires issuer_url and client_id")
		}
	case AuthSAML:
		if config.MetadataURL == "" {
			return fmt.Errorf("SAML requires metadata_url")
		}
	case AuthAPIKey:
		// No additional config needed
	default:
		return fmt.Errorf("unsupported auth provider: %s", config.Provider)
	}

	cp.authConfigs[tenantID] = &config
	return nil
}

// GetAuthConfig returns the auth configuration for a tenant.
func (cp *ControlPlane) GetAuthConfig(tenantID string) (*AuthConfig, error) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	config, exists := cp.authConfigs[tenantID]
	if !exists {
		return nil, fmt.Errorf("no auth config for tenant %s", tenantID)
	}
	return config, nil
}

// CreateAPIKey generates a new API key for a tenant.
func (cp *ControlPlane) CreateAPIKey(tenantID, name string, permissions []string, ttl time.Duration) (*APIKey, string, error) {
	// Generate random key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", fmt.Errorf("generating key: %w", err)
	}
	rawKey := hex.EncodeToString(keyBytes)

	cp.mu.Lock()
	defer cp.mu.Unlock()

	key := &APIKey{
		ID:          fmt.Sprintf("key-%d", time.Now().UnixNano()),
		TenantID:    tenantID,
		Name:        name,
		KeyPrefix:   rawKey[:8],
		KeyHash:     hashAPIKey(rawKey),
		Permissions: permissions,
		CreatedAt:   time.Now(),
	}
	if ttl > 0 {
		key.ExpiresAt = time.Now().Add(ttl)
	}

	cp.apiKeys[key.ID] = key
	return key, rawKey, nil
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// RevokeAPIKey revokes an API key.
func (cp *ControlPlane) RevokeAPIKey(keyID string) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	key, exists := cp.apiKeys[keyID]
	if !exists {
		return fmt.Errorf("API key %s not found", keyID)
	}
	key.Revoked = true
	return nil
}

// ListAPIKeys returns all API keys for a tenant.
func (cp *ControlPlane) ListAPIKeys(tenantID string) []APIKey {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var keys []APIKey
	for _, key := range cp.apiKeys {
		if key.TenantID == tenantID {
			keys = append(keys, *key)
		}
	}
	return keys
}

// RecordUsage records a usage event for a tenant.
func (cp *ControlPlane) RecordUsage(tenantID string, apiReqs, reads, writes int64, storageBytes, computeMs int64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	now := time.Now()
	current, exists := cp.currentUsage[tenantID]
	if !exists || now.Sub(current.PeriodStart) > time.Hour {
		// Roll over to new period
		if current != nil {
			current.PeriodEnd = now
			cp.usageRecords[tenantID] = append(cp.usageRecords[tenantID], *current)
			// Cap history
			if len(cp.usageRecords[tenantID]) > 720 { // ~30 days hourly
				cp.usageRecords[tenantID] = cp.usageRecords[tenantID][1:]
			}
		}
		current = &UsageRecord{
			TenantID:    tenantID,
			Period:      "hourly",
			PeriodStart: now,
		}
		cp.currentUsage[tenantID] = current
	}

	current.APIRequests += apiReqs
	current.FeaturesRead += reads
	current.FeaturesWrite += writes
	current.StorageBytes = storageBytes
	current.ComputeMs += computeMs
}

// GetUsage returns usage records for a tenant.
func (cp *ControlPlane) GetUsage(tenantID string, since time.Time) []UsageRecord {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	var result []UsageRecord
	for _, r := range cp.usageRecords[tenantID] {
		if r.PeriodStart.After(since) {
			result = append(result, r)
		}
	}
	// Include current period
	if current, ok := cp.currentUsage[tenantID]; ok {
		result = append(result, *current)
	}
	return result
}

// GetCostAttribution returns cost attribution for a tenant.
func (cp *ControlPlane) GetCostAttribution(tenantID string, since time.Time) float64 {
	records := cp.GetUsage(tenantID, since)
	var total float64
	for _, r := range records {
		// Simple cost model: $0.001 per API request, $0.0001 per feature op
		total += float64(r.APIRequests) * 0.001
		total += float64(r.FeaturesRead+r.FeaturesWrite) * 0.0001
		total += float64(r.ComputeMs) * 0.000001
	}
	return total
}

// K8sCRDSpec represents a Kubernetes Custom Resource Definition for tenant provisioning.
type K8sCRDSpec struct {
	APIVersion string          `json:"apiVersion" yaml:"apiVersion"`
	Kind       string          `json:"kind" yaml:"kind"`
	Metadata   K8sCRDMetadata  `json:"metadata" yaml:"metadata"`
	Spec       K8sTenantSpec   `json:"spec" yaml:"spec"`
}

// K8sCRDMetadata holds CRD metadata.
type K8sCRDMetadata struct {
	Name      string            `json:"name" yaml:"name"`
	Namespace string            `json:"namespace" yaml:"namespace"`
	Labels    map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// K8sTenantSpec defines a tenant in K8s CRD format.
type K8sTenantSpec struct {
	TenantID    string            `json:"tenantId" yaml:"tenantId"`
	DisplayName string            `json:"displayName" yaml:"displayName"`
	Tier        string            `json:"tier" yaml:"tier"`
	Replicas    int               `json:"replicas" yaml:"replicas"`
	Resources   K8sResources      `json:"resources" yaml:"resources"`
	Auth        K8sAuthSpec       `json:"auth,omitempty" yaml:"auth,omitempty"`
	Scaling     K8sScalingSpec    `json:"scaling,omitempty" yaml:"scaling,omitempty"`
}

// K8sResources defines resource limits.
type K8sResources struct {
	CPU    string `json:"cpu" yaml:"cpu"`
	Memory string `json:"memory" yaml:"memory"`
}

// K8sAuthSpec defines auth config in CRD format.
type K8sAuthSpec struct {
	Provider  string `json:"provider" yaml:"provider"`
	IssuerURL string `json:"issuerUrl,omitempty" yaml:"issuerUrl,omitempty"`
	ClientID  string `json:"clientId,omitempty" yaml:"clientId,omitempty"`
}

// K8sScalingSpec defines auto-scaling parameters.
type K8sScalingSpec struct {
	MinReplicas    int `json:"minReplicas" yaml:"minReplicas"`
	MaxReplicas    int `json:"maxReplicas" yaml:"maxReplicas"`
	TargetCPUPct   int `json:"targetCPUPercent" yaml:"targetCPUPercent"`
	TargetMemPct   int `json:"targetMemoryPercent,omitempty" yaml:"targetMemoryPercent,omitempty"`
}

// GenerateCRD creates a K8s CRD spec for a tenant.
func (cp *ControlPlane) GenerateCRD(tenantID string) (*K8sCRDSpec, error) {
	t, err := cp.registry.GetTenant(tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting tenant: %w", err)
	}

	tier := string(t.Tier)
	cpu := "500m"
	mem := "512Mi"
	replicas := 1

	switch t.Tier {
	case TierStandard:
		cpu = "1000m"
		mem = "1Gi"
		replicas = 2
	case TierPremium:
		cpu = "2000m"
		mem = "4Gi"
		replicas = 3
	case TierEnterprise:
		cpu = "4000m"
		mem = "8Gi"
		replicas = 5
	}

	crd := &K8sCRDSpec{
		APIVersion: "feather.store/v1",
		Kind:       "FeatherTenant",
		Metadata: K8sCRDMetadata{
			Name:      tenantID,
			Namespace: "feather-system",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "feather-operator",
				"feather.store/tier":           tier,
			},
		},
		Spec: K8sTenantSpec{
			TenantID:    tenantID,
			DisplayName: t.Name,
			Tier:        tier,
			Replicas:    replicas,
			Resources:   K8sResources{CPU: cpu, Memory: mem},
			Scaling: K8sScalingSpec{
				MinReplicas:  1,
				MaxReplicas:  replicas * 3,
				TargetCPUPct: 70,
			},
		},
	}

	// Add auth config if configured
	cp.mu.RLock()
	if auth, ok := cp.authConfigs[tenantID]; ok {
		crd.Spec.Auth = K8sAuthSpec{
			Provider:  string(auth.Provider),
			IssuerURL: auth.IssuerURL,
			ClientID:  auth.ClientID,
		}
	}
	cp.mu.RUnlock()

	return crd, nil
}

// RegisterRoutes registers control plane HTTP routes.
func (cp *ControlPlane) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/tenants/{id}/auth", cp.handleConfigureAuth)
	mux.HandleFunc("GET /v1/tenants/{id}/auth", cp.handleGetAuth)
	mux.HandleFunc("POST /v1/tenants/{id}/apikeys", cp.handleCreateAPIKey)
	mux.HandleFunc("GET /v1/tenants/{id}/apikeys", cp.handleListAPIKeys)
	mux.HandleFunc("GET /v1/tenants/{id}/usage", cp.handleGetUsage)
	mux.HandleFunc("GET /v1/tenants/{id}/crd", cp.handleGenerateCRD)
	mux.HandleFunc("GET /v1/tenants/{id}/cost", cp.handleGetCost)
}

func (cp *ControlPlane) handleConfigureAuth(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	var config AuthConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeControlJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := cp.ConfigureAuth(tenantID, config); err != nil {
		writeControlJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeControlJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (cp *ControlPlane) handleGetAuth(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	config, err := cp.GetAuthConfig(tenantID)
	if err != nil {
		writeControlJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeControlJSON(w, http.StatusOK, config)
}

func (cp *ControlPlane) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeControlJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	key, rawKey, err := cp.CreateAPIKey(tenantID, req.Name, req.Permissions, 0)
	if err != nil {
		writeControlJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeControlJSON(w, http.StatusCreated, map[string]interface{}{
		"key":     key,
		"raw_key": rawKey,
	})
}

func (cp *ControlPlane) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	keys := cp.ListAPIKeys(tenantID)
	writeControlJSON(w, http.StatusOK, map[string]interface{}{"keys": keys})
}

func (cp *ControlPlane) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	since := time.Now().Add(-24 * time.Hour)
	records := cp.GetUsage(tenantID, since)
	writeControlJSON(w, http.StatusOK, map[string]interface{}{"usage": records})
}

func (cp *ControlPlane) handleGenerateCRD(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	crd, err := cp.GenerateCRD(tenantID)
	if err != nil {
		writeControlJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeControlJSON(w, http.StatusOK, crd)
}

func (cp *ControlPlane) handleGetCost(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("id")
	since := time.Now().Add(-30 * 24 * time.Hour)
	cost := cp.GetCostAttribution(tenantID, since)
	writeControlJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id": tenantID,
		"cost_usd":  cost,
		"currency":  "USD",
		"period":    "30d",
	})
}

func writeControlJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
