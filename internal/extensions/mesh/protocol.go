package mesh

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ProtocolVersion is the mesh protocol version.
const ProtocolVersion = "1.0.0"

// AccessLevel defines feature sharing permissions.
type AccessLevel string

const (
	AccessNone      AccessLevel = "none"
	AccessRead      AccessLevel = "read"
	AccessReadWrite AccessLevel = "read_write"
	AccessAdmin     AccessLevel = "admin"
)

// Organization represents a participating organization in the mesh.
type Organization struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Endpoint    string            `json:"endpoint"`
	PublicKey   string            `json:"public_key,omitempty"`
	SharedSecret string           `json:"shared_secret,omitempty"`
	Status      string            `json:"status"` // active, suspended, pending
	JoinedAt    time.Time         `json:"joined_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AccessGrant defines permissions for an organization to access specific features.
type AccessGrant struct {
	ID           string      `json:"id"`
	GranterOrg   string      `json:"granter_org"`
	GranteeOrg   string      `json:"grantee_org"`
	Features     []string    `json:"features"`
	Level        AccessLevel `json:"level"`
	ExpiresAt    *time.Time  `json:"expires_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	Revoked      bool        `json:"revoked"`
}

// FeatureTransferRequest represents a cross-org feature request.
type FeatureTransferRequest struct {
	RequestID   string   `json:"request_id"`
	FromOrg     string   `json:"from_org"`
	ToOrg       string   `json:"to_org"`
	Features    []string `json:"features"`
	EntityKeys  []string `json:"entity_keys"`
	Signature   string   `json:"signature"`
	RequestedAt time.Time `json:"requested_at"`
}

// FeatureTransferResponse contains the transferred feature values.
type FeatureTransferResponse struct {
	RequestID string                            `json:"request_id"`
	Features  map[string]map[string]interface{} `json:"features"` // entity -> feature -> value
	Signature string                            `json:"signature"`
	Encrypted bool                              `json:"encrypted"`
	SentAt    time.Time                         `json:"sent_at"`
}

// MeshProtocol manages cross-organization feature sharing.
type MeshProtocol struct {
	mu           sync.RWMutex
	localOrgID   string
	organizations map[string]*Organization
	grants       map[string]*AccessGrant
	transfers    []FeatureTransferRequest
	stats        MeshProtocolStats
}

// MeshProtocolStats tracks protocol usage.
type MeshProtocolStats struct {
	TotalOrganizations    int   `json:"total_organizations"`
	ActiveGrants          int   `json:"active_grants"`
	TotalTransfers        int64 `json:"total_transfers"`
	SuccessfulTransfers   int64 `json:"successful_transfers"`
	DeniedTransfers       int64 `json:"denied_transfers"`
	TotalFeaturesShared   int64 `json:"total_features_shared"`
}

// NewMeshProtocol creates a new mesh protocol instance.
func NewMeshProtocol(localOrgID string) *MeshProtocol {
	return &MeshProtocol{
		localOrgID:    localOrgID,
		organizations: make(map[string]*Organization),
		grants:        make(map[string]*AccessGrant),
	}
}

// RegisterOrganization adds an organization to the mesh.
func (mp *MeshProtocol) RegisterOrganization(org Organization) error {
	if org.ID == "" || org.Name == "" {
		return fmt.Errorf("id and name are required")
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()

	if _, exists := mp.organizations[org.ID]; exists {
		return fmt.Errorf("organization %s already registered", org.ID)
	}

	org.JoinedAt = time.Now()
	if org.Status == "" {
		org.Status = "active"
	}
	mp.organizations[org.ID] = &org
	mp.stats.TotalOrganizations++
	return nil
}

// ListOrganizations returns all registered organizations.
func (mp *MeshProtocol) ListOrganizations() []Organization {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	result := make([]Organization, 0, len(mp.organizations))
	for _, org := range mp.organizations {
		result = append(result, *org)
	}
	return result
}

// GrantAccess creates an access grant for feature sharing.
func (mp *MeshProtocol) GrantAccess(grant AccessGrant) error {
	if grant.GranterOrg == "" || grant.GranteeOrg == "" {
		return fmt.Errorf("granter_org and grantee_org are required")
	}
	if len(grant.Features) == 0 {
		return fmt.Errorf("at least one feature is required")
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()

	if _, ok := mp.organizations[grant.GranteeOrg]; !ok {
		return fmt.Errorf("grantee organization %s not found", grant.GranteeOrg)
	}

	grant.ID = fmt.Sprintf("grant-%s-%s-%d", grant.GranterOrg, grant.GranteeOrg, time.Now().UnixNano())
	grant.CreatedAt = time.Now()
	if grant.Level == "" {
		grant.Level = AccessRead
	}

	mp.grants[grant.ID] = &grant
	mp.stats.ActiveGrants++
	return nil
}

// RevokeAccess revokes an access grant.
func (mp *MeshProtocol) RevokeAccess(grantID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	grant, exists := mp.grants[grantID]
	if !exists {
		return fmt.Errorf("grant %s not found", grantID)
	}
	if grant.Revoked {
		return fmt.Errorf("grant %s already revoked", grantID)
	}
	grant.Revoked = true
	mp.stats.ActiveGrants--
	return nil
}

// ListGrants returns all access grants.
func (mp *MeshProtocol) ListGrants(orgID string) []AccessGrant {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	result := make([]AccessGrant, 0)
	for _, g := range mp.grants {
		if orgID == "" || g.GranterOrg == orgID || g.GranteeOrg == orgID {
			result = append(result, *g)
		}
	}
	return result
}

// CheckAccess verifies if an org has access to specific features.
func (mp *MeshProtocol) CheckAccess(orgID string, features []string) (AccessLevel, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	bestLevel := AccessNone
	for _, grant := range mp.grants {
		if grant.Revoked {
			continue
		}
		if grant.GranteeOrg != orgID {
			continue
		}
		if grant.ExpiresAt != nil && time.Now().After(*grant.ExpiresAt) {
			continue
		}

		// Check if grant covers all requested features
		grantedSet := make(map[string]bool)
		for _, f := range grant.Features {
			grantedSet[f] = true
		}

		allCovered := true
		for _, f := range features {
			if !grantedSet[f] {
				allCovered = false
				break
			}
		}

		if allCovered && accessLevelOrder(grant.Level) > accessLevelOrder(bestLevel) {
			bestLevel = grant.Level
		}
	}

	return bestLevel, nil
}

// RequestFeatures creates a cross-org feature transfer request.
func (mp *MeshProtocol) RequestFeatures(req FeatureTransferRequest) (*FeatureTransferResponse, error) {
	mp.mu.Lock()
	mp.stats.TotalTransfers++
	mp.mu.Unlock()

	// Verify access
	level, err := mp.CheckAccess(req.FromOrg, req.Features)
	if err != nil {
		return nil, err
	}
	if level == AccessNone {
		mp.mu.Lock()
		mp.stats.DeniedTransfers++
		mp.mu.Unlock()
		return nil, fmt.Errorf("access denied: organization %s has no access to requested features", req.FromOrg)
	}

	// Verify request signature
	if req.Signature != "" {
		mp.mu.RLock()
		org, exists := mp.organizations[req.FromOrg]
		mp.mu.RUnlock()

		if exists && org.SharedSecret != "" {
			expectedSig := computeSignature(req.RequestID, org.SharedSecret)
			if !hmac.Equal([]byte(req.Signature), []byte(expectedSig)) {
				mp.mu.Lock()
				mp.stats.DeniedTransfers++
				mp.mu.Unlock()
				return nil, fmt.Errorf("invalid request signature")
			}
		}
	}

	mp.mu.Lock()
	req.RequestedAt = time.Now()
	mp.transfers = append(mp.transfers, req)
	mp.stats.SuccessfulTransfers++
	mp.stats.TotalFeaturesShared += int64(len(req.Features))
	mp.mu.Unlock()

	// Build response (placeholder values)
	resp := &FeatureTransferResponse{
		RequestID: req.RequestID,
		Features:  make(map[string]map[string]interface{}),
		SentAt:    time.Now(),
	}

	// Sign response
	mp.mu.RLock()
	org, exists := mp.organizations[mp.localOrgID]
	mp.mu.RUnlock()
	if exists && org.SharedSecret != "" {
		resp.Signature = computeSignature(req.RequestID+"-response", org.SharedSecret)
		resp.Encrypted = true
	}

	return resp, nil
}

// Stats returns protocol statistics.
func (mp *MeshProtocol) Stats() MeshProtocolStats {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.stats
}

func computeSignature(data, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func accessLevelOrder(level AccessLevel) int {
	switch level {
	case AccessNone:
		return 0
	case AccessRead:
		return 1
	case AccessReadWrite:
		return 2
	case AccessAdmin:
		return 3
	default:
		return 0
	}
}

// AddProtocol integrates the mesh protocol with the mesh manager.
func (m *MeshManager) AddProtocol(protocol *MeshProtocol) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.protocol = protocol
}

// GetProtocol returns the mesh protocol.
func (m *MeshManager) GetProtocol() *MeshProtocol {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.protocol
}
