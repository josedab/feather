package obsconsole

import (
	"testing"
	"time"
)

func TestConsoleRegisterAndSnapshot(t *testing.T) {
	t.Parallel()
	c := NewConsole(DefaultConsoleConfig())
	c.RegisterFeature("clicks", 5*time.Minute)
	c.RegisterFeature("views", 10*time.Minute)

	snap := c.GetSnapshot()
	if snap.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", snap.TotalFeatures)
	}
	if snap.HealthyFeatures != 2 {
		t.Errorf("expected 2 healthy, got %d", snap.HealthyFeatures)
	}
}

func TestConsoleAddAlert(t *testing.T) {
	t.Parallel()
	c := NewConsole(DefaultConsoleConfig())
	c.RegisterFeature("clicks", time.Minute)

	alert := c.AddAlert("drift", AlertSeverityCritical, "clicks", "distribution shifted")
	if alert.Severity != AlertSeverityCritical {
		t.Errorf("expected critical, got %s", alert.Severity)
	}

	alerts := c.GetAlerts(true)
	if len(alerts) != 1 {
		t.Errorf("expected 1 active alert, got %d", len(alerts))
	}

	c.ResolveAlert(alert.ID)
	activeAlerts := c.GetAlerts(true)
	if len(activeAlerts) != 0 {
		t.Errorf("expected 0 active alerts, got %d", len(activeAlerts))
	}
}

func TestConsoleUpdateQuality(t *testing.T) {
	t.Parallel()
	c := NewConsole(DefaultConsoleConfig())
	c.RegisterFeature("score", time.Minute)
	c.UpdateQuality("score", 0.9, 0.8, 1.0)

	snap := c.GetSnapshot()
	if len(snap.Quality) != 1 {
		t.Fatalf("expected 1 quality entry, got %d", len(snap.Quality))
	}
	if snap.Quality[0].OverallScore != 0.9 {
		t.Errorf("expected 0.9 overall, got %f", snap.Quality[0].OverallScore)
	}
}

func TestConsoleSetCost(t *testing.T) {
	t.Parallel()
	c := NewConsole(DefaultConsoleConfig())
	c.SetCost("clicks", 12.50)

	snap := c.GetSnapshot()
	if snap.CostByFeature["clicks"] != 12.50 {
		t.Errorf("expected 12.50, got %f", snap.CostByFeature["clicks"])
	}
}

func TestConsoleGrafanaDashboard(t *testing.T) {
	t.Parallel()
	c := NewConsole(DefaultConsoleConfig())
	dashboard := c.GenerateGrafanaDashboard()
	if dashboard == "" {
		t.Error("expected non-empty dashboard")
	}
}

func TestConsoleStaleness(t *testing.T) {
	t.Parallel()
	c := NewConsole(DefaultConsoleConfig())
	c.RegisterFeature("old_feature", 0)
	c.freshness["old_feature"].LastUpdate = time.Now().Add(-time.Hour)
	c.freshness["old_feature"].SLATarget = time.Minute

	snap := c.GetSnapshot()
	if snap.StaleFeatures != 1 {
		t.Errorf("expected 1 stale feature, got %d", snap.StaleFeatures)
	}
}
