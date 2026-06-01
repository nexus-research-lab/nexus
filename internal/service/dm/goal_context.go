package dm

import (
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

const goalContextualInputName = "goal"

func goalContextualInputs(contextText string, goalID string, sessionKey string) []runtimectx.ContextualInputBlock {
	contextText = strings.TrimSpace(contextText)
	if contextText == "" {
		return nil
	}
	metadata := map[string]string{}
	if goalID = strings.TrimSpace(goalID); goalID != "" {
		metadata["goal_id"] = goalID
	}
	if sessionKey = strings.TrimSpace(sessionKey); sessionKey != "" {
		metadata["session_key"] = sessionKey
	}
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock(goalContextualInputName, contextText, 0, metadata),
	}
}
