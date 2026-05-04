package operator

import (
	"context"
	"errors"
	"fmt"

	"github.com/ponchione/sodoryard/internal/chain"
	"github.com/ponchione/sodoryard/internal/chainrun"
)

func (s *Service) PauseChain(ctx context.Context, chainID string) (ControlResult, error) {
	return s.setChainStatus(ctx, chainID, "paused", chain.EventChainPaused, "paused")
}

func (s *Service) ResumeChain(ctx context.Context, chainID string) (ControlResult, error) {
	return s.resumeChainExecution(ctx, chainID)
}

func (s *Service) CancelChain(ctx context.Context, chainID string) (ControlResult, error) {
	return s.setChainStatus(ctx, chainID, "cancelled", chain.EventChainCancelled, "cancelled")
}

func (s *Service) setChainStatus(ctx context.Context, chainID string, targetStatus string, eventType chain.EventType, fallbackMessage string) (ControlResult, error) {
	store, err := s.store()
	if err != nil {
		return ControlResult{}, err
	}
	existing, err := store.GetChain(ctx, chainID)
	if err != nil {
		return ControlResult{}, err
	}
	nextStatus, err := chain.NextControlStatus(existing.Status, targetStatus)
	if err != nil {
		return ControlResult{}, fmt.Errorf("chain %s %w", chainID, err)
	}
	result := ControlResult{
		ChainID:        chainID,
		PreviousStatus: existing.Status,
		TargetStatus:   targetStatus,
		Status:         nextStatus,
		EventType:      eventType,
		Message:        controlStatusMessage(targetStatus, nextStatus, fallbackMessage),
	}
	if existing.Status == nextStatus {
		result.Already = true
		result.Message = "already " + fallbackMessage
		return result, nil
	}
	if err := store.SetChainStatus(ctx, chainID, nextStatus); err != nil {
		return ControlResult{}, err
	}
	if err := store.LogEvent(ctx, chainID, "", eventType, map[string]any{"status": nextStatus}); err != nil {
		return ControlResult{}, err
	}
	if nextStatus == "cancel_requested" {
		if s.cancelActiveStart(chainID) {
			result.Warnings = append(result.Warnings, warningf("cancelled in-process chain runner"))
		}
		pids, signalErr := s.signalActiveChainProcesses(ctx, store, chainID)
		result.SignaledPIDs = pids
		if signalErr != nil {
			result.Warnings = append(result.Warnings, warningf("signal active chain process: %v", signalErr))
		}
	}
	return result, nil
}

func (s *Service) signalActiveChainProcesses(ctx context.Context, store *chain.Store, chainID string) ([]int, error) {
	if s == nil || s.processSignaler == nil {
		return nil, nil
	}
	events, err := store.ListEvents(ctx, chainID)
	if err != nil {
		return nil, err
	}
	signaled := make([]int, 0, 2)
	var firstErr error
	if stepProcess, ok := chain.LatestActiveStepProcess(events); ok && stepProcess.ProcessID > 0 {
		signaled = append(signaled, stepProcess.ProcessID)
		if err := s.processSignaler(stepProcess.ProcessID); err != nil && !errors.Is(err, ErrProcessNotRunning) {
			firstErr = err
		}
	}
	if exec, ok := chain.LatestActiveExecution(events); ok && exec.OrchestratorPID > 0 {
		if currentPID := s.currentProcessID(); currentPID > 0 && exec.OrchestratorPID == currentPID {
			return signaled, firstErr
		}
		signaled = append(signaled, exec.OrchestratorPID)
		if err := s.processSignaler(exec.OrchestratorPID); err != nil && !errors.Is(err, ErrProcessNotRunning) && firstErr == nil {
			firstErr = err
		}
	}
	return signaled, firstErr
}

func (s *Service) currentProcessID() int {
	if s == nil || s.processID == nil {
		return 0
	}
	return s.processID()
}

func controlStatusMessage(targetStatus string, persistedStatus string, fallback string) string {
	switch {
	case targetStatus == "paused" && persistedStatus == "pause_requested":
		return "pause requested"
	case targetStatus == "cancelled" && persistedStatus == "cancel_requested":
		return "cancel requested"
	default:
		return fallback
	}
}

func (s *Service) resumeChainExecution(ctx context.Context, chainID string) (ControlResult, error) {
	store, err := s.store()
	if err != nil {
		return ControlResult{}, err
	}
	existing, err := store.GetChain(ctx, chainID)
	if err != nil {
		return ControlResult{}, err
	}
	if _, err := chain.ResumeExecutionReady(existing.Status); err != nil {
		return ControlResult{}, fmt.Errorf("chain %s %w", chainID, err)
	}
	cfg, err := s.config()
	if err != nil {
		return ControlResult{}, err
	}
	startOpts := chainrun.Options{ChainID: chainID}
	chainIDCh := make(chan string, 1)
	doneCh := make(chan startChainDone, 1)
	runnerCtx, runnerCancel := context.WithCancel(context.WithoutCancel(ctx))
	startOpts.OnChainID = func(startedChainID string) {
		s.registerActiveStart(startedChainID, runnerCancel, doneCh)
		select {
		case chainIDCh <- startedChainID:
		default:
		}
	}
	starter := s.chainStarter
	if starter == nil {
		starter = chainrun.Start
	}
	go func() {
		defer s.unregisterActiveStart(chainID)
		result, err := starter(runnerCtx, cfg, startOpts, chainrun.Deps{BuildRuntime: s.buildRuntime, ProcessID: func() int { return 0 }})
		if result != nil && result.ChainID != "" {
			select {
			case chainIDCh <- result.ChainID:
			default:
			}
		}
		doneCh <- startChainDone{Result: result, Err: err}
	}()

	select {
	case startedChainID := <-chainIDCh:
		return ControlResult{
			ChainID:        startedChainID,
			PreviousStatus: existing.Status,
			TargetStatus:   "running",
			Status:         "running",
			EventType:      chain.EventChainResumed,
			Message:        "resumed",
		}, nil
	case done := <-doneCh:
		if done.Err != nil {
			return ControlResult{}, done.Err
		}
		if done.Result == nil {
			return ControlResult{}, fmt.Errorf("chain resume returned no result")
		}
		return ControlResult{
			ChainID:        done.Result.ChainID,
			PreviousStatus: existing.Status,
			TargetStatus:   "running",
			Status:         done.Result.Status,
			EventType:      chain.EventChainResumed,
			Message:        "resumed",
		}, nil
	case <-ctx.Done():
		runnerCancel()
		return ControlResult{}, ctx.Err()
	}
}
