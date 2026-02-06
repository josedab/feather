// Package cost provides cost attribution and chargeback functionality.
package cost

import (
	"time"
)

// CostCategory represents a category of costs.
type CostCategory string //nolint:revive

const (
	// CostCategoryStorage represents storage costs.
	CostCategoryStorage CostCategory = "storage"
	// CostCategoryCompute represents compute costs.
	CostCategoryCompute CostCategory = "compute"
	// CostCategoryNetwork represents network costs.
	CostCategoryNetwork CostCategory = "network"
	// CostCategoryAPI represents API costs.
	CostCategoryAPI CostCategory = "api"
	// CostCategoryML represents ML costs.
	CostCategoryML CostCategory = "ml"
	// CostCategoryVector represents vector costs.
	CostCategoryVector CostCategory = "vector"
)

// CostUnit represents the unit of measurement for costs.
type CostUnit string //nolint:revive

const (
	// CostUnitBytes represents bytes.
	CostUnitBytes CostUnit = "bytes"
	// CostUnitRequests represents requests.
	CostUnitRequests CostUnit = "requests"
	// CostUnitCPUSeconds represents CPU seconds.
	CostUnitCPUSeconds CostUnit = "cpu_seconds"
	// CostUnitGPUSeconds represents GPU seconds.
	CostUnitGPUSeconds CostUnit = "gpu_seconds"
	// CostUnitTokens represents tokens.
	CostUnitTokens CostUnit = "tokens"
	// CostUnitEmbeddings represents embeddings.
	CostUnitEmbeddings CostUnit = "embeddings"
)

// CostRate defines pricing for a specific metric.
type CostRate struct { //nolint:revive
	Category      CostCategory `json:"category"`
	Unit          CostUnit     `json:"unit"`
	PricePerUnit  float64      `json:"pricePerUnit"`
	Description   string       `json:"description,omitempty"`
	MinCharge     float64      `json:"minCharge,omitempty"`
	FreeAllowance float64      `json:"freeAllowance,omitempty"`
}

// UsageRecord represents a single usage event.
type UsageRecord struct {
	ID           string            `json:"id"`
	TenantID     string            `json:"tenantId"`
	FeatureGroup string            `json:"featureGroup,omitempty"`
	Feature      string            `json:"feature,omitempty"`
	Category     CostCategory      `json:"category"`
	Unit         CostUnit          `json:"unit"`
	Quantity     float64           `json:"quantity"`
	Timestamp    time.Time         `json:"timestamp"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// CostEntry represents a calculated cost for usage.
type CostEntry struct { //nolint:revive
	ID           string       `json:"id"`
	TenantID     string       `json:"tenantId"`
	FeatureGroup string       `json:"featureGroup,omitempty"`
	Feature      string       `json:"feature,omitempty"`
	Category     CostCategory `json:"category"`
	Unit         CostUnit     `json:"unit"`
	Quantity     float64      `json:"quantity"`
	Rate         float64      `json:"rate"`
	Cost         float64      `json:"cost"`
	Currency     string       `json:"currency"`
	Timestamp    time.Time    `json:"timestamp"`
	PeriodStart  time.Time    `json:"periodStart"`
	PeriodEnd    time.Time    `json:"periodEnd"`
}

// CostSummary provides an aggregated view of costs.
type CostSummary struct { //nolint:revive
	TenantID     string                   `json:"tenantId,omitempty"`
	FeatureGroup string                   `json:"featureGroup,omitempty"`
	PeriodStart  time.Time                `json:"periodStart"`
	PeriodEnd    time.Time                `json:"periodEnd"`
	TotalCost    float64                  `json:"totalCost"`
	Currency     string                   `json:"currency"`
	ByCategory   map[CostCategory]float64 `json:"byCategory"`
	ByFeature    map[string]float64       `json:"byFeature,omitempty"`
	ByTenant     map[string]float64       `json:"byTenant,omitempty"`
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID          string        `json:"id"`
	TenantID    string        `json:"tenantId"`
	PeriodStart time.Time     `json:"periodStart"`
	PeriodEnd   time.Time     `json:"periodEnd"`
	GeneratedAt time.Time     `json:"generatedAt"`
	DueDate     time.Time     `json:"dueDate"`
	Status      InvoiceStatus `json:"status"`
	Subtotal    float64       `json:"subtotal"`
	Tax         float64       `json:"tax"`
	TaxRate     float64       `json:"taxRate"`
	Total       float64       `json:"total"`
	Currency    string        `json:"currency"`
	LineItems   []LineItem    `json:"lineItems"`
	Credits     []CreditEntry `json:"credits,omitempty"`
	Notes       string        `json:"notes,omitempty"`
}

// InvoiceStatus represents the status of an invoice.
type InvoiceStatus string

const (
	// InvoiceStatusDraft indicates a draft invoice.
	InvoiceStatusDraft InvoiceStatus = "draft"
	// InvoiceStatusPending indicates a pending invoice.
	InvoiceStatusPending InvoiceStatus = "pending"
	// InvoiceStatusPaid indicates a paid invoice.
	InvoiceStatusPaid InvoiceStatus = "paid"
	// InvoiceStatusOverdue indicates an overdue invoice.
	InvoiceStatusOverdue InvoiceStatus = "overdue"
	// InvoiceStatusCancelled indicates a canceled invoice.
	InvoiceStatusCancelled InvoiceStatus = "cancelled" //nolint:misspell
)

// LineItem represents a single item on an invoice.
type LineItem struct {
	Description  string       `json:"description"`
	Category     CostCategory `json:"category"`
	Unit         CostUnit     `json:"unit"`
	Quantity     float64      `json:"quantity"`
	UnitPrice    float64      `json:"unitPrice"`
	Amount       float64      `json:"amount"`
	FeatureGroup string       `json:"featureGroup,omitempty"`
}

// CreditEntry represents a credit applied to an invoice.
type CreditEntry struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Reason      string  `json:"reason,omitempty"`
}

// Budget defines spending limits.
type Budget struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenantId"`
	Name            string         `json:"name"`
	Amount          float64        `json:"amount"`
	Currency        string         `json:"currency"`
	Period          BudgetPeriod   `json:"period"`
	AlertThresholds []float64      `json:"alertThresholds"` // Percentages (0.5, 0.8, 0.95)
	Categories      []CostCategory `json:"categories,omitempty"`
	FeatureGroups   []string       `json:"featureGroups,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

// BudgetPeriod defines the budget period.
type BudgetPeriod string

const (
	// BudgetPeriodDaily represents a daily budget.
	BudgetPeriodDaily BudgetPeriod = "daily"
	// BudgetPeriodWeekly represents a weekly budget.
	BudgetPeriodWeekly BudgetPeriod = "weekly"
	// BudgetPeriodMonthly represents a monthly budget.
	BudgetPeriodMonthly BudgetPeriod = "monthly"
	// BudgetPeriodYearly represents a yearly budget.
	BudgetPeriodYearly BudgetPeriod = "yearly"
)

// BudgetStatus represents current budget consumption.
type BudgetStatus struct {
	BudgetID     string    `json:"budgetId"`
	TenantID     string    `json:"tenantId"`
	PeriodStart  time.Time `json:"periodStart"`
	PeriodEnd    time.Time `json:"periodEnd"`
	Spent        float64   `json:"spent"`
	BudgetAmount float64   `json:"budgetAmount"`
	Remaining    float64   `json:"remaining"`
	PercentUsed  float64   `json:"percentUsed"`
	Projected    float64   `json:"projected"`
	Currency     string    `json:"currency"`
	AlertLevel   string    `json:"alertLevel,omitempty"` // warning, critical, exceeded
}

// Alert represents a cost alert.
type Alert struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenantId"`
	Type         AlertType `json:"type"`
	Severity     string    `json:"severity"` // info, warning, critical
	Message      string    `json:"message"`
	BudgetID     string    `json:"budgetId,omitempty"`
	Threshold    float64   `json:"threshold,omitempty"`
	ActualValue  float64   `json:"actualValue,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

// AlertType represents the type of alert.
type AlertType string

const (
	// AlertTypeBudgetThreshold indicates a budget threshold alert.
	AlertTypeBudgetThreshold AlertType = "budget_threshold"
	// AlertTypeBudgetExceeded indicates a budget exceeded alert.
	AlertTypeBudgetExceeded AlertType = "budget_exceeded"
	// AlertTypeAnomalyDetected indicates an anomaly alert.
	AlertTypeAnomalyDetected AlertType = "anomaly_detected"
	// AlertTypeCostSpike indicates a cost spike alert.
	AlertTypeCostSpike AlertType = "cost_spike"
)

// Chargeback represents cost allocation to a cost center.
type Chargeback struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenantId"`
	CostCenter  string            `json:"costCenter"`
	PeriodStart time.Time         `json:"periodStart"`
	PeriodEnd   time.Time         `json:"periodEnd"`
	TotalCost   float64           `json:"totalCost"`
	Currency    string            `json:"currency"`
	Breakdown   []ChargebackItem  `json:"breakdown"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	GeneratedAt time.Time         `json:"generatedAt"`
}

// ChargebackItem represents a single item in a chargeback.
type ChargebackItem struct {
	FeatureGroup string       `json:"featureGroup"`
	Category     CostCategory `json:"category"`
	Cost         float64      `json:"cost"`
	Percentage   float64      `json:"percentage"`
}

// CostAllocationRule defines how costs are allocated.
type CostAllocationRule struct { //nolint:revive
	ID            string    `json:"id"`
	TenantID      string    `json:"tenantId"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	SourcePattern string    `json:"sourcePattern"` // Feature pattern to match
	CostCenter    string    `json:"costCenter"`
	Percentage    float64   `json:"percentage"` // 0-100
	Priority      int       `json:"priority"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ReportConfig defines configuration for cost reports.
type ReportConfig struct {
	TenantID      string         `json:"tenantId,omitempty"`
	FeatureGroups []string       `json:"featureGroups,omitempty"`
	Categories    []CostCategory `json:"categories,omitempty"`
	CostCenters   []string       `json:"costCenters,omitempty"`
	Granularity   string         `json:"granularity"` // hourly, daily, weekly, monthly
	Format        string         `json:"format"`      // json, csv, pdf
	IncludeTrends bool           `json:"includeTrends"`
}

// CostReport represents a generated cost report.
type CostReport struct { //nolint:revive
	ID           string                   `json:"id"`
	Config       ReportConfig             `json:"config"`
	PeriodStart  time.Time                `json:"periodStart"`
	PeriodEnd    time.Time                `json:"periodEnd"`
	GeneratedAt  time.Time                `json:"generatedAt"`
	TotalCost    float64                  `json:"totalCost"`
	Currency     string                   `json:"currency"`
	ByCategory   map[CostCategory]float64 `json:"byCategory"`
	ByFeature    map[string]float64       `json:"byFeature,omitempty"`
	ByTenant     map[string]float64       `json:"byTenant,omitempty"`
	ByCostCenter map[string]float64       `json:"byCostCenter,omitempty"`
	TimeSeries   []TimeSeriesPoint        `json:"timeSeries,omitempty"`
	Trends       *CostTrends              `json:"trends,omitempty"`
}

// TimeSeriesPoint represents a single point in cost time series.
type TimeSeriesPoint struct {
	Timestamp  time.Time                `json:"timestamp"`
	Cost       float64                  `json:"cost"`
	ByCategory map[CostCategory]float64 `json:"byCategory,omitempty"`
}

// CostTrends represents cost trends over time.
type CostTrends struct { //nolint:revive
	PeriodOverPeriodChange float64   `json:"periodOverPeriodChange"` // Percentage
	AverageDailyCost       float64   `json:"averageDailyCost"`
	ProjectedMonthly       float64   `json:"projectedMonthly"`
	HighestDay             time.Time `json:"highestDay"`
	HighestDayCost         float64   `json:"highestDayCost"`
	LowestDay              time.Time `json:"lowestDay"`
	LowestDayCost          float64   `json:"lowestDayCost"`
}
