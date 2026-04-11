package main

import (
	"fmt"
	"time"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/spf13/cobra"
)

func newLogsCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "logs <chain-id>", Short: "Show chain event log", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load(*configPath)
		if err != nil {
			return err
		}
		rt, err := buildOrchestratorRuntime(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer rt.Cleanup()
		events, err := rt.ChainStore.ListEvents(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		for _, event := range events {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\n", event.ID, event.CreatedAt.Format(time.RFC3339), event.EventType, event.EventData)
		}
		return nil
	}}
}
