package agent

// AgentState describes the current phase of the agent-loop state machine:
// idle -> assembling context -> waiting for the LLM -> executing tools ->
// (optionally compressing) -> back to idle.
type AgentState string

const (
	// StateIdle means the agent is waiting for a new turn.
	StateIdle AgentState = "idle"
	// StateAssemblingContext means Layer 3 context assembly is running.
	StateAssemblingContext AgentState = "assembling_context"
	// StateWaitingForLLM means the agent is waiting on an LLM response.
	StateWaitingForLLM AgentState = "waiting_for_llm"
	// StateExecutingTools means the agent is dispatching or awaiting tools.
	StateExecutingTools AgentState = "executing_tools"
	// StateCompressing means the agent is compressing conversation history.
	StateCompressing AgentState = "compressing"
)

// String returns the human-readable state value.
func (s AgentState) String() string {
	return string(s)
}
