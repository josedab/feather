package operator

import (
	"testing"
)

func TestNewWebhookController_DefaultRules(t *testing.T) {
	wc := NewWebhookController()
	rules := wc.ListRules()
	if len(rules) != 5 {
		t.Errorf("expected 5 default rules, got %d", len(rules))
	}
}

func TestWebhookController_AddRule(t *testing.T) {
	wc := NewWebhookController()
	wc.AddRule(AdmissionRule{Name: "custom", Resource: "FeatureStore", Enabled: true})
	rules := wc.ListRules()
	if len(rules) != 6 {
		t.Errorf("expected 6 rules, got %d", len(rules))
	}
}

func TestWebhookController_AllowValidRequest(t *testing.T) {
	wc := NewWebhookController()
	resp := wc.Evaluate(AdmissionRequest{
		Operation: "CREATE",
		Resource:  "FeatureStore",
		Name:      "my-store",
		Namespace: "default",
		Object: &FeatureStore{
			Spec: FeatureStoreSpec{Replicas: 3},
		},
	})
	if !resp.Allowed {
		t.Errorf("expected allowed, got denied: %s", resp.Reason)
	}
}

func TestWebhookController_DenyEmptyName(t *testing.T) {
	wc := NewWebhookController()
	resp := wc.Evaluate(AdmissionRequest{
		Operation: "CREATE",
		Resource:  "FeatureStore",
		Name:      "",
		Namespace: "default",
	})
	if resp.Allowed {
		t.Error("expected denied for empty name")
	}
	if resp.Action != AdmissionDeny {
		t.Errorf("expected deny action, got %s", resp.Action)
	}
}

func TestWebhookController_DenyExcessiveReplicas(t *testing.T) {
	wc := NewWebhookController()
	resp := wc.Evaluate(AdmissionRequest{
		Operation: "CREATE",
		Resource:  "FeatureStore",
		Name:      "big-store",
		Namespace: "default",
		Object: &FeatureStore{
			Spec: FeatureStoreSpec{
				Autoscaling: &AutoscalingSpec{MaxReplicas: 100},
			},
		},
	})
	if resp.Allowed {
		t.Error("expected denied for max_replicas > 50")
	}
}

func TestWebhookController_WarnMissingOwner(t *testing.T) {
	wc := NewWebhookController()
	resp := wc.Evaluate(AdmissionRequest{
		Operation: "CREATE",
		Resource:  "FeatureGroup",
		Name:      "my-group",
		Namespace: "default",
		Object: &FeatureGroup{
			Spec: FeatureGroupSpec{
				FeatureStoreRef: "store-1",
				EntityType:      "user",
			},
		},
	})
	if !resp.Allowed {
		t.Error("expected allowed with warning")
	}
	if len(resp.Warnings) == 0 {
		t.Error("expected warnings for missing owner")
	}
}

func TestWebhookController_Stats(t *testing.T) {
	wc := NewWebhookController()

	wc.Evaluate(AdmissionRequest{
		Operation: "CREATE",
		Resource:  "FeatureStore",
		Name:      "store-1",
		Object:    &FeatureStore{Spec: FeatureStoreSpec{Replicas: 1}},
	})
	wc.Evaluate(AdmissionRequest{
		Operation: "CREATE",
		Resource:  "FeatureStore",
		Name:      "",
	})

	stats := wc.Stats()
	if stats.TotalRequests != 2 {
		t.Errorf("expected 2 total requests, got %d", stats.TotalRequests)
	}
	if stats.Allowed != 1 {
		t.Errorf("expected 1 allowed, got %d", stats.Allowed)
	}
	if stats.Denied != 1 {
		t.Errorf("expected 1 denied, got %d", stats.Denied)
	}
}

func TestWebhookController_AuditLog(t *testing.T) {
	wc := NewWebhookController()
	for i := 0; i < 5; i++ {
		wc.Evaluate(AdmissionRequest{
			Operation: "CREATE",
			Resource:  "FeatureStore",
			Name:      "store",
			Object:    &FeatureStore{Spec: FeatureStoreSpec{Replicas: 1}},
		})
	}

	audit := wc.GetAuditLog(3)
	if len(audit) != 3 {
		t.Errorf("expected 3 audit entries, got %d", len(audit))
	}

	all := wc.GetAuditLog(0)
	if len(all) != 5 {
		t.Errorf("expected 5 audit entries for limit=0, got %d", len(all))
	}
}
