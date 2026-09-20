package runtime

import "testing"

func TestGoalContinuationAuthorityRequiresExactHostFields(t *testing.T) {
	base := GoalContinuationAuthority{
		OwnerUserID:       "owner-1",
		AgentID:           "agent-1",
		ScopeSessionKey:   "agent:agent-1:websocket:dm:conversation-1",
		GoalID:            "goal-1",
		ObjectiveRevision: 3,
		ExecutionID:       "execution-1",
		RootRoundID:       "round-1",
	}
	if !base.Valid() {
		t.Fatal("complete continuation authority should be valid")
	}
	for name, mutate := range map[string]func(*GoalContinuationAuthority){
		"missing owner":    func(value *GoalContinuationAuthority) { value.OwnerUserID = "" },
		"missing agent":    func(value *GoalContinuationAuthority) { value.AgentID = "" },
		"missing session":  func(value *GoalContinuationAuthority) { value.ScopeSessionKey = "" },
		"missing goal":     func(value *GoalContinuationAuthority) { value.GoalID = "" },
		"missing revision": func(value *GoalContinuationAuthority) { value.ObjectiveRevision = 0 },
		"missing round":    func(value *GoalContinuationAuthority) { value.RootRoundID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			value := base
			mutate(&value)
			if value.Valid() {
				t.Fatalf("invalid authority accepted: %+v", value)
			}
		})
	}
}

func TestGoalContinuationAuthorityAllowsGoalOnlyExecutionBinding(t *testing.T) {
	value := GoalContinuationAuthority{
		OwnerUserID:       "owner-1",
		AgentID:           "agent-1",
		ScopeSessionKey:   "agent:agent-1:websocket:dm:conversation-1",
		GoalID:            "goal-1",
		ObjectiveRevision: 1,
		RootRoundID:       "round-1",
	}
	if !value.Valid() {
		t.Fatal("Goal-only continuation should not require an Execution ID")
	}
}
