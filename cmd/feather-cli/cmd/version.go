package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information (set at build time)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Display version information for the CLI and optionally the server.

Examples:
  # Show CLI version
  feather-cli version

  # Show version as JSON
  feather-cli version -o json
`,
	RunE: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

type versionInfo struct {
	CLI    cliVersion     `json:"cli" yaml:"cli"`
	Server *serverVersion `json:"server,omitempty" yaml:"server,omitempty"`
}

type cliVersion struct {
	Version   string `json:"version" yaml:"version"`
	GitCommit string `json:"git_commit" yaml:"git_commit"`
	BuildDate string `json:"build_date" yaml:"build_date"`
	GoVersion string `json:"go_version" yaml:"go_version"`
	Platform  string `json:"platform" yaml:"platform"`
}

type serverVersion struct {
	Version string `json:"version" yaml:"version"`
	Status  string `json:"status" yaml:"status"`
}

func runVersion(cmd *cobra.Command, args []string) error {
	info := versionInfo{
		CLI: cliVersion{
			Version:   Version,
			GitCommit: GitCommit,
			BuildDate: BuildDate,
			GoVersion: runtime.Version(),
			Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		},
	}

	// Try to get server version
	if cfg.ServerURL != "" {
		serverVer := getServerVersion()
		if serverVer != nil {
			info.Server = serverVer
		}
	}

	return formatter.Print(info)
}

func getServerVersion() *serverVersion {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	url := cfg.ServerURL + "/version"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &serverVersion{Status: "unreachable"}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &serverVersion{Status: "unreachable"}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		// Try the health endpoint to at least confirm server is running
		healthReq, err := http.NewRequestWithContext(ctx, "GET", cfg.ServerURL+"/live", nil)
		if err != nil {
			return &serverVersion{Status: "unreachable"}
		}
		healthResp, err := client.Do(healthReq)
		if err != nil {
			return &serverVersion{Status: "unreachable"}
		}
		defer func() {
			_ = healthResp.Body.Close()
		}()
		if healthResp.StatusCode != http.StatusOK {
			return &serverVersion{Status: "unreachable"}
		}
		return &serverVersion{Version: "unknown", Status: "healthy"}
	}

	body, _ := io.ReadAll(resp.Body)
	var verResp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &verResp); err == nil && verResp.Version != "" {
		return &serverVersion{Version: verResp.Version, Status: "healthy"}
	}

	return &serverVersion{Version: string(body), Status: "healthy"}
}
