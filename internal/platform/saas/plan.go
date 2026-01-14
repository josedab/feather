package saas

import (
	"errors"
	"sync"
	"time"
)

// Common errors
var (
	ErrPlanNotFound         = errors.New("plan not found")
	ErrPlanAlreadyExists    = errors.New("plan already exists")
	ErrInvalidPlan          = errors.New("invalid plan configuration")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrQuotaExceeded        = errors.New("quota exceeded")
)

// PlanTier defines the service tier level.
type PlanTier string

// PlanTier constants for plan tiers.
const (
	TierFree       PlanTier = "free"
	TierStarter    PlanTier = "starter"
	TierPro        PlanTier = "pro"
	TierEnterprise PlanTier = "enterprise"
)

// BillingPeriod defines the billing cycle.
type BillingPeriod string

// BillingPeriod constants for billing cycles.
const (
	BillingMonthly BillingPeriod = "monthly"
	BillingYearly  BillingPeriod = "yearly"
)

// Plan defines a subscription plan.
type Plan struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tier        PlanTier          `json:"tier"`
	Pricing     PlanPricing       `json:"pricing"`
	Quotas      PlanQuotas        `json:"quotas"`
	Features    map[string]bool   `json:"features"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Active      bool              `json:"active"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PlanPricing defines the pricing structure for a plan.
type PlanPricing struct {
	MonthlyPrice  float64            `json:"monthly_price"`
	YearlyPrice   float64            `json:"yearly_price"`
	Currency      string             `json:"currency"`
	UsageRates    map[string]float64 `json:"usage_rates,omitempty"` // Per-unit pricing
	IncludedUnits map[string]int64   `json:"included_units,omitempty"`
	OverageRates  map[string]float64 `json:"overage_rates,omitempty"`
}

// PlanQuotas defines resource limits for a plan.
type PlanQuotas struct {
	// Storage limits
	MaxStorageGB        int64 `json:"max_storage_gb"`
	MaxFeatureGroups    int   `json:"max_feature_groups"`
	MaxFeaturesPerGroup int   `json:"max_features_per_group"`

	// Request limits
	RequestsPerSecond int64 `json:"requests_per_second"`
	RequestsPerDay    int64 `json:"requests_per_day"`
	RequestsPerMonth  int64 `json:"requests_per_month"`

	// Data limits
	MaxEntities         int64 `json:"max_entities"`
	MaxVectorDimensions int   `json:"max_vector_dimensions"`
	MaxBatchSize        int   `json:"max_batch_size"`

	// Instance limits
	MaxInstances int `json:"max_instances"`
	MaxVCPUs     int `json:"max_vcpus"`
	MaxMemoryGB  int `json:"max_memory_gb"`

	// Feature flags
	AllowMultiTenant  bool `json:"allow_multi_tenant"`
	AllowFederation   bool `json:"allow_federation"`
	AllowCustomDomain bool `json:"allow_custom_domain"`
	AllowSSO          bool `json:"allow_sso"`

	// Support
	SupportLevel string  `json:"support_level"` // community, standard, priority, dedicated
	UptimeSLA    float64 `json:"uptime_sla"`    // e.g., 99.9
}

// Subscription represents a customer's subscription to a plan.
type Subscription struct {
	ID                 string             `json:"id"`
	OrganizationID     string             `json:"organization_id"`
	PlanID             string             `json:"plan_id"`
	Status             SubscriptionStatus `json:"status"`
	BillingPeriod      BillingPeriod      `json:"billing_period"`
	CurrentPeriodStart time.Time          `json:"current_period_start"`
	CurrentPeriodEnd   time.Time          `json:"current_period_end"`
	CancelAtPeriodEnd  bool               `json:"cancel_at_period_end"`
	TrialEnd           *time.Time         `json:"trial_end,omitempty"`
	Metadata           map[string]string  `json:"metadata,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// SubscriptionStatus defines subscription states.
type SubscriptionStatus string

// SubscriptionStatus constants for subscriptions.
const (
	SubscriptionActive   SubscriptionStatus = "active"
	SubscriptionTrialing SubscriptionStatus = "trialing"
	SubscriptionPastDue  SubscriptionStatus = "past_due"
	SubscriptionCanceled SubscriptionStatus = "canceled"
	SubscriptionUnpaid   SubscriptionStatus = "unpaid"
	SubscriptionPaused   SubscriptionStatus = "paused"
)

// PlanRegistry manages available plans.
type PlanRegistry struct {
	plans map[string]*Plan
	mu    sync.RWMutex
}

// NewPlanRegistry creates a new plan registry with default plans.
func NewPlanRegistry() *PlanRegistry {
	registry := &PlanRegistry{
		plans: make(map[string]*Plan),
	}

	// Register default plans
	registry.registerDefaultPlans()

	return registry
}

func (r *PlanRegistry) registerDefaultPlans() {
	now := time.Now()

	// Free tier
	r.plans["free"] = &Plan{
		ID:          "free",
		Name:        "Free",
		Description: "For personal projects and experimentation",
		Tier:        TierFree,
		Pricing: PlanPricing{
			MonthlyPrice: 0,
			YearlyPrice:  0,
			Currency:     "USD",
		},
		Quotas: PlanQuotas{
			MaxStorageGB:        1,
			MaxFeatureGroups:    5,
			MaxFeaturesPerGroup: 20,
			RequestsPerSecond:   10,
			RequestsPerDay:      10000,
			RequestsPerMonth:    100000,
			MaxEntities:         10000,
			MaxVectorDimensions: 128,
			MaxBatchSize:        100,
			MaxInstances:        1,
			MaxVCPUs:            1,
			MaxMemoryGB:         1,
			AllowMultiTenant:    false,
			AllowFederation:     false,
			AllowCustomDomain:   false,
			AllowSSO:            false,
			SupportLevel:        "community",
			UptimeSLA:           0,
		},
		Features: map[string]bool{
			"hot_storage":     true,
			"warm_storage":    false,
			"vector_search":   true,
			"drift_detection": false,
			"real_time_agg":   false,
			"export":          true,
			"api_access":      true,
			"webhooks":        false,
		},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Starter tier
	r.plans["starter"] = &Plan{
		ID:          "starter",
		Name:        "Starter",
		Description: "For small teams getting started with ML features",
		Tier:        TierStarter,
		Pricing: PlanPricing{
			MonthlyPrice: 49,
			YearlyPrice:  490,
			Currency:     "USD",
			OverageRates: map[string]float64{
				"requests": 0.001, // $0.001 per 1000 requests
				"storage":  0.10,  // $0.10 per GB
			},
		},
		Quotas: PlanQuotas{
			MaxStorageGB:        10,
			MaxFeatureGroups:    20,
			MaxFeaturesPerGroup: 50,
			RequestsPerSecond:   100,
			RequestsPerDay:      500000,
			RequestsPerMonth:    10000000,
			MaxEntities:         1000000,
			MaxVectorDimensions: 512,
			MaxBatchSize:        1000,
			MaxInstances:        2,
			MaxVCPUs:            4,
			MaxMemoryGB:         8,
			AllowMultiTenant:    false,
			AllowFederation:     false,
			AllowCustomDomain:   false,
			AllowSSO:            false,
			SupportLevel:        "standard",
			UptimeSLA:           99.5,
		},
		Features: map[string]bool{
			"hot_storage":     true,
			"warm_storage":    true,
			"vector_search":   true,
			"drift_detection": true,
			"real_time_agg":   true,
			"export":          true,
			"api_access":      true,
			"webhooks":        true,
		},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Pro tier
	r.plans["pro"] = &Plan{
		ID:          "pro",
		Name:        "Pro",
		Description: "For growing teams with production workloads",
		Tier:        TierPro,
		Pricing: PlanPricing{
			MonthlyPrice: 199,
			YearlyPrice:  1990,
			Currency:     "USD",
			OverageRates: map[string]float64{
				"requests": 0.0005,
				"storage":  0.08,
			},
		},
		Quotas: PlanQuotas{
			MaxStorageGB:        100,
			MaxFeatureGroups:    100,
			MaxFeaturesPerGroup: 200,
			RequestsPerSecond:   1000,
			RequestsPerDay:      5000000,
			RequestsPerMonth:    100000000,
			MaxEntities:         100000000,
			MaxVectorDimensions: 2048,
			MaxBatchSize:        10000,
			MaxInstances:        5,
			MaxVCPUs:            16,
			MaxMemoryGB:         32,
			AllowMultiTenant:    true,
			AllowFederation:     false,
			AllowCustomDomain:   true,
			AllowSSO:            false,
			SupportLevel:        "priority",
			UptimeSLA:           99.9,
		},
		Features: map[string]bool{
			"hot_storage":      true,
			"warm_storage":     true,
			"vector_search":    true,
			"drift_detection":  true,
			"real_time_agg":    true,
			"export":           true,
			"api_access":       true,
			"webhooks":         true,
			"audit_logs":       true,
			"custom_retention": true,
		},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Enterprise tier
	r.plans["enterprise"] = &Plan{
		ID:          "enterprise",
		Name:        "Enterprise",
		Description: "For large organizations with custom needs",
		Tier:        TierEnterprise,
		Pricing: PlanPricing{
			MonthlyPrice: 0, // Custom pricing
			YearlyPrice:  0,
			Currency:     "USD",
		},
		Quotas: PlanQuotas{
			MaxStorageGB:        0, // Unlimited
			MaxFeatureGroups:    0,
			MaxFeaturesPerGroup: 0,
			RequestsPerSecond:   0,
			RequestsPerDay:      0,
			RequestsPerMonth:    0,
			MaxEntities:         0,
			MaxVectorDimensions: 0,
			MaxBatchSize:        0,
			MaxInstances:        0,
			MaxVCPUs:            0,
			MaxMemoryGB:         0,
			AllowMultiTenant:    true,
			AllowFederation:     true,
			AllowCustomDomain:   true,
			AllowSSO:            true,
			SupportLevel:        "dedicated",
			UptimeSLA:           99.99,
		},
		Features: map[string]bool{
			"hot_storage":        true,
			"warm_storage":       true,
			"vector_search":      true,
			"drift_detection":    true,
			"real_time_agg":      true,
			"export":             true,
			"api_access":         true,
			"webhooks":           true,
			"audit_logs":         true,
			"custom_retention":   true,
			"data_residency":     true,
			"private_link":       true,
			"custom_integration": true,
		},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// GetPlan retrieves a plan by ID.
func (r *PlanRegistry) GetPlan(id string) (*Plan, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plan, exists := r.plans[id]
	if !exists {
		return nil, ErrPlanNotFound
	}
	return plan, nil
}

// ListPlans returns all active plans.
func (r *PlanRegistry) ListPlans() []*Plan {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Plan, 0)
	for _, plan := range r.plans {
		if plan.Active {
			result = append(result, plan)
		}
	}
	return result
}

// RegisterPlan adds a custom plan.
func (r *PlanRegistry) RegisterPlan(plan *Plan) error {
	if plan.ID == "" || plan.Name == "" {
		return ErrInvalidPlan
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plans[plan.ID]; exists {
		return ErrPlanAlreadyExists
	}

	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()
	r.plans[plan.ID] = plan
	return nil
}

// UpdatePlan updates an existing plan.
func (r *PlanRegistry) UpdatePlan(plan *Plan) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plans[plan.ID]; !exists {
		return ErrPlanNotFound
	}

	plan.UpdatedAt = time.Now()
	r.plans[plan.ID] = plan
	return nil
}

// DeactivatePlan marks a plan as inactive.
func (r *PlanRegistry) DeactivatePlan(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	plan, exists := r.plans[id]
	if !exists {
		return ErrPlanNotFound
	}

	plan.Active = false
	plan.UpdatedAt = time.Now()
	return nil
}

// ComparePlans returns the differences between two plans.
func (r *PlanRegistry) ComparePlans(planID1, planID2 string) (*PlanComparison, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plan1, exists := r.plans[planID1]
	if !exists {
		return nil, ErrPlanNotFound
	}

	plan2, exists := r.plans[planID2]
	if !exists {
		return nil, ErrPlanNotFound
	}

	comparison := &PlanComparison{
		Plan1:        plan1,
		Plan2:        plan2,
		PriceDiff:    plan2.Pricing.MonthlyPrice - plan1.Pricing.MonthlyPrice,
		QuotaDiffs:   make(map[string]QuotaDiff),
		FeatureDiffs: make(map[string]bool),
	}

	// Compare quotas
	comparison.QuotaDiffs["storage"] = QuotaDiff{
		Plan1Value: float64(plan1.Quotas.MaxStorageGB),
		Plan2Value: float64(plan2.Quotas.MaxStorageGB),
	}
	comparison.QuotaDiffs["requests_per_second"] = QuotaDiff{
		Plan1Value: float64(plan1.Quotas.RequestsPerSecond),
		Plan2Value: float64(plan2.Quotas.RequestsPerSecond),
	}

	// Compare features
	allFeatures := make(map[string]bool)
	for f := range plan1.Features {
		allFeatures[f] = true
	}
	for f := range plan2.Features {
		allFeatures[f] = true
	}

	for feature := range allFeatures {
		p1Has := plan1.Features[feature]
		p2Has := plan2.Features[feature]
		if p1Has != p2Has {
			comparison.FeatureDiffs[feature] = p2Has
		}
	}

	return comparison, nil
}

// PlanComparison represents differences between two plans.
type PlanComparison struct {
	Plan1        *Plan                `json:"plan1"`
	Plan2        *Plan                `json:"plan2"`
	PriceDiff    float64              `json:"price_diff"`
	QuotaDiffs   map[string]QuotaDiff `json:"quota_diffs"`
	FeatureDiffs map[string]bool      `json:"feature_diffs"` // true = gained in plan2
}

// QuotaDiff represents a quota difference.
type QuotaDiff struct {
	Plan1Value float64 `json:"plan1_value"`
	Plan2Value float64 `json:"plan2_value"`
}
