// Package qualitygates provides CI/CD integration for feature quality validation,
// including schema validation, data quality assertions, and merge-blocking rules.
//
// # Usage
//
//	validator := qualitygates.NewValidator(qualitygates.DefaultConfig())
//	report, _ := validator.Validate(qualitygates.ValidationRequest{
//	    SchemaPath: "features/user_engagement.yaml",
//	})
package qualitygates
