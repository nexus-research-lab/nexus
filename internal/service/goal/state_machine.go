package goal

import "github.com/nexus-research-lab/nexus/internal/protocol"

func canTransition(source protocol.GoalUpdateSource, from protocol.GoalStatus, to protocol.GoalStatus) bool {
	from = protocol.NormalizeGoalStatus(from)
	to = protocol.NormalizeGoalStatus(to)
	if from == to {
		return true
	}
	switch source {
	case protocol.GoalUpdateSourceModel:
		return (from == protocol.GoalStatusActive || from == protocol.GoalStatusBudgetLimited) &&
			(to == protocol.GoalStatusComplete || to == protocol.GoalStatusBlocked)
	case protocol.GoalUpdateSourceSystem:
		if from == protocol.GoalStatusBudgetLimited && to == protocol.GoalStatusUsageLimited {
			return true
		}
		if from != protocol.GoalStatusActive {
			return false
		}
		return to == protocol.GoalStatusBlocked ||
			to == protocol.GoalStatusComplete ||
			to == protocol.GoalStatusBudgetLimited ||
			to == protocol.GoalStatusUsageLimited
	case protocol.GoalUpdateSourceExternal:
		return canExternalTransition(from, to)
	default:
		return canUserTransition(from, to)
	}
}

func shouldPreserveBudgetLimitedStopRequest(from protocol.GoalStatus, to protocol.GoalStatus) bool {
	from = protocol.NormalizeGoalStatus(from)
	to = protocol.NormalizeGoalStatus(to)
	return from == protocol.GoalStatusBudgetLimited &&
		(to == protocol.GoalStatusPaused || to == protocol.GoalStatusBlocked)
}

func canExternalTransition(from protocol.GoalStatus, to protocol.GoalStatus) bool {
	return to == protocol.GoalStatusActive ||
		to == protocol.GoalStatusPaused ||
		to == protocol.GoalStatusBlocked ||
		to == protocol.GoalStatusBudgetLimited ||
		to == protocol.GoalStatusUsageLimited ||
		to == protocol.GoalStatusComplete
}

func canUserTransition(from protocol.GoalStatus, to protocol.GoalStatus) bool {
	switch from {
	case protocol.GoalStatusActive:
		return to == protocol.GoalStatusPaused || to == protocol.GoalStatusComplete || to == protocol.GoalStatusBlocked
	case protocol.GoalStatusPaused, protocol.GoalStatusBlocked:
		return to == protocol.GoalStatusActive
	case protocol.GoalStatusBudgetLimited, protocol.GoalStatusUsageLimited:
		return to == protocol.GoalStatusActive ||
			to == protocol.GoalStatusPaused ||
			to == protocol.GoalStatusComplete ||
			to == protocol.GoalStatusBlocked
	case protocol.GoalStatusComplete:
		return to == protocol.GoalStatusActive
	default:
		return false
	}
}
