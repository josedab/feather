// Package fedlearning provides privacy-preserving feature aggregation across
// organizations using secure aggregation protocols and FL framework adapters.
//
// # Usage
//
//	adapter := fedlearning.NewAdapter(fedlearning.DefaultConfig())
//	adapter.RegisterOrg("org-a", fedlearning.OrgConfig{Region: "us-east-1"})
//	result, _ := adapter.SecureAggregate(ctx, request)
package fedlearning
