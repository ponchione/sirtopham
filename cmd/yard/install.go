package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/initializer"
)

func newInstallCmd() *cobra.Command {
	var sodoryardAgentsDir string
	var configFilename string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Substitute {{SODORYARD_AGENTS_DIR}} in yard.yaml",
		Long: `Resolve the {{SODORYARD_AGENTS_DIR}} placeholder that 'yard init'
leaves in the generated yard.yaml.

The agents directory is resolved in this order:
  1. The --sodoryard-agents-dir flag value (if set)
  2. The SODORYARD_AGENTS_DIR environment variable (if set)
  3. Error: no agents directory provided

The substitution is destructive (overwrites yard.yaml in place)
and idempotent (re-running on an already-substituted file is a
no-op).

Inside the official yard Docker image, SODORYARD_AGENTS_DIR is
preset to /opt/yard/agents so 'yard install' works with no flags.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, sodoryardAgentsDir, configFilename)
		},
	}
	cmd.Flags().StringVar(&sodoryardAgentsDir, "sodoryard-agents-dir", "", "Absolute path to sodoryard's agents/ directory (overrides SODORYARD_AGENTS_DIR env var)")
	cmd.Flags().StringVar(&configFilename, "config", "yard.yaml", "Path to the yard.yaml file to substitute")
	return cmd
}

func runInstall(cmd *cobra.Command, sodoryardAgentsDir, configFilename string) error {
	if sodoryardAgentsDir == "" {
		sodoryardAgentsDir = os.Getenv("SODORYARD_AGENTS_DIR")
	}
	if sodoryardAgentsDir == "" {
		return fmt.Errorf("no agents directory provided: pass --sodoryard-agents-dir or set SODORYARD_AGENTS_DIR")
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Installing yard config in %s\n", configFilename)
	_, _ = fmt.Fprintf(out, "  agents dir: %s\n", sodoryardAgentsDir)

	result, err := initializer.Install(initializer.InstallOptions{
		ConfigPath:         configFilename,
		SodoryardAgentsDir: sodoryardAgentsDir,
	})
	if err != nil {
		return err
	}

	if result.Substitutions == 0 {
		_, _ = fmt.Fprintln(out, "  no substitutions made (already installed)")
	} else {
		_, _ = fmt.Fprintf(out, "  substituted %d {{SODORYARD_AGENTS_DIR}} occurrences\n", result.Substitutions)
	}
	_, _ = fmt.Fprintln(out, "Done.")
	return nil
}
