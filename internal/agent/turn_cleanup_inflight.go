package agent

func cleanupInflightTurnBase(turnExec *turnExecution, iteration int) inflightTurn {
	return inflightTurn{
		ConversationID:      turnExec.req.ConversationID,
		TurnNumber:          turnExec.req.TurnNumber,
		Iteration:           iteration,
		CompletedIterations: turnExec.completedIterations,
	}
}

func cleanupInflightTurn(conversationID string, turnNumber, iteration, completedIterations int) inflightTurn {
	return inflightTurn{
		ConversationID:      conversationID,
		TurnNumber:          turnNumber,
		Iteration:           iteration,
		CompletedIterations: completedIterations,
	}
}
