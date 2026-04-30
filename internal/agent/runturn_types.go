package agent

import (
	stdctx "context"
	"time"

	"github.com/ponchione/sodoryard/internal/db"
	"github.com/ponchione/sodoryard/internal/provider"
)

type turnExecution struct {
	req                 RunTurnRequest
	turnStart           time.Time
	turnCtx             *TurnStartResult
	effectiveProvider   string
	effectiveModel      string
	persistedHistory    []db.Message
	historyNeedsRefresh bool
	currentTurnMessages []provider.Message
	completedIterations int
	totalUsage          provider.Usage
	allToolCalls        []completedToolCall
	detector            *loopDetector
}

type preparedTurn struct {
	ctx     stdctx.Context
	exec    *turnExecution
	cleanup func()
}

type iterationOutcome struct {
	done   bool
	result *TurnResult
}

func (l *AgentLoop) resolveTurnOverrides(req RunTurnRequest) (providerName, modelName string) {
	providerName = l.cfg.ProviderName
	if req.Provider != "" {
		providerName = req.Provider
	}
	modelName = l.cfg.ModelName
	if req.Model != "" {
		modelName = req.Model
	}
	return providerName, modelName
}

func (l *AgentLoop) newTurnExecution(req RunTurnRequest, turnCtx *TurnStartResult, turnStart time.Time) *turnExecution {
	effectiveProvider, effectiveModel := l.resolveTurnOverrides(req)
	var history []db.Message
	if turnCtx != nil {
		history = turnCtx.History
	}
	return &turnExecution{
		req:                 req,
		turnStart:           turnStart,
		turnCtx:             turnCtx,
		effectiveProvider:   effectiveProvider,
		effectiveModel:      effectiveModel,
		persistedHistory:    append([]db.Message(nil), history...),
		currentTurnMessages: []provider.Message{provider.NewUserMessage(req.Message)},
		detector:            newLoopDetector(l.cfg.LoopDetectionThreshold),
	}
}
