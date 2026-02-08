// Package contract provides declarative feature contracts and SLA enforcement.
//
// Contracts define expected feature distributions, freshness guarantees, and
// quality thresholds with automated evaluation and alerting when contracts
// are violated.
//
// # Defining Contracts
//
// Contracts are defined as [Spec] instances which combine multiple [Rule]
// constraints:
//
//	spec := &contract.Spec{
//	    Name:         "user_clicks_quality",
//	    FeatureGroup: "user_engagement",
//	    Rules: []contract.Rule{
//	        {Type: RuleFreshness, MaxStaleness: 5 * time.Minute},
//	        {Type: RuleDistribution, MinValue: 0, MaxValue: 10000},
//	        {Type: RuleCompleteness, MinCompleteness: 0.99},
//	    },
//	}
//
// # Evaluation
//
// The [Manager] evaluates contracts periodically and produces [Violation]
// records when constraints are breached.
package contract
