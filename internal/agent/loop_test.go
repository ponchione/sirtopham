package agent

import (
	stdctx "context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ponchione/sirtopham/internal/conversation"
	contextpkg "github.com/ponchione/sirtopham/internal/context"
	"github.com/ponchione/sirtopham/internal/db"
)

type loopSeenFilesStub struct{}

func (loopSeenFilesStub) Contains(path string) (bool, int) {
	if path == "internal/auth/service.go" {
		return true, 2
	}
	return false, 0
}

type persistedUserMessageCall struct {
	conversationID string
	turnNumber     int
	message        string
}

type persistedIterationCall struct {
	conversationID string
	turnNumber     int
	iteration      int
	messages       []conversation.IterationMessage
}

type loopConversationManagerStub struct {
	history               []db.Message
	err                   error
	persistErr            error
	persistIterErr        error
	reconstructCalls      []string
	seenFilesConversation []string
	persistCalls          []persistedUserMessageCall
	persistIterCalls      []persistedIterationCall
	callOrder             []string
	seen                  contextpkg.SeenFileLookup
}

func (s *loopConversationManagerStub) PersistUserMessage(_ stdctx.Context, conversationID string, turnNumber int, message string) error {
	s.persistCalls = append(s.persistCalls, persistedUserMessageCall{
		conversationID: conversationID,
		turnNumber:     turnNumber,
		message:        message,
	})
	s.callOrder = append(s.callOrder, "persist")
	return s.persistErr
}

func (s *loopConversationManagerStub) PersistIteration(_ stdctx.Context, conversationID string, turnNumber, iteration int, messages []conversation.IterationMessage) error {
	s.persistIterCalls = append(s.persistIterCalls, persistedIterationCall{
		conversationID: conversationID,
		turnNumber:     turnNumber,
		iteration:      iteration,
		messages:       append([]conversation.IterationMessage(nil), messages...),
	})
	s.callOrder = append(s.callOrder, "persist_iteration")
	return s.persistIterErr
}

func (s *loopConversationManagerStub) ReconstructHistory(_ stdctx.Context, conversationID string) ([]db.Message, error) {
	s.reconstructCalls = append(s.reconstructCalls, conversationID)
	s.callOrder = append(s.callOrder, "reconstruct")
	if s.err != nil {
		return nil, s.err
	}
	return append([]db.Message(nil), s.history...), nil
}

func (s *loopConversationManagerStub) SeenFiles(conversationID string) contextpkg.SeenFileLookup {
	s.seenFilesConversation = append(s.seenFilesConversation, conversationID)
	return s.seen
}

type loopContextAssemblerStub struct {
	message           string
	history           []db.Message
	scope             contextpkg.AssemblyScope
	modelContextLimit int
	historyTokenCount int
	pkg               *contextpkg.FullContextPackage
	compressionNeeded bool
	err               error
}

func (s *loopContextAssemblerStub) Assemble(
	_ stdctx.Context,
	message string,
	history []db.Message,
	scope contextpkg.AssemblyScope,
	modelContextLimit int,
	historyTokenCount int,
) (*contextpkg.FullContextPackage, bool, error) {
	s.message = message
	s.history = append([]db.Message(nil), history...)
	s.scope = scope
	s.modelContextLimit = modelContextLimit
	s.historyTokenCount = historyTokenCount
	return s.pkg, s.compressionNeeded, s.err
}

func (s *loopContextAssemblerStub) UpdateQuality(stdctx.Context, string, int, bool, []string) error {
	return nil
}

func TestNewAgentLoopPrepareTurnContextCallsLayer3AndEmitsEvents(t *testing.T) {
	sink := NewChannelSink(8)
	history := []db.Message{{ConversationID: "conversation-1", Role: "user", TurnNumber: 1, Iteration: 0, Sequence: 0}}
	report := &contextpkg.ContextAssemblyReport{TurnNumber: 3}
	assembler := &loopContextAssemblerStub{
		pkg: &contextpkg.FullContextPackage{
			Content:    "assembled context",
			TokenCount: 123,
			Report:     report,
			Frozen:     true,
		},
		compressionNeeded: true,
	}
	conversations := &loopConversationManagerStub{history: history, seen: loopSeenFilesStub{}}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    assembler,
		ConversationManager: conversations,
		EventSink:           sink,
	})
	loop.now = func() time.Time { return time.Unix(1700000500, 0).UTC() }

	result, err := loop.PrepareTurnContext(stdctx.Background(), "conversation-1", 3, "fix auth", 200000, 4096)
	if err != nil {
		t.Fatalf("PrepareTurnContext returned error: %v", err)
	}
	if result == nil {
		t.Fatal("PrepareTurnContext returned nil result")
	}
	if !result.CompressionNeeded {
		t.Fatal("CompressionNeeded = false, want true")
	}
	if result.ContextPackage == nil || result.ContextPackage.Report != report {
		t.Fatal("ContextPackage report was not preserved")
	}
	if len(result.History) != 1 || result.History[0].ConversationID != "conversation-1" {
		t.Fatalf("History = %#v, want reconstructed history", result.History)
	}
	if got := assembler.message; got != "fix auth" {
		t.Fatalf("assembler message = %q, want fix auth", got)
	}
	if assembler.scope.ConversationID != "conversation-1" || assembler.scope.TurnNumber != 3 {
		t.Fatalf("assembler scope = %#v, want conversation-1 turn 3", assembler.scope)
	}
	if seen, turn := assembler.scope.SeenFiles.Contains("internal/auth/service.go"); !seen || turn != 2 {
		t.Fatalf("scope seen-files lookup returned (%t, %d), want (true, 2)", seen, turn)
	}
	if assembler.modelContextLimit != 200000 || assembler.historyTokenCount != 4096 {
		t.Fatalf("assembler limits = (%d, %d), want (200000, 4096)", assembler.modelContextLimit, assembler.historyTokenCount)
	}

	first := readEvent(t, sink.Events())
	if got := first.EventType(); got != "status" {
		t.Fatalf("first event type = %q, want status", got)
	}
	status, ok := first.(StatusEvent)
	if !ok || status.State != StateAssemblingContext {
		t.Fatalf("first event = %#v, want StatusEvent(StateAssemblingContext)", first)
	}

	second := readEvent(t, sink.Events())
	if got := second.EventType(); got != "context_debug" {
		t.Fatalf("second event type = %q, want context_debug", got)
	}
	debug, ok := second.(ContextDebugEvent)
	if !ok || debug.Report != report {
		t.Fatalf("second event = %#v, want ContextDebugEvent with report", second)
	}

	third := readEvent(t, sink.Events())
	if got := third.EventType(); got != "status" {
		t.Fatalf("third event type = %q, want status", got)
	}
	waiting, ok := third.(StatusEvent)
	if !ok || waiting.State != StateWaitingForLLM {
		t.Fatalf("third event = %#v, want StatusEvent(StateWaitingForLLM)", third)
	}
}

func TestPrepareTurnContextValidatesDependencies(t *testing.T) {
	loop := NewAgentLoop(AgentLoopDeps{})

	_, err := loop.PrepareTurnContext(stdctx.Background(), "conversation-1", 1, "hello", 1000, 0)
	if err == nil {
		t.Fatal("PrepareTurnContext error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "context assembler is nil") {
		t.Fatalf("error = %q, want missing context assembler", err)
	}
}

func TestPrepareTurnContextBubblesHistoryErrors(t *testing.T) {
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler: &loopContextAssemblerStub{},
		ConversationManager: &loopConversationManagerStub{
			err: sql.ErrNoRows,
		},
	})

	_, err := loop.PrepareTurnContext(stdctx.Background(), "conversation-1", 1, "hello", 1000, 0)
	if err == nil {
		t.Fatal("PrepareTurnContext error = nil, want history error")
	}
	if !strings.Contains(err.Error(), "reconstruct history") {
		t.Fatalf("error = %q, want reconstruct history context", err)
	}
}

func TestRunTurnPersistsUserMessageBeforePreparingContext(t *testing.T) {
	sink := NewChannelSink(8)
	report := &contextpkg.ContextAssemblyReport{TurnNumber: 4}
	assembler := &loopContextAssemblerStub{
		pkg: &contextpkg.FullContextPackage{Content: "assembled", Report: report, Frozen: true},
	}
	conversations := &loopConversationManagerStub{
		history: []db.Message{{ConversationID: "conversation-1", Role: "user", TurnNumber: 4, Sequence: 0}},
		seen:    loopSeenFilesStub{},
	}
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    assembler,
		ConversationManager: conversations,
		EventSink:           sink,
	})
	loop.now = func() time.Time { return time.Unix(1700000600, 0).UTC() }

	result, err := loop.RunTurn(stdctx.Background(), RunTurnRequest{
		ConversationID:    "conversation-1",
		TurnNumber:        4,
		Message:           "fix auth",
		ModelContextLimit: 200000,
		HistoryTokenCount: 4096,
	})
	if err != nil {
		t.Fatalf("RunTurn returned error: %v", err)
	}
	if result == nil || result.ContextPackage == nil || result.ContextPackage.Report != report {
		t.Fatalf("RunTurn result = %#v, want preserved TurnStartResult", result)
	}
	if len(conversations.persistCalls) != 1 {
		t.Fatalf("PersistUserMessage call count = %d, want 1", len(conversations.persistCalls))
	}
	persist := conversations.persistCalls[0]
	if persist.conversationID != "conversation-1" || persist.turnNumber != 4 || persist.message != "fix auth" {
		t.Fatalf("PersistUserMessage call = %#v, want conversation-1/4/fix auth", persist)
	}
	if got := strings.Join(conversations.callOrder, ","); got != "persist,reconstruct" {
		t.Fatalf("call order = %q, want persist,reconstruct", got)
	}

	first := readEvent(t, sink.Events())
	if got := first.EventType(); got != "status" {
		t.Fatalf("first event type = %q, want status", got)
	}
}

func TestRunTurnReturnsErrorEventWhenPersistenceFails(t *testing.T) {
	sink := NewChannelSink(4)
	persistErr := errors.New("db write failed")
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler: &loopContextAssemblerStub{},
		ConversationManager: &loopConversationManagerStub{
			persistErr: persistErr,
		},
		EventSink: sink,
	})
	loop.now = func() time.Time { return time.Unix(1700000700, 0).UTC() }

	_, err := loop.RunTurn(stdctx.Background(), RunTurnRequest{
		ConversationID:    "conversation-1",
		TurnNumber:        1,
		Message:           "hello",
		ModelContextLimit: 200000,
	})
	if err == nil {
		t.Fatal("RunTurn error = nil, want persistence error")
	}
	if !strings.Contains(err.Error(), "persist user message") {
		t.Fatalf("error = %q, want persist user message context", err)
	}

	event := readEvent(t, sink.Events())
	if got := event.EventType(); got != "error" {
		t.Fatalf("event type = %q, want error", got)
	}
	errEvent, ok := event.(ErrorEvent)
	if !ok {
		t.Fatalf("event = %#v, want ErrorEvent", event)
	}
	if errEvent.Recoverable {
		t.Fatal("ErrorEvent.Recoverable = true, want false")
	}
	if errEvent.ErrorCode != "persist_user_message_failed" {
		t.Fatalf("ErrorCode = %q, want persist_user_message_failed", errEvent.ErrorCode)
	}
}

func TestRunTurnValidatesRequest(t *testing.T) {
	loop := NewAgentLoop(AgentLoopDeps{
		ContextAssembler:    &loopContextAssemblerStub{},
		ConversationManager: &loopConversationManagerStub{},
	})

	_, err := loop.RunTurn(stdctx.Background(), RunTurnRequest{Message: "hello", TurnNumber: 1, ModelContextLimit: 200000})
	if err == nil {
		t.Fatal("RunTurn error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "conversation ID") {
		t.Fatalf("error = %q, want conversation ID validation", err)
	}
}
