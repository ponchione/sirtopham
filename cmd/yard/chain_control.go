package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/chain"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
)

func signalYardActiveChainProcess(ctx context.Context, store *chain.Store, chainID string) error {
	events, err := store.ListEvents(ctx, chainID)
	if err != nil {
		return err
	}

	var firstErr error
	if stepProcess, ok := chain.LatestActiveStepProcess(events); ok && stepProcess.ProcessID > 0 {
		if err := interruptYardChainPID(stepProcess.ProcessID); err != nil {
			if !errors.Is(err, errYardChainPIDNotRunning) {
				firstErr = err
			}
		}
	}

	if exec, ok := chain.LatestActiveExecution(events); ok && exec.OrchestratorPID > 0 {
		if err := interruptYardChainPID(exec.OrchestratorPID); err != nil {
			if !errors.Is(err, errYardChainPIDNotRunning) && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func validateYardChainStatusTransition(currentStatus string, targetStatus string, chainID string) error {
	_, err := chain.NextControlStatus(currentStatus, targetStatus)
	if err != nil {
		return fmt.Errorf("chain %s %w", chainID, err)
	}
	return nil
}

func yardSetChainStatus(cmd *cobra.Command, configPath string, chainID string, status string, eventType chain.EventType, message string) error {
	cfg, err := appconfig.Load(configPath)
	if err != nil {
		return err
	}
	rt, err := rtpkg.BuildOrchestratorRuntime(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	defer rt.Cleanup()

	existing, err := rt.ChainStore.GetChain(cmd.Context(), chainID)
	if err != nil {
		return err
	}
	if err := validateYardChainStatusTransition(existing.Status, status, chainID); err != nil {
		return err
	}
	nextStatus, err := chain.NextControlStatus(existing.Status, status)
	if err != nil {
		return fmt.Errorf("chain %s %w", chainID, err)
	}
	if existing.Status == nextStatus {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "chain %s already %s\n", chainID, message)
		return nil
	}
	if nextStatus == "cancel_requested" {
		_ = signalYardActiveChainProcess(cmd.Context(), rt.ChainStore, chainID)
	}
	if err := rt.ChainStore.SetChainStatus(cmd.Context(), chainID, nextStatus); err != nil {
		return err
	}
	_ = rt.ChainStore.LogEvent(cmd.Context(), chainID, "", eventType, map[string]any{"status": nextStatus})
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "chain %s %s\n", chainID, yardControlStatusMessage(status, nextStatus, message))
	return nil
}

func yardControlStatusMessage(targetStatus string, persistedStatus string, fallback string) string {
	switch {
	case targetStatus == "paused" && persistedStatus == "pause_requested":
		return "pause requested"
	case targetStatus == "cancelled" && persistedStatus == "cancel_requested":
		return "cancel requested"
	default:
		return fallback
	}
}
