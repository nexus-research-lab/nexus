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
		return from == protocol.GoalStatusActive && (to == protocol.GoalStatusComplete || to == protocol.GoalStatusBlocked)
	case protocol.GoalUpdateSourceSystem:
		return from == protocol.GoalStatusActive && (to == protocol.GoalStatusPaused || to == protocol.GoalStatusBlocked)
	default:
		return canUserTransition(from, to)
	}
}

func canUserTransition(from protocol.GoalStatus, to protocol.GoalStatus) bool {
	switch from {
	case protocol.GoalStatusActive:
		return to == protocol.GoalStatusPaused || to == protocol.GoalStatusComplete || to == protocol.GoalStatusBlocked || to == protocol.GoalStatusCleared
	case protocol.GoalStatusPaused:
		return to == protocol.GoalStatusActive || to == protocol.GoalStatusCleared
	case protocol.GoalStatusBlocked:
		return to == protocol.GoalStatusActive || to == protocol.GoalStatusCleared
	case protocol.GoalStatusComplete, protocol.GoalStatusCleared:
		return false
	default:
		return false
	}
}
