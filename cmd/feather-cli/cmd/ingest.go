package cmd

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/feather-store/feather/sdk/go/feather"
)

var (
	ingestFormat  string
	entityColumn  string
	batchSize     int
)

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest feature data",
	Long:  `Commands for ingesting feature data from files or streams.`,
}

// ingestFileCmd represents the ingest file command
var ingestFileCmd = &cobra.Command{
	Use:   "file <path>",
	Short: "Ingest features from a file",
	Long: `Ingest feature values from a JSON or CSV file.

JSON format (newline-delimited):
  {"entity_id": "user:123", "features": {"score": 0.95, "age": 25}}
  {"entity_id": "user:456", "features": {"score": 0.87, "age": 30}}

CSV format (first row is header, must have entity_id column):
  entity_id,score,age
  user:123,0.95,25
  user:456,0.87,30

Examples:
  # Ingest JSON file
  feather-cli ingest file features.json --format json

  # Ingest CSV file
  feather-cli ingest file features.csv --format csv --entity-column entity_id

  # Ingest from stdin
  cat features.json | feather-cli ingest file - --format json
`,
	Args: cobra.ExactArgs(1),
	RunE: runIngestFile,
}

// ingestStreamCmd represents the ingest stream command
var ingestStreamCmd = &cobra.Command{
	Use:   "stream",
	Short: "Ingest features from stdin stream",
	Long: `Continuously ingest feature values from a stream.

Reads newline-delimited JSON from stdin and ingests each record.

Examples:
  # Pipe data from another command
  kafka-console-consumer --topic features | feather-cli ingest stream

  # Interactive input
  feather-cli ingest stream
  {"entity_id": "user:123", "features": {"score": 0.95}}
`,
	RunE: runIngestStream,
}

func init() {
	rootCmd.AddCommand(ingestCmd)
	ingestCmd.AddCommand(ingestFileCmd)
	ingestCmd.AddCommand(ingestStreamCmd)

	ingestFileCmd.Flags().StringVarP(&ingestFormat, "format", "F", "json", "file format: json, csv")
	ingestFileCmd.Flags().StringVar(&entityColumn, "entity-column", "entity_id", "entity ID column name for CSV")
	ingestFileCmd.Flags().IntVarP(&batchSize, "batch-size", "b", 100, "batch size for ingestion")

	ingestStreamCmd.Flags().IntVarP(&batchSize, "batch-size", "b", 100, "batch size for ingestion")
}

func runIngestFile(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	var reader io.Reader
	if filePath == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}
		defer f.Close()
		reader = f
	}

	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	var count int
	var err error

	switch strings.ToLower(ingestFormat) {
	case "json", "jsonl":
		count, err = ingestJSON(ctx, client, reader)
	case "csv":
		count, err = ingestCSV(ctx, client, reader)
	default:
		return fmt.Errorf("unsupported format: %s", ingestFormat)
	}

	if err != nil {
		formatter.PrintError(err)
		return err
	}

	formatter.PrintSuccess(fmt.Sprintf("ingested %d records", count))
	return nil
}

func runIngestStream(cmd *cobra.Command, args []string) error {
	client := feather.NewClient(cfg.ServerURL, cfg.APIKey, nil)
	ctx := context.Background()

	count, err := ingestJSON(ctx, client, os.Stdin)
	if err != nil {
		formatter.PrintError(err)
		return err
	}

	formatter.PrintSuccess(fmt.Sprintf("ingested %d records", count))
	return nil
}

type ingestRecord struct {
	EntityID string                 `json:"entity_id"`
	Features map[string]interface{} `json:"features"`
}

func ingestJSON(ctx context.Context, client *feather.Client, reader io.Reader) (int, error) {
	scanner := bufio.NewScanner(reader)
	var count int
	var batch []*feather.PutRequest

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record ingestRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return count, fmt.Errorf("parsing JSON at line %d: %w", count+1, err)
		}

		batch = append(batch, &feather.PutRequest{
			EntityID: record.EntityID,
			Features: record.Features,
		})

		if len(batch) >= batchSize {
			if err := ingestBatch(ctx, client, batch); err != nil {
				return count, err
			}
			count += len(batch)
			batch = batch[:0]
		}
	}

	// Ingest remaining batch
	if len(batch) > 0 {
		if err := ingestBatch(ctx, client, batch); err != nil {
			return count, err
		}
		count += len(batch)
	}

	return count, scanner.Err()
}

func ingestCSV(ctx context.Context, client *feather.Client, reader io.Reader) (int, error) {
	csvReader := csv.NewReader(reader)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return 0, fmt.Errorf("reading CSV header: %w", err)
	}

	// Find entity column index
	entityIdx := -1
	for i, col := range header {
		if col == entityColumn {
			entityIdx = i
			break
		}
	}
	if entityIdx == -1 {
		return 0, fmt.Errorf("entity column '%s' not found in header", entityColumn)
	}

	var count int
	var batch []*feather.PutRequest

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("reading CSV at row %d: %w", count+2, err)
		}

		features := make(map[string]interface{})
		for i, val := range row {
			if i == entityIdx {
				continue
			}
			features[header[i]] = parseValue(val)
		}

		batch = append(batch, &feather.PutRequest{
			EntityID: row[entityIdx],
			Features: features,
		})

		if len(batch) >= batchSize {
			if err := ingestBatch(ctx, client, batch); err != nil {
				return count, err
			}
			count += len(batch)
			batch = batch[:0]
		}
	}

	// Ingest remaining batch
	if len(batch) > 0 {
		if err := ingestBatch(ctx, client, batch); err != nil {
			return count, err
		}
		count += len(batch)
	}

	return count, nil
}

func ingestBatch(ctx context.Context, client *feather.Client, batch []*feather.PutRequest) error {
	for _, req := range batch {
		if err := client.Features.Put(ctx, req); err != nil {
			return fmt.Errorf("ingesting entity %s: %w", req.EntityID, err)
		}
	}
	return nil
}
