package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/feather-store/feather/cmd/feather-cli/internal/output"
	"github.com/feather-store/feather/sdk/go/feather"
)

var (
	schemaFile string
)

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Manage feature group schemas",
	Long:  `Commands for listing, viewing, and creating feature group schemas.`,
}

// schemaListCmd represents the schema list command
var schemaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all feature group schemas",
	Long: `List all registered feature group schemas.

Examples:
  # List all schemas
  feather-cli schema list

  # Output as JSON
  feather-cli schema list -o json
`,
	RunE: runSchemaList,
}

// schemaGetCmd represents the schema get command
var schemaGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get a specific feature group schema",
	Long: `Retrieve details of a specific feature group schema.

Examples:
  # Get schema details
  feather-cli schema get user_features

  # Output as YAML
  feather-cli schema get user_features -o yaml
`,
	Args: cobra.ExactArgs(1),
	RunE: runSchemaGet,
}

// schemaCreateCmd represents the schema create command
var schemaCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new feature group schema",
	Long: `Create a new feature group schema from a JSON/YAML file.

Examples:
  # Create from file
  feather-cli schema create --file schema.json

  # Create from stdin
  cat schema.json | feather-cli schema create --file -
`,
	RunE: runSchemaCreate,
}

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.AddCommand(schemaListCmd)
	schemaCmd.AddCommand(schemaGetCmd)
	schemaCmd.AddCommand(schemaCreateCmd)

	schemaCreateCmd.Flags().StringVarP(&schemaFile, "file", "f", "", "path to schema definition file (use - for stdin)")
	schemaCreateCmd.MarkFlagRequired("file")
}

func runSchemaList(cmd *cobra.Command, args []string) error {
	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// List features from catalog
	features, err := client.Catalog.List(ctx, nil)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	// Group by entity type to create schema-like view
	schemaMap := make(map[string]*output.SchemaResult)
	for _, f := range features {
		key := f.EntityType
		if key == "" {
			key = "default"
		}
		if _, ok := schemaMap[key]; !ok {
			schemaMap[key] = &output.SchemaResult{
				Name:       key,
				EntityType: f.EntityType,
				Features:   []output.SchemaFeature{},
			}
		}
		schemaMap[key].Features = append(schemaMap[key].Features, output.SchemaFeature{
			Name:        f.Name,
			DataType:    f.DataType,
			Description: f.Description,
		})
	}

	schemas := make([]*output.SchemaResult, 0, len(schemaMap))
	for _, s := range schemaMap {
		schemas = append(schemas, s)
	}

	return formatter.PrintSchemaList(schemas)
}

func runSchemaGet(cmd *cobra.Command, args []string) error {
	name := args[0]

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Get feature definition
	feature, err := client.Catalog.Get(ctx, name)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	result := &output.SchemaResult{
		Name:       feature.Name,
		EntityType: feature.EntityType,
		Features: []output.SchemaFeature{
			{
				Name:        feature.Name,
				DataType:    feature.DataType,
				Description: feature.Description,
			},
		},
		Metadata: feature.Metadata,
	}

	return formatter.Print(result)
}

func runSchemaCreate(cmd *cobra.Command, args []string) error {
	var data []byte
	var err error

	if schemaFile == "-" {
		// Read from stdin
		data, err = os.ReadFile("/dev/stdin")
	} else {
		data, err = os.ReadFile(schemaFile)
	}
	if err != nil {
		return fmt.Errorf("reading schema file: %w", err)
	}

	// Parse the schema definition
	var def feather.FeatureDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return fmt.Errorf("parsing schema file: %w", err)
	}

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	err = client.Catalog.Register(ctx, &def)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	formatter.PrintSuccess(fmt.Sprintf("created schema '%s'", def.Name))
	return nil
}
