package main

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ponchione/sodoryard/internal/brain/mcpserver"
	"github.com/ponchione/sodoryard/internal/brain/vault"
	"github.com/ponchione/sodoryard/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newYardBrainCmd(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brain",
		Short: "Brain operations (index, serve)",
	}
	cmd.AddCommand(newYardBrainIndexCmd(configPath), newYardBrainServeCmd())
	return cmd
}

func newYardBrainIndexCmd(configPath *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Rebuild derived brain metadata from the vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := cmdutil.RunBrainIndexForConfig(cmd.Context(), *configPath, cmdutil.DefaultBrainIndexDeps())
			if err != nil {
				return err
			}
			if jsonOut {
				return cmdutil.WriteJSON(cmd.OutOrStdout(), result)
			}
			cmdutil.PrintBrainIndexSummary(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON output")
	return cmd
}

func newYardBrainServeCmd() *cobra.Command {
	var vaultPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the project brain as a standalone MCP server over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			if vaultPath == "" {
				return fmt.Errorf("--vault is required")
			}
			vc, err := vault.New(vaultPath)
			if err != nil {
				return err
			}
			server := mcpserver.NewServer(vc)
			return server.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	cmd.Flags().StringVar(&vaultPath, "vault", "", "Path to the Obsidian vault directory")
	return cmd
}
