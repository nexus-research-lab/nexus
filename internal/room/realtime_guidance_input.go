package room

import (
	"context"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

func roomSlotGuidanceHook(slot *activeRoomSlot) sdkprotocol.HookCallback {
	return func(_ context.Context, input sdkprotocol.HookInput, _ string) (sdkprotocol.HookOutput, error) {
		if input.EventName != "" && input.EventName != sdkprotocol.HookEventPostToolUse {
			return sdkprotocol.HookOutput{}, nil
		}
		queuedInputs := slot.drainGuidedInputs()
		if len(queuedInputs) == 0 {
			return sdkprotocol.HookOutput{}, nil
		}
		inputs := make([]runtimectx.GuidedInput, 0, len(queuedInputs))
		for _, item := range queuedInputs {
			inputs = append(inputs, runtimectx.GuidedInput{
				RoundID: item.RoundID,
				Content: item.Content,
			})
		}
		return sdkprotocol.HookOutput{
			HookSpecificOutput: map[string]any{
				"hookEventName":     string(sdkprotocol.HookEventPostToolUse),
				"additionalContext": runtimectx.FormatGuidanceAdditionalContext(inputs),
			},
		}, nil
	}
}
