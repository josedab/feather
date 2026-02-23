package feastcompat

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// FeatureServiceConfig configures the feature service manager.
type FeatureServiceConfig struct {
	MaxServices      int           `json:"max_services"`
	CacheTTL         time.Duration `json:"cache_ttl"`
	EnableVersioning bool          `json:"enable_versioning"`
}

// DefaultFeatureServiceConfig returns sensible defaults.
func DefaultFeatureServiceConfig() FeatureServiceConfig {
	return FeatureServiceConfig{
		MaxServices:      100,
		CacheTTL:         5 * time.Minute,
		EnableVersioning: true,
	}
}

// FeatureServiceVersion represents a versioned snapshot of a feature service.
type FeatureServiceVersion struct {
	Version     int               `json:"version"`
	Views       []string          `json:"views"`
	Description string            `json:"description,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// FeatureServiceStats provides statistics about feature service management.
type FeatureServiceStats struct {
	TotalServices  int   `json:"total_services"`
	TotalVersions  int   `json:"total_versions"`
	TotalRollbacks int64 `json:"total_rollbacks"`
	TotalUpdates   int64 `json:"total_updates"`
}

// FeatureServiceManager provides enhanced feature service management with
// versioning, rollback, search, and composition capabilities.
type FeatureServiceManager struct {
	mu       sync.RWMutex
	config   FeatureServiceConfig
	services map[string]*FeatureService
	versions map[string][]FeatureServiceVersion
	stats    FeatureServiceStats
}

// NewFeatureServiceManager creates a new feature service manager.
func NewFeatureServiceManager(cfg FeatureServiceConfig) *FeatureServiceManager {
	if cfg.MaxServices == 0 {
		cfg = DefaultFeatureServiceConfig()
	}
	return &FeatureServiceManager{
		config:   cfg,
		services: make(map[string]*FeatureService),
		versions: make(map[string][]FeatureServiceVersion),
	}
}

// Create creates a new feature service with the given views and description.
func (m *FeatureServiceManager) Create(name string, views []string, desc string) (*FeatureService, error) {
	if name == "" {
		return nil, fmt.Errorf("service name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[name]; exists {
		return nil, fmt.Errorf("feature service %q already exists", name)
	}

	if len(m.services) >= m.config.MaxServices {
		return nil, fmt.Errorf("max services reached (%d)", m.config.MaxServices)
	}

	now := time.Now()
	svc := &FeatureService{
		Name:         name,
		FeatureViews: copyStrings(views),
		Description:  desc,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	m.services[name] = svc

	if m.config.EnableVersioning {
		m.versions[name] = []FeatureServiceVersion{
			{
				Version:     1,
				Views:       copyStrings(views),
				Description: desc,
				CreatedAt:   now,
			},
		}
	}

	m.stats.TotalServices = len(m.services)
	return svc, nil
}

// Get returns a feature service by name.
func (m *FeatureServiceManager) Get(name string) (*FeatureService, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, exists := m.services[name]
	if !exists {
		return nil, fmt.Errorf("feature service %q not found", name)
	}
	return svc, nil
}

// List returns all feature services.
func (m *FeatureServiceManager) List() []FeatureService {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]FeatureService, 0, len(m.services))
	for _, svc := range m.services {
		result = append(result, *svc)
	}
	return result
}

// Update updates a feature service's views, creating a new version if versioning is enabled.
func (m *FeatureServiceManager) Update(name string, views []string) (*FeatureService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return nil, fmt.Errorf("feature service %q not found", name)
	}

	now := time.Now()
	svc.FeatureViews = copyStrings(views)
	svc.UpdatedAt = now

	if m.config.EnableVersioning {
		versions := m.versions[name]
		nextVersion := 1
		if len(versions) > 0 {
			nextVersion = versions[len(versions)-1].Version + 1
		}
		m.versions[name] = append(versions, FeatureServiceVersion{
			Version:     nextVersion,
			Views:       copyStrings(views),
			Description: svc.Description,
			CreatedAt:   now,
			Tags:        copyTags(svc.Tags),
		})
	}

	m.stats.TotalUpdates++
	return svc, nil
}

// Delete removes a feature service and its version history.
func (m *FeatureServiceManager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[name]; !exists {
		return fmt.Errorf("feature service %q not found", name)
	}

	delete(m.services, name)
	delete(m.versions, name)
	m.stats.TotalServices = len(m.services)
	return nil
}

// Rollback reverts a feature service to a previous version.
func (m *FeatureServiceManager) Rollback(name string, version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, exists := m.services[name]
	if !exists {
		return fmt.Errorf("feature service %q not found", name)
	}

	if !m.config.EnableVersioning {
		return fmt.Errorf("versioning is not enabled")
	}

	versions := m.versions[name]
	var target *FeatureServiceVersion
	for i := range versions {
		if versions[i].Version == version {
			target = &versions[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("version %d not found for service %q", version, name)
	}

	now := time.Now()
	svc.FeatureViews = copyStrings(target.Views)
	svc.Description = target.Description
	svc.UpdatedAt = now

	nextVersion := versions[len(versions)-1].Version + 1
	m.versions[name] = append(versions, FeatureServiceVersion{
		Version:     nextVersion,
		Views:       copyStrings(target.Views),
		Description: target.Description,
		CreatedAt:   now,
		Tags:        copyTags(target.Tags),
	})

	m.stats.TotalRollbacks++
	return nil
}

// GetVersion returns a specific version of a feature service.
func (m *FeatureServiceManager) GetVersion(name string, version int) (*FeatureServiceVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, exists := m.versions[name]
	if !exists {
		return nil, fmt.Errorf("feature service %q not found", name)
	}

	for i := range versions {
		if versions[i].Version == version {
			return &versions[i], nil
		}
	}
	return nil, fmt.Errorf("version %d not found for service %q", version, name)
}

// Search finds feature services matching the query in name, description, or tags.
func (m *FeatureServiceManager) Search(query string) []FeatureService {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(query)
	var results []FeatureService
	for _, svc := range m.services {
		if matchesQuery(svc, q) {
			results = append(results, *svc)
		}
	}
	return results
}

// Stats returns feature service management statistics.
func (m *FeatureServiceManager) Stats() FeatureServiceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.TotalServices = len(m.services)
	totalVersions := 0
	for _, v := range m.versions {
		totalVersions += len(v)
	}
	stats.TotalVersions = totalVersions
	return stats
}

func matchesQuery(svc *FeatureService, query string) bool {
	if strings.Contains(strings.ToLower(svc.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(svc.Description), query) {
		return true
	}
	for k, v := range svc.Tags {
		if strings.Contains(strings.ToLower(k), query) ||
			strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

func copyStrings(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}

func copyTags(tags map[string]string) map[string]string {
	if tags == nil {
		return nil
	}
	cp := make(map[string]string, len(tags))
	for k, v := range tags {
		cp[k] = v
	}
	return cp
}
