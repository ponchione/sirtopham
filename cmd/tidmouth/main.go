package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	appconfig "github.com/ponchione/sodoryard/internal/config"
)

const defaultCLIConfigPath = appconfig.ConfigFilename

var version = "dev"

func newRootCmd() *cobra.Command {
	var configPath string

	rootCmd := &cobra.Command{
		Use:          "tidmouth",
		Short:        "A self-hosted AI coding agent",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stdout, "tidmouth %s\n", version)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", defaultCLIConfigPath, "Path to config file")

	initCmd := newInitCmd(&configPath)
	indexCmd := newIndexCmd(&configPath)

	configCmd := newConfigCmd(&configPath)
	llmCmd := newLLMCmd(&configPath)

	authCmd := newAuthCmd(&configPath)
	doctorCmd := newDoctorCmd(&configPath)
	serveCmd := newServeCmd(&configPath)
	runCmd := newRunCmd(&configPath)
	brainServeCmd := newBrainServeCmd()

	rootCmd.AddCommand(serveCmd, runCmd, brainServeCmd, initCmd, indexCmd, configCmd, llmCmd, authCmd, doctorCmd)
	return rootCmd
}

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		if coded, ok := err.(interface{ ExitCode() int }); ok {
			os.Exit(coded.ExitCode())
		}
		os.Exit(1)
	}
}
