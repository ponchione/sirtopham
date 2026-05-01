package operator

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ponchione/sodoryard/internal/chain"
)

func (s *Service) ReadReceipt(ctx context.Context, chainID string, step string) (ReceiptView, error) {
	path := fmt.Sprintf("receipts/orchestrator/%s.md", chainID)
	if step != "" {
		store, err := s.store()
		if err != nil {
			return ReceiptView{}, err
		}
		steps, err := store.ListSteps(ctx, chainID)
		if err != nil {
			return ReceiptView{}, err
		}
		if stepPath, ok := receiptPathForStep(chainID, step, steps); ok {
			path = stepPath
		}
	}
	backend, err := s.brainBackend()
	if err != nil {
		return ReceiptView{}, err
	}
	content, err := backend.ReadDocument(ctx, path)
	if err != nil && step == "" {
		fallbackPath, ok := s.defaultStepReceiptPath(ctx, chainID)
		if ok {
			fallbackContent, fallbackErr := backend.ReadDocument(ctx, fallbackPath)
			if fallbackErr == nil {
				return ReceiptView{ChainID: chainID, Step: step, Path: fallbackPath, Content: fallbackContent}, nil
			}
		}
	}
	if err != nil {
		return ReceiptView{}, err
	}
	return ReceiptView{ChainID: chainID, Step: step, Path: path, Content: content}, nil
}

func (s *Service) defaultStepReceiptPath(ctx context.Context, chainID string) (string, bool) {
	store, err := s.store()
	if err != nil {
		return "", false
	}
	steps, err := store.ListSteps(ctx, chainID)
	if err != nil {
		return "", false
	}
	for _, step := range steps {
		if step.ReceiptPath != "" {
			return step.ReceiptPath, true
		}
	}
	return "", false
}

func (s *Service) receiptSummaries(ctx context.Context, chainID string, steps []chain.Step) []ReceiptSummary {
	receipts := make([]ReceiptSummary, 0, len(steps)+1)
	if backend, err := s.brainBackend(); err == nil {
		path := fmt.Sprintf("receipts/orchestrator/%s.md", chainID)
		if _, err := backend.ReadDocument(ctx, path); err == nil {
			receipts = append(receipts, ReceiptSummary{Label: "orchestrator", Path: path})
		}
	}
	for _, step := range steps {
		if step.ReceiptPath == "" {
			continue
		}
		receipts = append(receipts, ReceiptSummary{
			Label: fmt.Sprintf("step %d %s", step.SequenceNum, step.Role),
			Step:  strconv.Itoa(step.SequenceNum),
			Path:  step.ReceiptPath,
		})
	}
	return receipts
}

func receiptPathForStep(chainID string, step string, steps []chain.Step) (string, bool) {
	for _, candidate := range steps {
		if fmt.Sprintf("%d", candidate.SequenceNum) == step {
			return candidate.ReceiptPath, true
		}
	}
	return fmt.Sprintf("receipts/orchestrator/%s.md", chainID), false
}

func (s *Service) brainBackend() (interface {
	ReadDocument(context.Context, string) (string, error)
}, error) {
	if s == nil || s.rt == nil {
		return nil, errors.New("operator service is closed")
	}
	if s.rt.BrainBackend == nil {
		return nil, errors.New("operator runtime brain backend is nil")
	}
	return s.rt.BrainBackend, nil
}
