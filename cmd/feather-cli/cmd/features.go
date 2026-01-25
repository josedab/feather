package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/feather-store/feather/cmd/feather-cli/internal/output"
	"github.com/feather-store/feather/sdk/go/feather"
)

var (
	featureNames []string
	asOfTime     string
)

// featuresCmd represents the features command
var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "Manage features",
	Long:  `Commands for getting, putting, and querying features.`,
}

// getCmd represents the features get command
var featuresGetCmd = &cobra.Command{
	Use:   "get <entity-id>",
	Short: "Get features for an entity",
	Long: `Retrieve feature values for a specific entity.

Examples:
  # Get all features for an entity
  feather-cli features get user:123

  # Get specific features
  feather-cli features get user:123 --feature score --feature age

  # Output as JSON
  feather-cli features get user:123 -o json
`,
	Args: cobra.ExactArgs(1),
	RunE: runFeaturesGet,
}

// featuresPutCmd represents the features put command
var featuresPutCmd = &cobra.Command{
	Use:   "put <entity-id> <feature=value>...",
	Short: "Store features for an entity",
	Long: `Store one or more feature values for an entity.

Examples:
  # Put single feature
  feather-cli features put user:123 score=0.95

  # Put multiple features
  feather-cli features put user:123 score=0.95 age=25 active=true

  # Feature values are auto-detected:
  # - Numbers: 123, 0.95
  # - Booleans: true, false
  # - Strings: everything else
`,
	Args: cobra.MinimumNArgs(2),
	RunE: runFeaturesPut,
}

// featuresBatchCmd represents the features batch command
var featuresBatchCmd = &cobra.Command{
	Use:   "batch <entity-id>...",
	Short: "Get features for multiple entities",
	Long: `Retrieve feature values for multiple entities in a single request.

Examples:
  # Get features for multiple entities
  feather-cli features batch user:123 user:456 user:789 --feature score

  # Get all features for multiple entities
  feather-cli features batch user:123 user:456
`,
	Args: cobra.MinimumNArgs(1),
	RunE: runFeaturesBatch,
}

// featuresHistoryCmd represents the features history command
var featuresHistoryCmd = &cobra.Command{
	Use:   "history <entity-id>",
	Short: "Get features as of a specific time",
	Long: `Retrieve feature values as they existed at a specific point in time.

Examples:
  # Get features as of a specific time
  feather-cli features history user:123 --as-of "2024-01-15T10:30:00Z"

  # With specific features
  feather-cli features history user:123 --feature score --as-of "2024-01-15T10:30:00Z"
`,
	Args: cobra.ExactArgs(1),
	RunE: runFeaturesHistory,
}

func init() {
	rootCmd.AddCommand(featuresCmd)
	featuresCmd.AddCommand(featuresGetCmd)
	featuresCmd.AddCommand(featuresPutCmd)
	featuresCmd.AddCommand(featuresBatchCmd)
	featuresCmd.AddCommand(featuresHistoryCmd)

	// Add flags
	featuresGetCmd.Flags().StringArrayVarP(&featureNames, "feature", "f", nil, "feature names to retrieve (can be repeated)")
	featuresBatchCmd.Flags().StringArrayVarP(&featureNames, "feature", "f", nil, "feature names to retrieve (can be repeated)")
	featuresHistoryCmd.Flags().StringArrayVarP(&featureNames, "feature", "f", nil, "feature names to retrieve (can be repeated)")
	featuresHistoryCmd.Flags().StringVar(&asOfTime, "as-of", "", "point-in-time timestamp (RFC3339 format)")
	featuresHistoryCmd.MarkFlagRequired("as-of")
}

func runFeaturesGet(cmd *cobra.Command, args []string) error {
	entityID := args[0]

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	resp, err := client.Features.Get(ctx, entityID, featureNames)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	result := &output.FeatureResult{
		EntityID: resp.EntityID,
		Features: make(map[string]output.FeatureDisplay),
	}
	for name, fv := range resp.Features {
		result.Features[name] = output.FeatureDisplay{
			Value:     fv.Value,
			Timestamp: fv.Timestamp.Format(time.RFC3339),
			Version:   fv.Version,
		}
	}

	return formatter.PrintFeatureResult(result)
}

func runFeaturesPut(cmd *cobra.Command, args []string) error {
	entityID := args[0]
	features := make(map[string]interface{})

	// Parse feature=value pairs
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid feature format: %s (expected feature=value)", arg)
		}
		name := parts[0]
		value := parseValue(parts[1])
		features[name] = value
	}

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	req := &feather.PutRequest{
		EntityID: entityID,
		Features: features,
	}

	err := client.Features.Put(ctx, req)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	formatter.PrintSuccess(fmt.Sprintf("stored %d features for %s", len(features), entityID))
	return nil
}

func runFeaturesBatch(cmd *cobra.Command, args []string) error {
	entityIDs := args

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	results, err := client.Features.GetBatch(ctx, entityIDs, featureNames)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	// Convert to display format
	displayResults := make(map[string]*output.FeatureResult)
	for entityID, resp := range results {
		result := &output.FeatureResult{
			EntityID: entityID,
			Features: make(map[string]output.FeatureDisplay),
		}
		if resp != nil {
			for name, fv := range resp.Features {
				result.Features[name] = output.FeatureDisplay{
					Value:     fv.Value,
					Timestamp: fv.Timestamp.Format(time.RFC3339),
					Version:   fv.Version,
				}
			}
		}
		displayResults[entityID] = result
	}

	return formatter.Print(displayResults)
}

func runFeaturesHistory(cmd *cobra.Command, args []string) error {
	entityID := args[0]

	// Parse the as-of timestamp
	asOf, err := time.Parse(time.RFC3339, asOfTime)
	if err != nil {
		return fmt.Errorf("invalid as-of timestamp: %w (expected RFC3339 format)", err)
	}

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	resp, err := client.Features.GetAsOf(ctx, entityID, featureNames, asOf)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	result := &output.FeatureResult{
		EntityID: resp.EntityID,
		Features: make(map[string]output.FeatureDisplay),
	}
	for name, fv := range resp.Features {
		result.Features[name] = output.FeatureDisplay{
			Value:     fv.Value,
			Timestamp: fv.Timestamp.Format(time.RFC3339),
			Version:   fv.Version,
		}
	}

	return formatter.PrintFeatureResult(result)
}

// parseValue attempts to parse a string value as int, float, bool, or string
func parseValue(s string) interface{} {
	// Try bool
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Try int
	var i int64
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil && fmt.Sprintf("%d", i) == s {
		return i
	}

	// Try float
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}

	// Default to string
	return s
}
