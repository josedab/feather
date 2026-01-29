package finops

import (
	"testing"
	"time"
)

func TestManager_TeamCRUD(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	team := &Team{ID: "ml-team", Name: "ML Team", Budget: 1000}
	if err := m.RegisterTeam(team); err != nil {
		t.Fatal(err)
	}

	if err := m.RegisterTeam(team); err != ErrTeamExists {
		t.Fatalf("expected exists, got %v", err)
	}

	got, err := m.GetTeam("ml-team")
	if err != nil || got.Name != "ML Team" {
		t.Fatal("get failed")
	}

	list := m.ListTeams()
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
}

func TestManager_RecordAndGetCost(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.RegisterTeam(&Team{ID: "team-a", Name: "Team A"})

	now := time.Now()
	m.RecordUsage(UsageRecord{
		Team:         "team-a",
		FeatureGroup: "user_clicks",
		Category:     CostStorage,
		Quantity:     100, // 100 GB-months
		Timestamp:    now,
	})
	m.RecordUsage(UsageRecord{
		Team:         "team-a",
		FeatureGroup: "user_clicks",
		Category:     CostAPI,
		Quantity:     50, // 50K requests
		Timestamp:    now,
	})

	tc, err := m.GetTeamCost("team-a", now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	if tc.Total <= 0 {
		t.Fatal("expected positive total cost")
	}
	if len(tc.ByCategory) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(tc.ByCategory))
	}
	if tc.ByCategory[CostStorage] <= 0 {
		t.Fatal("expected storage cost > 0")
	}
}

func TestManager_FeatureGroupCost(t *testing.T) {
	m := NewManager(DefaultManagerConfig())

	now := time.Now()
	m.RecordUsage(UsageRecord{
		Team: "t1", FeatureGroup: "fg1", Category: CostCompute, Quantity: 10, Timestamp: now,
	})
	m.RecordUsage(UsageRecord{
		Team: "t2", FeatureGroup: "fg1", Category: CostStorage, Quantity: 5, Timestamp: now,
	})

	fgc := m.GetFeatureGroupCost("fg1", now.Add(-time.Hour), now.Add(time.Hour))
	if fgc.Total <= 0 {
		t.Fatal("expected positive total")
	}
	if len(fgc.ByTeam) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(fgc.ByTeam))
	}
}

func TestManager_Recommendations(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.RegisterTeam(&Team{ID: "team-a", Name: "Team A", Budget: 1})

	// Record high-cost usage to trigger recommendation
	for i := 0; i < 100; i++ {
		m.RecordUsage(UsageRecord{
			Team:         "team-a",
			FeatureGroup: "expensive_fg",
			Category:     CostStorage,
			Quantity:     10,
			Timestamp:    time.Now(),
		})
	}

	recs := m.GetRecommendations()
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	hasBudgetWarning := false
	for _, r := range recs {
		if r.Type == "budget_warning" {
			hasBudgetWarning = true
		}
	}
	if !hasBudgetWarning {
		t.Fatal("expected budget warning recommendation")
	}
}

func TestManager_PredictCost(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.RegisterTeam(&Team{ID: "team-a"})

	// Need usage data
	for i := 0; i < 10; i++ {
		m.RecordUsage(UsageRecord{
			Team:      "team-a",
			Category:  CostAPI,
			Quantity:  100,
			Timestamp: time.Now().Add(-time.Duration(i) * 24 * time.Hour),
		})
	}

	pred, err := m.PredictCost("team-a", 30)
	if err != nil {
		t.Fatal(err)
	}
	if pred.EstimatedCost <= 0 {
		t.Fatal("expected positive prediction")
	}
	if pred.Confidence <= 0 {
		t.Fatal("expected positive confidence")
	}
}

func TestManager_Summary(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.RecordUsage(UsageRecord{
		Team: "t1", Category: CostStorage, Quantity: 10, Timestamp: time.Now(),
	})

	summary := m.Summary(time.Now().Add(-time.Hour))
	if summary["total_cost"].(float64) <= 0 {
		t.Fatal("expected positive total cost in summary")
	}
}

func TestManager_SetRate(t *testing.T) {
	m := NewManager(DefaultManagerConfig())
	m.SetRate(&CostRate{Category: CostStorage, Rate: 0.05, Unit: "GB-month", Currency: "USD"})

	rates := m.GetRates()
	if rates[CostStorage].Rate != 0.05 {
		t.Fatalf("expected 0.05, got %f", rates[CostStorage].Rate)
	}
}
