package contractcicd

import (
	"errors"
	"time"
)

// Errors returned by the contract CI/CD engine.
var (
	ErrContractNotFound      = errors.New("contractcicd: contract not found")
	ErrContractExists        = errors.New("contractcicd: contract already exists")
	ErrBreakingChange        = errors.New("contractcicd: breaking change detected")
	ErrInvalidContract       = errors.New("contractcicd: invalid contract definition")
	ErrMigrationFailed       = errors.New("contractcicd: migration failed")
	ErrPlanNotFound          = errors.New("contractcicd: plan not found")
)

// ChangeType classifies a schema change.
type ChangeType string

const (
	ChangeTypeAdd        ChangeType = "add"
	ChangeTypeRemove     ChangeType = "remove"
	ChangeTypeModify     ChangeType = "modify"
	ChangeTypeRename     ChangeType = "rename"
	ChangeTypeNoOp       ChangeType = "no_op"
)

// Severity indicates the impact of a change.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityBreaking Severity = "breaking"
)

// Contract defines a feature schema contract in YAML/HCL format.
type Contract struct {
	APIVersion string          `json:"api_version" yaml:"api_version"`
	Kind       string          `json:"kind" yaml:"kind"`
	Metadata   ContractMeta    `json:"metadata" yaml:"metadata"`
	Spec       ContractSpec    `json:"spec" yaml:"spec"`
}

// ContractMeta holds contract metadata.
type ContractMeta struct {
	Name      string            `json:"name" yaml:"name"`
	Version   string            `json:"version" yaml:"version"`
	Owner     string            `json:"owner" yaml:"owner"`
	Tags      map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	CreatedAt time.Time         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
}

// ContractSpec defines the feature group contract specification.
type ContractSpec struct {
	EntityType  string          `json:"entity_type" yaml:"entity_type"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	TTL         string          `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	Features    []FeatureContract `json:"features" yaml:"features"`
	Validation  *ValidationRule `json:"validation,omitempty" yaml:"validation,omitempty"`
}

// FeatureContract defines a single feature within a contract.
type FeatureContract struct {
	Name        string   `json:"name" yaml:"name"`
	Type        string   `json:"type" yaml:"type"`
	Required    bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Default     interface{} `json:"default,omitempty" yaml:"default,omitempty"`
}

// ValidationRule defines validation constraints for the contract.
type ValidationRule struct {
	NotNull  bool        `json:"not_null,omitempty" yaml:"not_null,omitempty"`
	MinValue *float64    `json:"min_value,omitempty" yaml:"min_value,omitempty"`
	MaxValue *float64    `json:"max_value,omitempty" yaml:"max_value,omitempty"`
	OneOf    []string    `json:"one_of,omitempty" yaml:"one_of,omitempty"`
	Regex    string      `json:"regex,omitempty" yaml:"regex,omitempty"`
}

// Change describes a single schema change between two contract versions.
type Change struct {
	Type        ChangeType `json:"type"`
	Severity    Severity   `json:"severity"`
	Field       string     `json:"field"`
	Description string     `json:"description"`
	OldValue    string     `json:"old_value,omitempty"`
	NewValue    string     `json:"new_value,omitempty"`
}

// Plan represents the result of comparing contracts against current state.
type Plan struct {
	ID              string    `json:"id"`
	Changes         []Change  `json:"changes"`
	Migrations      []Migration `json:"migrations,omitempty"`
	BreakingChanges int       `json:"breaking_changes"`
	Warnings        int       `json:"warnings"`
	Additions       int       `json:"additions"`
	CreatedAt       time.Time `json:"created_at"`
}

// HasBreakingChanges returns true if the plan contains breaking changes.
func (p *Plan) HasBreakingChanges() bool {
	return p.BreakingChanges > 0
}

// Migration describes an auto-generated migration step.
type Migration struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	SQL         string `json:"sql,omitempty"`
	Reversible  bool   `json:"reversible"`
}

// ApplyResult describes the outcome of applying a plan.
type ApplyResult struct {
	PlanID     string    `json:"plan_id"`
	Applied    int       `json:"applied"`
	Skipped    int       `json:"skipped"`
	Errors     []string  `json:"errors,omitempty"`
	AppliedAt  time.Time `json:"applied_at"`
}

// EngineStats provides aggregate statistics.
type EngineStats struct {
	TotalContracts   int   `json:"total_contracts"`
	TotalPlans       int   `json:"total_plans"`
	TotalApplied     int   `json:"total_applied"`
	BreakingBlocked  int   `json:"breaking_blocked"`
}

// CITemplate represents a generated CI/CD configuration.
type CITemplate struct {
	Provider string `json:"provider"` // "github", "gitlab", "generic"
	Content  string `json:"content"`
}
