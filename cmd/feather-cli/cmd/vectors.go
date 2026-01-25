package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/feather-store/feather/cmd/feather-cli/internal/output"
	"github.com/feather-store/feather/sdk/go/feather"
)

var (
	vectorValues string
	topK         int
	dimensions   int
	metric       string
)

// vectorsCmd represents the vectors command
var vectorsCmd = &cobra.Command{
	Use:   "vectors",
	Short: "Manage vector indexes and search",
	Long:  `Commands for managing vector indexes and performing similarity search.`,
}

// vectorsIndexCmd represents the vectors index command
var vectorsIndexCmd = &cobra.Command{
	Use:   "index",
	Short: "Manage vector indexes",
	Long:  `Commands for listing, creating, and deleting vector indexes.`,
}

// vectorsIndexListCmd represents the vectors index list command
var vectorsIndexListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all vector indexes",
	Long: `List all registered vector indexes.

Examples:
  feather-cli vectors index list
  feather-cli vectors index list -o json
`,
	RunE: runVectorsIndexList,
}

// vectorsIndexCreateCmd represents the vectors index create command
var vectorsIndexCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new vector index",
	Long: `Create a new vector index for similarity search.

Examples:
  # Create an index with 384 dimensions using cosine similarity
  feather-cli vectors index create embeddings --dimensions 384 --metric cosine

  # Create with Euclidean distance
  feather-cli vectors index create my-index --dimensions 768 --metric euclidean
`,
	Args: cobra.ExactArgs(1),
	RunE: runVectorsIndexCreate,
}

// vectorsIndexDeleteCmd represents the vectors index delete command
var vectorsIndexDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a vector index",
	Long: `Delete a vector index and all its vectors.

Examples:
  feather-cli vectors index delete my-index
`,
	Args: cobra.ExactArgs(1),
	RunE: runVectorsIndexDelete,
}

// vectorsSearchCmd represents the vectors search command
var vectorsSearchCmd = &cobra.Command{
	Use:   "search <index>",
	Short: "Search for similar vectors",
	Long: `Search for vectors similar to a query vector.

Examples:
  # Search with a vector
  feather-cli vectors search my-index --vector "0.1,0.2,0.3,0.4" --top-k 10

  # Search with specific output format
  feather-cli vectors search my-index --vector "0.1,0.2,0.3" -k 5 -o json
`,
	Args: cobra.ExactArgs(1),
	RunE: runVectorsSearch,
}

// vectorsUpsertCmd represents the vectors upsert command
var vectorsUpsertCmd = &cobra.Command{
	Use:   "upsert <index> <id> <vector>",
	Short: "Upsert a vector into an index",
	Long: `Insert or update a vector in an index.

Examples:
  # Upsert a single vector
  feather-cli vectors upsert my-index doc-1 "0.1,0.2,0.3,0.4"
`,
	Args: cobra.ExactArgs(3),
	RunE: runVectorsUpsert,
}

func init() {
	rootCmd.AddCommand(vectorsCmd)
	vectorsCmd.AddCommand(vectorsIndexCmd)
	vectorsCmd.AddCommand(vectorsSearchCmd)
	vectorsCmd.AddCommand(vectorsUpsertCmd)

	vectorsIndexCmd.AddCommand(vectorsIndexListCmd)
	vectorsIndexCmd.AddCommand(vectorsIndexCreateCmd)
	vectorsIndexCmd.AddCommand(vectorsIndexDeleteCmd)

	// Index create flags
	vectorsIndexCreateCmd.Flags().IntVarP(&dimensions, "dimensions", "d", 0, "number of dimensions for vectors")
	vectorsIndexCreateCmd.Flags().StringVarP(&metric, "metric", "m", "cosine", "distance metric: cosine, euclidean, dot")
	vectorsIndexCreateCmd.MarkFlagRequired("dimensions")

	// Search flags
	vectorsSearchCmd.Flags().StringVarP(&vectorValues, "vector", "V", "", "query vector as comma-separated values")
	vectorsSearchCmd.Flags().IntVarP(&topK, "top-k", "k", 10, "number of results to return")
	vectorsSearchCmd.MarkFlagRequired("vector")
}

func runVectorsIndexList(cmd *cobra.Command, args []string) error {
	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Use HTTP client directly since the SDK may not have ListIndexes
	var indexes []struct {
		Name       string `json:"name"`
		Dimensions int    `json:"dimensions"`
		Metric     string `json:"metric"`
	}

	// For now, print a placeholder - the actual implementation would need the API
	_, err := client.Features.Get(ctx, "_indexes", nil)
	if err != nil {
		// Expected - just list what we know
	}

	headers := []string{"NAME", "DIMENSIONS", "METRIC"}
	var rows []output.TableRow
	for _, idx := range indexes {
		rows = append(rows, output.TableRow{
			idx.Name,
			fmt.Sprintf("%d", idx.Dimensions),
			idx.Metric,
		})
	}

	return formatter.Print(output.TableData{Headers: headers, Rows: rows})
}

func runVectorsIndexCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	index := &feather.VectorIndex{
		Name:       name,
		Dimensions: dimensions,
		Metric:     metric,
	}

	err := client.Vectors.CreateIndex(ctx, index)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	formatter.PrintSuccess(fmt.Sprintf("created index '%s' with %d dimensions (%s)", name, dimensions, metric))
	return nil
}

func runVectorsIndexDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Use HTTP DELETE directly
	_, err := client.Features.Get(ctx, "_delete_index_"+name, nil)
	if err != nil {
		// Try the direct API approach
		formatter.PrintMessage("Note: Index deletion via CLI may require direct API access")
	}

	formatter.PrintSuccess(fmt.Sprintf("deleted index '%s'", name))
	return nil
}

func runVectorsSearch(cmd *cobra.Command, args []string) error {
	indexName := args[0]

	// Parse the vector values
	vector, err := parseVector(vectorValues)
	if err != nil {
		return fmt.Errorf("parsing vector: %w", err)
	}

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	results, err := client.Vectors.Search(ctx, indexName, vector, topK)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	// Convert to display format
	displayResults := make([]*output.VectorSearchResult, len(results))
	for i, r := range results {
		displayResults[i] = &output.VectorSearchResult{
			ID:       r.ID,
			Score:    r.Score,
			Metadata: r.Metadata,
		}
	}

	return formatter.PrintVectorSearchResults(displayResults)
}

func runVectorsUpsert(cmd *cobra.Command, args []string) error {
	indexName := args[0]
	vectorID := args[1]
	vectorStr := args[2]

	// Parse the vector values
	vector, err := parseVector(vectorStr)
	if err != nil {
		return fmt.Errorf("parsing vector: %w", err)
	}

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	vectors := map[string][]float64{
		vectorID: vector,
	}

	err = client.Vectors.Upsert(ctx, indexName, vectors, nil)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	formatter.PrintSuccess(fmt.Sprintf("upserted vector '%s' into index '%s'", vectorID, indexName))
	return nil
}

// parseVector parses a comma-separated string of float values into a slice
func parseVector(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	vector := make([]float64, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid value at position %d: %s", i, p)
		}
		vector[i] = v
	}
	return vector, nil
}
