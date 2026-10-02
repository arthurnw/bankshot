package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/phinze/bankshot/pkg/config"
	"github.com/phinze/bankshot/pkg/daemon"
	"github.com/phinze/bankshot/pkg/logging"
	"github.com/phinze/bankshot/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		configPath string
		debug      bool
		logFile    string
	)

	cmd := &cobra.Command{
		Use:   "bankshotd",
		Short: "Bankshot daemon - opens URLs and forwards ports from remote SSH sessions",
		Long: `Bankshot daemon runs on your local machine and listens for commands
from remote SSH sessions. It can open URLs in your local browser and
manage SSH port forwards dynamically.`,
		Version: version.GetFullVersion(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			if debug {
				cfg.LogLevel = "debug"
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			logger, err := logging.New(logging.ParseLevel(cfg.LogLevel), logFile)
			if err != nil {
				return err
			}
			slog.SetDefault(logger)

			slog.Info("Starting bankshot daemon",
				"version", version.GetVersion(),
				"commit", version.Commit,
				"date", version.Date,
			)

			// Create and run daemon
			d := daemon.New(cfg, logger)
			return d.Run()
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to configuration file (default: ~/.config/bankshot/config.yaml)")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	cmd.Flags().StringVar(&logFile, "log-file", "", "Write logs to this file, rotated at 10 MiB, instead of stderr")

	return cmd
}
