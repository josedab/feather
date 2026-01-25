package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/feather-store/feather/cmd/feather-cli/internal/output"
)

var (
	deepHealth bool
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check server health",
	Long: `Check the health status of the Feather server.

Examples:
  # Basic health check
  feather-cli health

  # Deep health check (includes all components)
  feather-cli health --deep

  # Output as JSON
  feather-cli health --deep -o json
`,
	RunE: runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
	healthCmd.Flags().BoolVar(&deepHealth, "deep", false, "perform deep health check of all components")
}

func runHealth(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	endpoint := "/live"
	if deepHealth {
		endpoint = "/health"
	}

	url := cfg.ServerURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		formatter.PrintError(fmt.Errorf("server unreachable: %w", err))
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	result := &output.HealthResult{
		Status: "unknown",
	}

	// Parse the response
	var healthResp struct {
		Status     string `json:"status"`
		Components map[string]struct {
			Status  string `json:"status"`
			Message string `json:"message,omitempty"`
		} `json:"components,omitempty"`
	}

	if err := json.Unmarshal(body, &healthResp); err == nil {
		result.Status = healthResp.Status
		if healthResp.Components != nil {
			result.Components = make(map[string]output.HealthCheck)
			for name, c := range healthResp.Components {
				result.Components[name] = output.HealthCheck{
					Status:  c.Status,
					Message: c.Message,
				}
			}
		}
	} else {
		// Simple response
		if resp.StatusCode == http.StatusOK {
			result.Status = "healthy"
		} else {
			result.Status = "unhealthy"
		}
	}

	return formatter.PrintHealthResult(result)
}
