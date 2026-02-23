// Package mlflow provides integration with MLflow experiment tracking.
//
// It enables automatic logging of feature usage to MLflow runs, lineage
// tracking between features and ML models, and experiment-level feature
// metadata. Configure via [Config] with tracking URI and auto-log settings.
//
// Usage:
//
//	tracker := mlflow.NewTracker(mlflow.Config{
//	    TrackingURI:     "http://localhost:5000",
//	    AutoLogFeatures: true,
//	    LineageTracking: true,
//	})
//	tracker.LogFeatureUsage(runID, []string{"user_click_count"})
package mlflow
