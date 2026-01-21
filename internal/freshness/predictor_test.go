package freshness

import (
	"testing"
	"time"
)

func TestNewPredictor(t *testing.T) {
	monitor := NewMonitor(DefaultMonitorConfig())
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)

	if predictor == nil {
		t.Fatal("Expected predictor to be non-nil")
	}

	predictor.Stop()
}

func TestDefaultPredictorConfig(t *testing.T) {
	config := DefaultPredictorConfig()

	if config.MinTTL != 1*time.Second {
		t.Errorf("Expected min TTL 1s, got %v", config.MinTTL)
	}
	if config.MaxTTL != 24*time.Hour {
		t.Errorf("Expected max TTL 24h, got %v", config.MaxTTL)
	}
	if config.DefaultTTL != 5*time.Minute {
		t.Errorf("Expected default TTL 5m, got %v", config.DefaultTTL)
	}
	if config.AccessWeight != 0.3 {
		t.Errorf("Expected access weight 0.3, got %f", config.AccessWeight)
	}
	if config.VolatilityWeight != 0.4 {
		t.Errorf("Expected volatility weight 0.4, got %f", config.VolatilityWeight)
	}
	if config.DriftWeight != 0.3 {
		t.Errorf("Expected drift weight 0.3, got %f", config.DriftWeight)
	}
}

func TestPredictor_Predict_NoData(t *testing.T) {
	monitor := NewMonitor(DefaultMonitorConfig())
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	prediction := predictor.Predict("unknown_feature")

	if prediction == nil {
		t.Fatal("Expected prediction to be non-nil")
	}
	if prediction.FeatureName != "unknown_feature" {
		t.Errorf("Expected feature name 'unknown_feature', got '%s'", prediction.FeatureName)
	}
	// Low confidence when no data
	if prediction.Confidence > 0.5 {
		t.Errorf("Expected low confidence when no data, got %f", prediction.Confidence)
	}
}

func TestPredictor_Predict_WithAccessData(t *testing.T) {
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)

	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	// Record high access rate with good cache hits
	for i := 0; i < 100; i++ {
		monitor.RecordAccess("popular_feature", 10*time.Millisecond, true)
	}

	prediction := predictor.Predict("popular_feature")

	if prediction == nil {
		t.Fatal("Expected prediction to be non-nil")
	}
	// Higher confidence with more data
	if prediction.Confidence < 0.3 {
		t.Errorf("Expected higher confidence with data, got %f", prediction.Confidence)
	}
	// Should recommend reasonable TTL
	if prediction.RecommendedTTL <= 0 {
		t.Errorf("Expected positive TTL, got %v", prediction.RecommendedTTL)
	}
}

func TestPredictor_Predict_HighVolatility(t *testing.T) {
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)

	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	// Record high volatility with varying magnitudes (1, 10, 100, etc.)
	// This creates high variance in change magnitudes
	magnitudes := []float64{1, 10, 100, 5, 50, 200, 2, 80, 150, 3}
	for _, mag := range magnitudes {
		monitor.RecordChange("volatile_feature", 0, mag)
	}

	prediction := predictor.Predict("volatile_feature")

	// Volatility score is based on update rate, magnitude, and variance
	// With large varying magnitudes, we should get measurable volatility
	if prediction.VolatilityScore < 0.2 {
		t.Errorf("Expected higher volatility score, got %f", prediction.VolatilityScore)
	}
}

func TestPredictor_Predict_HighDrift(t *testing.T) {
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)

	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	// Record high drift score
	monitor.RecordDriftScore("drifting_feature", 0.9)

	prediction := predictor.Predict("drifting_feature")

	if prediction.DriftScore < 0.8 {
		t.Errorf("Expected high drift score, got %f", prediction.DriftScore)
	}
}

func TestPredictor_GetAllPredictions(t *testing.T) {
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)

	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	// Generate predictions for multiple features
	predictor.Predict("feature1")
	predictor.Predict("feature2")
	predictor.Predict("feature3")

	predictions := predictor.GetAllPredictions()

	if len(predictions) != 3 {
		t.Errorf("Expected 3 predictions, got %d", len(predictions))
	}
}

func TestPredictor_Stats(t *testing.T) {
	monitor := NewMonitor(DefaultMonitorConfig())
	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	predictor.Predict("feature1")
	predictor.Predict("feature2")

	stats := predictor.Stats()

	if stats.TotalPredictions != 2 {
		t.Errorf("Expected 2 predictions, got %d", stats.TotalPredictions)
	}
}

func TestPredictor_TTLBounds(t *testing.T) {
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)

	config := DefaultPredictorConfig()
	config.MinTTL = 1 * time.Second
	config.MaxTTL = 10 * time.Second

	predictor := NewPredictor(config, monitor)
	defer predictor.Stop()

	prediction := predictor.Predict("feature1")

	if prediction.RecommendedTTL < config.MinTTL {
		t.Errorf("TTL %v below minimum %v", prediction.RecommendedTTL, config.MinTTL)
	}
	if prediction.RecommendedTTL > config.MaxTTL {
		t.Errorf("TTL %v above maximum %v", prediction.RecommendedTTL, config.MaxTTL)
	}
}

func TestPredictor_Reason(t *testing.T) {
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.CleanupInterval = 1 * time.Hour
	monitor := NewMonitor(monitorConfig)

	predictor := NewPredictor(DefaultPredictorConfig(), monitor)
	defer predictor.Stop()

	prediction := predictor.Predict("feature1")

	if prediction.Reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestMinMax(t *testing.T) {
	if min(3.0, 5.0) != 3.0 {
		t.Error("min(3, 5) should be 3")
	}
	if min(5.0, 3.0) != 3.0 {
		t.Error("min(5, 3) should be 3")
	}
	if max(3.0, 5.0) != 5.0 {
		t.Error("max(3, 5) should be 5")
	}
	if max(5.0, 3.0) != 5.0 {
		t.Error("max(5, 3) should be 5")
	}
}
