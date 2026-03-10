package feastcompat

import (
	"fmt"
	"sync"
	"time"
)

// FeatureService groups multiple feature views for a specific use case,
// matching Feast's FeatureService concept.
type FeatureService struct {
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	FeatureViews []string          `json:"feature_views"`
	Owner        string            `json:"owner,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// PushRequest represents a Feast-compatible push endpoint request.
type PushRequest struct {
	PushSourceName string                   `json:"push_source_name"`
	DfData         []map[string]interface{} `json:"df"`
	To             string                   `json:"to,omitempty"` // "online", "offline", or "online_and_offline"
}

// PushResponse represents the response from a push operation.
type PushResponse struct {
	Success      bool   `json:"success"`
	RowsIngested int    `json:"rows_ingested"`
	PushSource   string `json:"push_source"`
	Destination  string `json:"destination"`
}

// ApplyRequest represents a Feast-compatible apply request for registering feature definitions.
type ApplyRequest struct {
	FeatureViews    []FeatureViewDef    `json:"feature_views,omitempty"`
	FeatureServices []FeatureServiceDef `json:"feature_services,omitempty"`
	Entities        []EntityDef         `json:"entities,omitempty"`
}

// FeatureViewDef defines a feature view for apply.
type FeatureViewDef struct {
	Name     string            `json:"name"`
	Entities []string          `json:"entities"`
	Schema   []FieldDef        `json:"schema"`
	Source   string            `json:"source,omitempty"`
	TTL      string            `json:"ttl,omitempty"`
	Online   bool              `json:"online"`
	Tags     map[string]string `json:"tags,omitempty"`
}

// FeatureServiceDef defines a feature service for apply.
type FeatureServiceDef struct {
	Name         string   `json:"name"`
	FeatureViews []string `json:"feature_views"`
	Description  string   `json:"description,omitempty"`
	Owner        string   `json:"owner,omitempty"`
}

// EntityDef defines an entity for apply.
type EntityDef struct {
	Name        string `json:"name"`
	ValueType   string `json:"value_type"`
	Description string `json:"description,omitempty"`
	JoinKey     string `json:"join_key,omitempty"`
}

// FieldDef defines a feature field schema.
type FieldDef struct {
	Name  string `json:"name"`
	DType string `json:"dtype"`
}

// ApplyResponse is returned from the apply operation.
type ApplyResponse struct {
	Success             bool     `json:"success"`
	FeatureViewsApplied []string `json:"feature_views_applied,omitempty"`
	ServicesApplied     []string `json:"services_applied,omitempty"`
	EntitiesApplied     []string `json:"entities_applied,omitempty"`
	Message             string   `json:"message"`
}

// SavedDataset represents a point-in-time join snapshot for training.
type SavedDataset struct {
	Name           string                   `json:"name"`
	FeatureService string                   `json:"feature_service"`
	EntityDf       []map[string]interface{} `json:"entity_df,omitempty"`
	RowCount       int                      `json:"row_count"`
	CreatedAt      time.Time                `json:"created_at"`
	Storage        string                   `json:"storage,omitempty"` // storage location or "memory"
}

// Gateway extends the Feast compatibility adapter with full gateway capabilities.
type Gateway struct {
	mu              sync.RWMutex
	adapter         *Adapter
	storeAdapter    *StoreLookupAdapter
	featureServices map[string]*FeatureService
	entities        map[string]*EntityDef
	savedDatasets   map[string]*SavedDataset
	pushStats       PushStats
}

// PushStats tracks push endpoint statistics.
type PushStats struct {
	TotalPushes  int64 `json:"total_pushes"`
	TotalRows    int64 `json:"total_rows"`
	FailedPushes int64 `json:"failed_pushes"`
}

// NewGateway creates a new Feast-compatible gateway.
func NewGateway(adapter *Adapter) *Gateway {
	return &Gateway{
		adapter:         adapter,
		featureServices: make(map[string]*FeatureService),
		entities:        make(map[string]*EntityDef),
		savedDatasets:   make(map[string]*SavedDataset),
	}
}

// SetStoreAdapter configures the gateway to use real storage for reads and writes.
func (g *Gateway) SetStoreAdapter(adapter *StoreLookupAdapter) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.storeAdapter = adapter
	g.adapter.SetLookupFunc(adapter.LookupFunc())
}

// Adapter returns the underlying adapter.
func (g *Gateway) Adapter() *Adapter {
	return g.adapter
}

// Push handles a Feast-compatible push request.
func (g *Gateway) Push(req PushRequest) (*PushResponse, error) {
	if req.PushSourceName == "" {
		return nil, fmt.Errorf("push_source_name is required")
	}
	if len(req.DfData) == 0 {
		return nil, fmt.Errorf("df data is required")
	}

	dest := req.To
	if dest == "" {
		dest = "online"
	}

	g.mu.Lock()
	g.pushStats.TotalPushes++
	g.pushStats.TotalRows += int64(len(req.DfData))
	storeAdapter := g.storeAdapter
	g.mu.Unlock()

	if storeAdapter != nil {
		ingested, err := storeAdapter.PushToStore(req.PushSourceName, req.DfData)
		if err != nil {
			g.mu.Lock()
			g.pushStats.FailedPushes++
			g.mu.Unlock()
			return nil, fmt.Errorf("pushing to store: %w", err)
		}
		return &PushResponse{
			Success:      true,
			RowsIngested: ingested,
			PushSource:   req.PushSourceName,
			Destination:  dest,
		}, nil
	}

	return &PushResponse{
		Success:      true,
		RowsIngested: len(req.DfData),
		PushSource:   req.PushSourceName,
		Destination:  dest,
	}, nil
}

// Apply registers feature definitions in Feast format.
func (g *Gateway) Apply(req ApplyRequest) (*ApplyResponse, error) {
	resp := &ApplyResponse{Success: true}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Apply entities
	for i := range req.Entities {
		e := req.Entities[i]
		if e.Name == "" {
			return nil, fmt.Errorf("entity name is required")
		}
		g.entities[e.Name] = &e
		resp.EntitiesApplied = append(resp.EntitiesApplied, e.Name)
	}

	// Apply feature views (create mappings automatically)
	for _, fv := range req.FeatureViews {
		if fv.Name == "" {
			return nil, fmt.Errorf("feature view name is required")
		}
		mapping := FeatureViewMapping{
			FeastView:      fv.Name,
			FeatherGroup:   fv.Name,
			EntityMapping:  make(map[string]string),
			FeatureMapping: make(map[string]string),
		}
		for _, field := range fv.Schema {
			mapping.FeatureMapping[field.Name] = field.Name
		}
		// Ignore ErrMappingExists for idempotent apply
		_ = g.adapter.RegisterMapping(mapping)
		resp.FeatureViewsApplied = append(resp.FeatureViewsApplied, fv.Name)
	}

	// Apply feature services
	for _, fs := range req.FeatureServices {
		if fs.Name == "" {
			return nil, fmt.Errorf("feature service name is required")
		}
		now := time.Now()
		g.featureServices[fs.Name] = &FeatureService{
			Name:         fs.Name,
			Description:  fs.Description,
			FeatureViews: fs.FeatureViews,
			Owner:        fs.Owner,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		resp.ServicesApplied = append(resp.ServicesApplied, fs.Name)
	}

	resp.Message = "apply completed successfully"
	return resp, nil
}

// GetFeatureService returns a feature service by name.
func (g *Gateway) GetFeatureService(name string) (*FeatureService, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	svc, exists := g.featureServices[name]
	if !exists {
		return nil, fmt.Errorf("feature service %q not found", name)
	}
	return svc, nil
}

// ListFeatureServices returns all registered feature services.
func (g *Gateway) ListFeatureServices() []FeatureService {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]FeatureService, 0, len(g.featureServices))
	for _, svc := range g.featureServices {
		result = append(result, *svc)
	}
	return result
}

// SaveDataset creates a saved dataset snapshot.
func (g *Gateway) SaveDataset(ds SavedDataset) (*SavedDataset, error) {
	if ds.Name == "" {
		return nil, fmt.Errorf("dataset name is required")
	}
	if ds.FeatureService == "" {
		return nil, fmt.Errorf("feature_service is required")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	ds.CreatedAt = time.Now()
	ds.RowCount = len(ds.EntityDf)
	if ds.Storage == "" {
		ds.Storage = "memory"
	}

	g.savedDatasets[ds.Name] = &ds
	return &ds, nil
}

// GetSavedDataset returns a saved dataset by name.
func (g *Gateway) GetSavedDataset(name string) (*SavedDataset, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ds, exists := g.savedDatasets[name]
	if !exists {
		return nil, fmt.Errorf("saved dataset %q not found", name)
	}
	return ds, nil
}

// ListSavedDatasets returns all saved datasets.
func (g *Gateway) ListSavedDatasets() []SavedDataset {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]SavedDataset, 0, len(g.savedDatasets))
	for _, ds := range g.savedDatasets {
		result = append(result, *ds)
	}
	return result
}

// GatewayStats returns combined statistics.
func (g *Gateway) GatewayStats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return map[string]interface{}{
		"adapter":          g.adapter.Stats(),
		"push":             g.pushStats,
		"feature_services": len(g.featureServices),
		"entities":         len(g.entities),
		"saved_datasets":   len(g.savedDatasets),
	}
}

// ListFeatureViewMappings returns all registered feature view mappings.
func (g *Gateway) ListFeatureViewMappings() []FeatureViewMapping {
	return g.adapter.ListMappings()
}

// MaterializeStats holds materialization results.
type MaterializeStats struct {
	ViewsMaterialized int       `json:"views_materialized"`
	RowsWritten       int64     `json:"rows_written"`
	EndDate           time.Time `json:"end_date"`
}

// MaterializeIncremental materializes features up to the given end date.
func (g *Gateway) MaterializeIncremental(endDate time.Time) *MaterializeStats {
	g.mu.RLock()
	storeAdapter := g.storeAdapter
	mappings := g.adapter.ListMappings()
	g.mu.RUnlock()

	stats := &MaterializeStats{
		ViewsMaterialized: len(mappings),
		RowsWritten:       0,
		EndDate:           endDate,
	}

	if storeAdapter != nil {
		for _, m := range mappings {
			features := make([]string, 0, len(m.FeatureMapping))
			for _, featherName := range m.FeatureMapping {
				features = append(features, featherName)
			}
			entityKeys := make([]string, 0, len(m.EntityMapping))
			for _, featherKey := range m.EntityMapping {
				entityKeys = append(entityKeys, featherKey)
			}
			if len(entityKeys) > 0 && len(features) > 0 {
				rows, _ := storeAdapter.MaterializeFromStore(entityKeys, features, endDate)
				stats.RowsWritten += rows
			}
		}
	}

	return stats
}

// GetOnlineFeatures delegates to the adapter for Feast-compatible feature retrieval.
func (g *Gateway) GetOnlineFeatures(req OnlineFeatureRequest) (*OnlineFeatureResponse, error) {
	return g.adapter.GetOnlineFeatures(req)
}
