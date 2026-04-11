package main

import (
	"fmt"

	appconfig "github.com/ponchione/sodoryard/internal/config"
	"github.com/spf13/cobra"
)

func newStatusCmd(configPath *string) *cobra.Command {
	return &cobra.Command{Use: "status [chain-id]", Short: "Show chain status", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := appconfig.Load(*configPath)
		if err != nil {
			return err
		}
		rt, err := buildOrchestratorRuntime(cmd.Context(), cfg)
		if err != nil {
			return err
		}
		defer rt.Cleanup()
		if len(args) == 0 {
			chains, err := rt.ChainStore.ListChains(cmd.Context(), 20)
			if err != nil {
				return err
			}
			for _, ch := range chains {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\tsteps=%d\ttokens=%d\n", ch.ID, ch.Status, ch.TotalSteps, ch.TotalTokens)
			}
			return nil
		}
		ch, err := rt.ChainStore.GetChain(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		steps, err := rt.ChainStore.ListSteps(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "chain=%s status=%s steps=%d tokens=%d duration=%d summary=%s\n", ch.ID, ch.Status, ch.TotalSteps, ch.TotalTokens, ch.TotalDurationSecs, ch.Summary)
		for _, step := range steps {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "step=%d role=%s status=%s verdict=%s receipt=%s\n", step.SequenceNum, step.Role, step.Status, step.Verdict, step.ReceiptPath)
		}
		return nil
	}}
}
