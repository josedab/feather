// Package cmd provides the CLI commands for feather-cli.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/feather-store/feather/cmd/feather-cli/internal/config"
	"github.com/feather-store/feather/cmd/feather-cli/internal/output"
)

var (
	// Global flags
	cfgFile   string
	serverURL string
	outputFmt string
	apiKey    string
	verbose   bool

	// Global configuration and formatter
	cfg       *config.Config
	formatter *output.Formatter
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "feather-cli",
	Short: "CLI client for Feather feature store",
	Long: `feather-cli is a command-line interface for interacting with the
Feather feature store. It provides commands for managing features, schemas,
vectors, and more.

Examples:
  # Get features for an entity
  feather-cli features get user:123 --feature score --feature age

  # Put features
  feather-cli features put user:123 score=0.95 age=25

  # Search vectors
  feather-cli vectors search my-index --vector 0.1,0.2,0.3 --top-k 10

  # Check server health
  feather-cli health
`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize configuration
		var err error
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Override with flags
		if serverURL != "" {
			cfg.ServerURL = serverURL
		}
		if apiKey != "" {
			cfg.APIKey = apiKey
		}
		if outputFmt != "" {
			cfg.OutputFormat = outputFmt
		}
		if verbose {
			cfg.Verbose = verbose
		}

		// Validate server URL
		if cfg.ServerURL == "" {
			cfg.ServerURL = "http://localhost:8080"
		}

		// Initialize formatter
		formatter = output.NewFormatter(cfg.OutputFormat, os.Stdout)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.feather-cli.yaml)")
	rootCmd.PersistentFlags().StringVarP(&serverURL, "server", "s", "", "Feather server URL (default: http://localhost:8080)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "", "output format: table, json, yaml (default: table)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Bind flags to viper
	cobra.CheckErr(viper.BindPFlag("server_url", rootCmd.PersistentFlags().Lookup("server")))
	cobra.CheckErr(viper.BindPFlag("output_format", rootCmd.PersistentFlags().Lookup("output")))
	cobra.CheckErr(viper.BindPFlag("api_key", rootCmd.PersistentFlags().Lookup("api-key")))
	cobra.CheckErr(viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}

		// Search config in home directory with name ".feather-cli"
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".feather-cli")
	}

	// Read environment variables with FEATHER_CLI_ prefix
	viper.SetEnvPrefix("FEATHER_CLI")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err != nil {
		// Config is optional; ignore not found errors.
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			cobra.CheckErr(err)
		}
	}
}
