package pipelinebuilder

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Template is a reusable pipeline blueprint.
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Pipeline    *Pipeline `json:"pipeline"`
	UsageCount  int       `json:"usage_count"`
	Rating      float64   `json:"rating"`
	Author      string    `json:"author"`
	CreatedAt   time.Time `json:"created_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// TemplateStore is a thread-safe store of pipeline templates.
type TemplateStore struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewTemplateStore creates a store pre-loaded with built-in templates.
func NewTemplateStore() *TemplateStore {
	s := &TemplateStore{templates: make(map[string]*Template)}
	s.registerBuiltins()
	return s
}

// Create adds a template to the store.
func (s *TemplateStore) Create(t *Template) error {
	if t.ID == "" {
		return fmt.Errorf("template ID is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.templates[t.ID]; exists {
		return fmt.Errorf("template %q already exists", t.ID)
	}
	s.templates[t.ID] = t
	return nil
}

// Get returns a template by ID.
func (s *TemplateStore) Get(id string) (*Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.templates[id]
	if !ok {
		return nil, fmt.Errorf("template %q not found", id)
	}
	return t, nil
}

// List returns all templates.
func (s *TemplateStore) List() []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Template, 0, len(s.templates))
	for _, t := range s.templates {
		out = append(out, t)
	}
	return out
}

// Search returns templates whose name or description contains the query.
func (s *TemplateStore) Search(query string) []*Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(query)
	var out []*Template
	for _, t := range s.templates {
		if strings.Contains(strings.ToLower(t.Name), q) || strings.Contains(strings.ToLower(t.Description), q) {
			out = append(out, t)
		}
	}
	return out
}

// IncrementUsage bumps the usage counter for a template.
func (s *TemplateStore) IncrementUsage(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.templates[id]; ok {
		t.UsageCount++
	}
}

func (s *TemplateStore) registerBuiltins() {
	now := time.Now()

	// 1. Fraud Detection
	fraud := NewPipeline("Fraud Detection", "Real-time fraud detection feature pipeline")
	fraud.Tags = []string{"fraud", "real-time"}
	src := &PipelineNode{ID: "txn_source", Type: NodeSource, Name: "Transactions", Config: map[string]interface{}{"entity": "transaction"}, Position: Position{X: 0, Y: 0}}
	agg := &PipelineNode{ID: "txn_agg", Type: NodeAggregate, Name: "TxnAggregates", Config: map[string]interface{}{"window": "1h", "functions": []string{"count", "sum", "avg"}}, Inputs: []string{"txn_source"}, Position: Position{X: 200, Y: 0}}
	filt := &PipelineNode{ID: "txn_filter", Type: NodeFilter, Name: "HighValueFilter", Config: map[string]interface{}{"condition": "amount > 1000"}, Inputs: []string{"txn_agg"}, Position: Position{X: 400, Y: 0}}
	sink := &PipelineNode{ID: "txn_sink", Type: NodeSink, Name: "FeatureStore", Inputs: []string{"txn_filter"}, Position: Position{X: 600, Y: 0}}
	fraud.Nodes[src.ID] = src
	fraud.Nodes[agg.ID] = agg
	fraud.Nodes[filt.ID] = filt
	fraud.Nodes[sink.ID] = sink
	s.templates["fraud-detection"] = &Template{ID: "fraud-detection", Name: "Fraud Detection", Description: "Real-time transaction fraud detection features", Category: "fraud", Pipeline: fraud, Rating: 4.8, Author: "feather", CreatedAt: now, Tags: []string{"fraud", "real-time"}}

	// 2. Recommendation Features
	rec := NewPipeline("Recommendation Features", "User-item interaction feature pipeline")
	rec.Tags = []string{"recommendation", "ml"}
	uSrc := &PipelineNode{ID: "user_src", Type: NodeSource, Name: "UserEvents", Position: Position{X: 0, Y: 0}}
	iSrc := &PipelineNode{ID: "item_src", Type: NodeSource, Name: "ItemCatalog", Position: Position{X: 0, Y: 100}}
	join := &PipelineNode{ID: "ui_join", Type: NodeJoin, Name: "UserItemJoin", Inputs: []string{"user_src", "item_src"}, Position: Position{X: 200, Y: 50}}
	trans := &PipelineNode{ID: "ui_transform", Type: NodeTransform, Name: "InteractionFeatures", Inputs: []string{"ui_join"}, Position: Position{X: 400, Y: 50}}
	rec.Nodes[uSrc.ID] = uSrc
	rec.Nodes[iSrc.ID] = iSrc
	rec.Nodes[join.ID] = join
	rec.Nodes[trans.ID] = trans
	s.templates["recommendation-features"] = &Template{ID: "recommendation-features", Name: "Recommendation Features", Description: "User-item interaction features for recommendation systems", Category: "recommendation", Pipeline: rec, Rating: 4.5, Author: "feather", CreatedAt: now, Tags: []string{"recommendation", "ml"}}

	// 3. User Activity
	ua := NewPipeline("User Activity", "User behaviour aggregation pipeline")
	ua.Tags = []string{"user", "activity"}
	eSrc := &PipelineNode{ID: "event_src", Type: NodeSource, Name: "Events", Position: Position{X: 0, Y: 0}}
	eAgg := &PipelineNode{ID: "event_agg", Type: NodeAggregate, Name: "ActivityAggregates", Inputs: []string{"event_src"}, Config: map[string]interface{}{"window": "24h"}, Position: Position{X: 200, Y: 0}}
	eSink := &PipelineNode{ID: "event_sink", Type: NodeSink, Name: "FeatureStore", Inputs: []string{"event_agg"}, Position: Position{X: 400, Y: 0}}
	ua.Nodes[eSrc.ID] = eSrc
	ua.Nodes[eAgg.ID] = eAgg
	ua.Nodes[eSink.ID] = eSink
	s.templates["user-activity"] = &Template{ID: "user-activity", Name: "User Activity", Description: "User behaviour and engagement aggregation features", Category: "user", Pipeline: ua, Rating: 4.3, Author: "feather", CreatedAt: now, Tags: []string{"user", "activity"}}

	// 4. Time Series Features
	ts := NewPipeline("Time Series Features", "Time series feature extraction pipeline")
	ts.Tags = []string{"time-series"}
	tSrc := &PipelineNode{ID: "ts_source", Type: NodeSource, Name: "TimeSeries", Position: Position{X: 0, Y: 0}}
	tTrans := &PipelineNode{ID: "ts_transform", Type: NodeTransform, Name: "WindowFeatures", Inputs: []string{"ts_source"}, Config: map[string]interface{}{"windows": []string{"1h", "24h", "7d"}}, Position: Position{X: 200, Y: 0}}
	tAgg := &PipelineNode{ID: "ts_agg", Type: NodeAggregate, Name: "RollingStats", Inputs: []string{"ts_transform"}, Position: Position{X: 400, Y: 0}}
	ts.Nodes[tSrc.ID] = tSrc
	ts.Nodes[tTrans.ID] = tTrans
	ts.Nodes[tAgg.ID] = tAgg
	s.templates["time-series-features"] = &Template{ID: "time-series-features", Name: "Time Series Features", Description: "Rolling window and lag features for time series data", Category: "time-series", Pipeline: ts, Rating: 4.6, Author: "feather", CreatedAt: now, Tags: []string{"time-series"}}

	// 5. Text Features
	tf := NewPipeline("Text Features", "NLP text feature extraction pipeline")
	tf.Tags = []string{"text", "nlp"}
	txtSrc := &PipelineNode{ID: "text_source", Type: NodeSource, Name: "TextData", Position: Position{X: 0, Y: 0}}
	txtTrans := &PipelineNode{ID: "text_transform", Type: NodeTransform, Name: "Tokenize", Inputs: []string{"text_source"}, Position: Position{X: 200, Y: 0}}
	txtEnc := &PipelineNode{ID: "text_encode", Type: NodeTransform, Name: "Encode", Inputs: []string{"text_transform"}, Config: map[string]interface{}{"method": "tfidf"}, Position: Position{X: 400, Y: 0}}
	tf.Nodes[txtSrc.ID] = txtSrc
	tf.Nodes[txtTrans.ID] = txtTrans
	tf.Nodes[txtEnc.ID] = txtEnc
	s.templates["text-features"] = &Template{ID: "text-features", Name: "Text Features", Description: "NLP text feature extraction with tokenization and encoding", Category: "nlp", Pipeline: tf, Rating: 4.4, Author: "feather", CreatedAt: now, Tags: []string{"text", "nlp"}}
}
