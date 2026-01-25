package feastcompat

import (
	"testing"
)

func TestNewGateway(t *testing.T) {
	g := NewGateway(NewAdapter(DefaultAdapterConfig()))
	if g == nil {
		t.Fatal("expected non-nil gateway")
	}
}

func TestGatewayPush(t *testing.T) {
	g := NewGateway(NewAdapter(DefaultAdapterConfig()))
	resp, err := g.Push(PushRequest{
		PushSourceName: "user_events",
		DfData: []map[string]interface{}{
			{"user_id": "u1", "event": "click"},
			{"user_id": "u2", "event": "purchase"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RowsIngested != 2 {
		t.Errorf("expected 2 rows ingested, got %d", resp.RowsIngested)
	}
	if resp.Destination != "online" {
		t.Errorf("expected online destination, got %s", resp.Destination)
	}
}

func TestGatewayPushMissingSource(t *testing.T) {
	g := NewGateway(NewAdapter(DefaultAdapterConfig()))
	_, err := g.Push(PushRequest{})
	if err == nil {
		t.Fatal("expected error for missing push_source_name")
	}
}

func TestGatewayApply(t *testing.T) {
	g := NewGateway(NewAdapter(DefaultAdapterConfig()))
	resp, err := g.Apply(ApplyRequest{
		Entities: []EntityDef{
			{Name: "user", ValueType: "STRING", JoinKey: "user_id"},
		},
		FeatureViews: []FeatureViewDef{
			{
				Name:     "user_features",
				Entities: []string{"user"},
				Schema:   []FieldDef{{Name: "age", DType: "INT64"}, {Name: "name", DType: "STRING"}},
				Online:   true,
			},
		},
		FeatureServices: []FeatureServiceDef{
			{Name: "user_svc", FeatureViews: []string{"user_features"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if len(resp.EntitiesApplied) != 1 {
		t.Errorf("expected 1 entity applied, got %d", len(resp.EntitiesApplied))
	}
	if len(resp.FeatureViewsApplied) != 1 {
		t.Errorf("expected 1 feature view applied, got %d", len(resp.FeatureViewsApplied))
	}
	if len(resp.ServicesApplied) != 1 {
		t.Errorf("expected 1 service applied, got %d", len(resp.ServicesApplied))
	}

	// Verify feature service was created
	svc, err := g.GetFeatureService("user_svc")
	if err != nil {
		t.Fatal(err)
	}
	if svc.Name != "user_svc" {
		t.Errorf("expected user_svc, got %s", svc.Name)
	}
}

func TestGatewayFeatureServices(t *testing.T) {
	g := NewGateway(NewAdapter(DefaultAdapterConfig()))
	_, _ = g.Apply(ApplyRequest{
		FeatureServices: []FeatureServiceDef{
			{Name: "svc1", FeatureViews: []string{"view1"}},
			{Name: "svc2", FeatureViews: []string{"view2"}},
		},
	})

	services := g.ListFeatureServices()
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	_, err := g.GetFeatureService("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent service")
	}
}

func TestGatewaySavedDatasets(t *testing.T) {
	g := NewGateway(NewAdapter(DefaultAdapterConfig()))
	ds, err := g.SaveDataset(SavedDataset{
		Name:           "training_v1",
		FeatureService: "user_svc",
		EntityDf: []map[string]interface{}{
			{"user_id": "u1", "timestamp": "2024-01-01"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ds.RowCount != 1 {
		t.Errorf("expected row count 1, got %d", ds.RowCount)
	}

	datasets := g.ListSavedDatasets()
	if len(datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(datasets))
	}

	fetched, err := g.GetSavedDataset("training_v1")
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Name != "training_v1" {
		t.Errorf("expected training_v1, got %s", fetched.Name)
	}
}

func TestGatewayStats(t *testing.T) {
	g := NewGateway(NewAdapter(DefaultAdapterConfig()))
	_, _ = g.Push(PushRequest{
		PushSourceName: "src1",
		DfData:         []map[string]interface{}{{"a": 1}},
	})

	stats := g.GatewayStats()
	if stats["push"] == nil {
		t.Error("expected push stats")
	}
}
