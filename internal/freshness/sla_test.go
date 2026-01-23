package freshness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultSLAManagerConfig(t *testing.T) {
	cfg := DefaultSLAManagerConfig()

	assert.Equal(t, time.Minute, cfg.CheckInterval)
	assert.Equal(t, 24*time.Hour, cfg.AlertRetention)
	assert.Equal(t, 1000, cfg.MaxAlerts)
	assert.Equal(t, 5*time.Minute, cfg.DefaultCooldown)
	assert.True(t, cfg.EnableRemediation)
}

func TestNewSLAManager(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()

	manager := NewSLAManager(fm, cfg, nil)
	assert.NotNil(t, manager)
}

func TestSLAManagerLifecycle(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	cfg.CheckInterval = 100 * time.Millisecond
	manager := NewSLAManager(fm, cfg, nil)

	ctx := context.Background()

	// Start
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Start again should be idempotent
	err = manager.Start(ctx)
	require.NoError(t, err)

	// Stop
	err = manager.Stop()
	require.NoError(t, err)

	// Stop again should be idempotent
	err = manager.Stop()
	require.NoError(t, err)
}

func TestRegisterSLA(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	t.Run("valid SLA", func(t *testing.T) {
		spec := &SLASpec{
			ID:             "test-sla-1",
			Name:           "Test SLA",
			FeaturePattern: "user_*",
			Thresholds: SLAThresholds{
				WarningAge:  time.Minute,
				CriticalAge: 5 * time.Minute,
				BreachAge:   10 * time.Minute,
			},
			AlertConfig: SLAAlertConfig{
				CooldownPeriod: time.Minute,
			},
			Enabled: true,
		}

		err := manager.RegisterSLA(spec)
		require.NoError(t, err)

		// Retrieve
		retrieved, err := manager.GetSLA("test-sla-1")
		require.NoError(t, err)
		assert.Equal(t, spec.ID, retrieved.ID)
		assert.Equal(t, spec.Name, retrieved.Name)
	})

	t.Run("duplicate SLA", func(t *testing.T) {
		spec := &SLASpec{
			ID:             "test-sla-1",
			Name:           "Duplicate",
			FeaturePattern: "user_*",
			Thresholds: SLAThresholds{
				BreachAge: 10 * time.Minute,
			},
		}

		err := manager.RegisterSLA(spec)
		assert.ErrorIs(t, err, ErrSLAAlreadyExists)
	})

	t.Run("explicit features", func(t *testing.T) {
		spec := &SLASpec{
			ID:   "test-sla-explicit",
			Name: "Explicit Features SLA",
			Features: []string{
				"click_count",
				"purchase_total",
			},
			Thresholds: SLAThresholds{
				BreachAge: 10 * time.Minute,
			},
		}

		err := manager.RegisterSLA(spec)
		require.NoError(t, err)
	})

	t.Run("missing required fields", func(t *testing.T) {
		tests := []struct {
			name string
			spec *SLASpec
		}{
			{
				name: "missing id",
				spec: &SLASpec{
					FeaturePattern: "user_*",
					Thresholds:     SLAThresholds{BreachAge: time.Minute},
				},
			},
			{
				name: "missing features",
				spec: &SLASpec{
					ID:         "test",
					Thresholds: SLAThresholds{BreachAge: time.Minute},
				},
			},
			{
				name: "missing breach threshold",
				spec: &SLASpec{
					ID:             "test",
					FeaturePattern: "user_*",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := manager.RegisterSLA(tt.spec)
				assert.Error(t, err)
			})
		}
	})
}

func TestUpdateSLA(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	// Register initial
	spec := &SLASpec{
		ID:             "update-test",
		Name:           "Original Name",
		FeaturePattern: "user_*",
		Thresholds: SLAThresholds{
			BreachAge: 10 * time.Minute,
		},
	}
	manager.RegisterSLA(spec)

	t.Run("update existing", func(t *testing.T) {
		spec.Name = "Updated Name"
		err := manager.UpdateSLA(spec)
		require.NoError(t, err)

		retrieved, _ := manager.GetSLA("update-test")
		assert.Equal(t, "Updated Name", retrieved.Name)
	})

	t.Run("update non-existing", func(t *testing.T) {
		nonExisting := &SLASpec{
			ID:             "non-existing",
			FeaturePattern: "user_*",
			Thresholds: SLAThresholds{
				BreachAge: time.Minute,
			},
		}
		err := manager.UpdateSLA(nonExisting)
		assert.ErrorIs(t, err, ErrSLANotFound)
	})
}

func TestDeleteSLA(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	spec := &SLASpec{
		ID:             "delete-test",
		Name:           "To Delete",
		FeaturePattern: "user_*",
		Thresholds: SLAThresholds{
			BreachAge: 10 * time.Minute,
		},
	}
	manager.RegisterSLA(spec)

	t.Run("delete existing", func(t *testing.T) {
		err := manager.DeleteSLA("delete-test")
		require.NoError(t, err)

		_, err = manager.GetSLA("delete-test")
		assert.ErrorIs(t, err, ErrSLANotFound)
	})

	t.Run("delete non-existing", func(t *testing.T) {
		err := manager.DeleteSLA("non-existing")
		assert.ErrorIs(t, err, ErrSLANotFound)
	})
}

func TestListSLAs(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	// Register multiple SLAs
	for i := 0; i < 3; i++ {
		spec := &SLASpec{
			ID:             "list-test-" + string(rune('0'+i)),
			Name:           "Test SLA",
			FeaturePattern: "user_*",
			Thresholds: SLAThresholds{
				BreachAge: 10 * time.Minute,
			},
		}
		manager.RegisterSLA(spec)
	}

	slas := manager.ListSLAs()
	assert.Len(t, slas, 3)
}

func TestEvaluateFeature(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())

	// Record some access to create metrics
	fm.RecordAccess("test_feature", 10*time.Millisecond, true)

	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	spec := &SLASpec{
		ID:       "eval-test",
		Name:     "Evaluation Test",
		Features: []string{"test_feature"},
		Thresholds: SLAThresholds{
			WarningAge:  time.Second,
			CriticalAge: 5 * time.Second,
			BreachAge:   10 * time.Second,
		},
	}
	manager.RegisterSLA(spec)

	// Manually trigger evaluation
	state := manager.evaluateFeature(spec, "test_feature")

	assert.Equal(t, spec.ID, state.SLAID)
	assert.Equal(t, "test_feature", state.Feature)
	assert.NotZero(t, state.LastChecked)
}

func TestSeverityDetermination(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	spec := &SLASpec{
		ID:       "severity-test",
		Name:     "Severity Test",
		Features: []string{"fresh_feature", "stale_feature"},
		Thresholds: SLAThresholds{
			WarningAge:  time.Second,
			CriticalAge: 2 * time.Second,
			BreachAge:   3 * time.Second,
		},
	}
	manager.RegisterSLA(spec)

	// Fresh feature
	fm.RecordAccess("fresh_feature", time.Millisecond, true)
	state := manager.evaluateFeature(spec, "fresh_feature")
	// Severity depends on how quickly we evaluate after RecordAccess
	// Could be OK or Warning depending on timing
	assert.Contains(t, []SLASeverity{SeverityOK, SeverityWarning}, state.Severity)
}

func TestWebhookAlert(t *testing.T) {
	var receivedAlert int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&receivedAlert, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	spec := &SLASpec{
		ID:       "webhook-test",
		Name:     "Webhook Test",
		Features: []string{"webhook_feature"},
		Thresholds: SLAThresholds{
			WarningAge:  time.Millisecond,
			CriticalAge: 2 * time.Millisecond,
			BreachAge:   3 * time.Millisecond,
		},
		AlertConfig: SLAAlertConfig{
			Channels: []AlertChannelConfig{
				{
					Type: AlertChannelWebhook,
					URL:  server.URL,
				},
			},
			CooldownPeriod: time.Second,
		},
	}
	manager.RegisterSLA(spec)

	// Create alert
	alert := &SLAAlert{
		ID:       "test-alert",
		SLAID:    spec.ID,
		Feature:  "webhook_feature",
		Severity: SeverityWarning,
		Message:  "Test alert",
		FiredAt:  time.Now(),
	}

	// Send webhook alert
	ctx := context.Background()
	manager.sendWebhookAlert(ctx, spec.AlertConfig.Channels[0], alert)

	// Wait for webhook delivery
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&receivedAlert))
}

func TestSlackAlert(t *testing.T) {
	var receivedSlack int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&receivedSlack, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	channel := AlertChannelConfig{
		Type: AlertChannelSlack,
		URL:  server.URL,
	}

	alert := &SLAAlert{
		ID:       "test-slack-alert",
		SLAID:    "test-sla",
		Feature:  "test_feature",
		Severity: SeverityCritical,
		Message:  "Critical alert",
		FiredAt:  time.Now(),
		Metrics: &AlertMetrics{
			StaleDuration:  5 * time.Minute,
			FreshnessScore: 25.0,
		},
	}

	ctx := context.Background()
	manager.sendSlackAlert(ctx, channel, alert)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&receivedSlack))
}

func TestRemediation(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	cfg.EnableRemediation = true
	manager := NewSLAManager(fm, cfg, nil)

	var remediationCalled int32
	manager.SetRemediationCallback(func(ctx context.Context, feature string, action RemediationAction, config *SLARemediationConfig) error {
		atomic.AddInt32(&remediationCalled, 1)
		return nil
	})

	spec := &SLASpec{
		ID:       "remediation-test",
		Name:     "Remediation Test",
		Features: []string{"remediation_feature"},
		Thresholds: SLAThresholds{
			WarningAge:  time.Millisecond,
			CriticalAge: 2 * time.Millisecond,
			BreachAge:   3 * time.Millisecond,
		},
		AlertConfig: SLAAlertConfig{
			CooldownPeriod: time.Millisecond,
		},
		RemediationConfig: SLARemediationConfig{
			Enabled: true,
			Actions: map[SLASeverity]RemediationAction{
				SeverityWarning:  RemediationBackfill,
				SeverityCritical: RemediationBackfill,
				SeverityBreach:   RemediationBackfill,
			},
		},
	}
	manager.RegisterSLA(spec)

	// Trigger remediation
	state := &SLAState{
		SLAID:    spec.ID,
		Feature:  "remediation_feature",
		Severity: SeverityCritical,
	}

	ctx := context.Background()
	manager.triggerRemediation(ctx, spec, "remediation_feature", state)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&remediationCalled))
}

func TestAcknowledgeAlert(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	// Add an active alert manually
	alert := &SLAAlert{
		ID:       "ack-test-alert",
		SLAID:    "test-sla",
		Feature:  "test_feature",
		Severity: SeverityWarning,
		FiredAt:  time.Now(),
	}
	manager.mu.Lock()
	manager.activeAlerts[alert.ID] = alert
	manager.mu.Unlock()

	t.Run("acknowledge existing", func(t *testing.T) {
		err := manager.AcknowledgeAlert("ack-test-alert", "test-user")
		require.NoError(t, err)

		manager.mu.RLock()
		ack := manager.activeAlerts["ack-test-alert"]
		manager.mu.RUnlock()

		assert.True(t, ack.Acknowledged)
		assert.Equal(t, "test-user", ack.AcknowledgedBy)
		assert.NotNil(t, ack.AcknowledgedAt)
	})

	t.Run("acknowledge non-existing", func(t *testing.T) {
		err := manager.AcknowledgeAlert("non-existing", "test-user")
		assert.Error(t, err)
	})
}

func TestGetAlerts(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	// Add alerts
	past := time.Now().Add(-time.Hour)
	recent := time.Now().Add(-time.Minute)

	manager.mu.Lock()
	manager.alerts = []*SLAAlert{
		{ID: "old-alert", FiredAt: past},
		{ID: "recent-alert", FiredAt: recent},
	}
	manager.mu.Unlock()

	t.Run("get all alerts", func(t *testing.T) {
		alerts := manager.GetAlerts(past.Add(-time.Minute))
		assert.Len(t, alerts, 2)
	})

	t.Run("get recent alerts", func(t *testing.T) {
		alerts := manager.GetAlerts(past.Add(time.Minute))
		assert.Len(t, alerts, 1)
		assert.Equal(t, "recent-alert", alerts[0].ID)
	})
}

func TestGetActiveAlerts(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	// Add active alerts
	manager.mu.Lock()
	manager.activeAlerts["active-1"] = &SLAAlert{ID: "active-1"}
	manager.activeAlerts["active-2"] = &SLAAlert{ID: "active-2"}
	manager.mu.Unlock()

	alerts := manager.GetActiveAlerts()
	assert.Len(t, alerts, 2)
}

func TestMetrics(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	// Register SLA
	spec := &SLASpec{
		ID:             "metrics-test",
		Name:           "Metrics Test",
		FeaturePattern: "user_*",
		Thresholds: SLAThresholds{
			BreachAge: 10 * time.Minute,
		},
	}
	manager.RegisterSLA(spec)

	metrics := manager.Metrics()
	assert.Equal(t, int64(1), metrics.SLAsRegistered)
}

func TestSLAMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		match   bool
	}{
		{"*", "anything", true},
		{"user_*", "user_clicks", true},
		{"user_*", "purchase_total", false},
		{"*_count", "click_count", true},
		{"*_count", "click_total", false},
		// Note: middle wildcards like "user_*_count" are not supported by matchPattern
		// as it only handles prefix (* at end) and suffix (* at start) matching
		{"exact_match", "exact_match", true},
		{"exact_match", "different", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.name, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.name)
			assert.Equal(t, tt.match, result)
		})
	}
}

func TestSLAStateTracking(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	spec := &SLASpec{
		ID:       "state-test",
		Name:     "State Test",
		Features: []string{"track_feature"},
		Thresholds: SLAThresholds{
			WarningAge:  time.Millisecond,
			CriticalAge: 2 * time.Millisecond,
			BreachAge:   3 * time.Millisecond,
		},
	}
	manager.RegisterSLA(spec)

	// Trigger check
	ctx := context.Background()
	manager.checkSLA(ctx, spec)

	// Get state
	state, err := manager.GetSLAState("state-test", "track_feature")
	require.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, "state-test", state.SLAID)
	assert.Equal(t, "track_feature", state.Feature)

	// Get all states
	allStates := manager.GetAllStates()
	assert.NotEmpty(t, allStates)
	assert.NotEmpty(t, allStates["state-test"])
}

func TestResolveAlerts(t *testing.T) {
	fm := NewManager(DefaultManagerConfig())
	cfg := DefaultSLAManagerConfig()
	manager := NewSLAManager(fm, cfg, nil)

	// Add active alerts
	manager.mu.Lock()
	manager.activeAlerts["alert-1"] = &SLAAlert{
		ID:       "alert-1",
		SLAID:    "test-sla",
		Feature:  "test_feature",
		Severity: SeverityWarning,
	}
	manager.activeAlerts["alert-2"] = &SLAAlert{
		ID:       "alert-2",
		SLAID:    "other-sla",
		Feature:  "other_feature",
		Severity: SeverityWarning,
	}
	manager.mu.Unlock()

	// Resolve alerts for specific SLA/feature
	manager.resolveAlerts("test-sla", "test_feature")

	// Check alert-1 is resolved, alert-2 is still active
	manager.mu.RLock()
	_, exists1 := manager.activeAlerts["alert-1"]
	_, exists2 := manager.activeAlerts["alert-2"]
	manager.mu.RUnlock()

	assert.False(t, exists1)
	assert.True(t, exists2)
}
