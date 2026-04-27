package chat

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

func (s *Service) withInputQueueGuidanceHook(
	options agentclient.Options,
	sessionKey string,
	location workspacestore.InputQueueLocation,
	sessionItem protocol.Session,
) agentclient.Options {
	return runtimectx.WithPostToolUseGuidanceHook(options, s.inputQueueGuidanceHook(sessionKey, location, sessionItem))
}

func (s *Service) inputQueueGuidanceHook(
	sessionKey string,
	location workspacestore.InputQueueLocation,
	sessionItem protocol.Session,
) sdkprotocol.HookCallback {
	return func(ctx context.Context, input sdkprotocol.HookInput, _ string) (sdkprotocol.HookOutput, error) {
		if input.EventName != "" && input.EventName != sdkprotocol.HookEventPostToolUse {
			return sdkprotocol.HookOutput{}, nil
		}
		runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
		if len(runningRoundIDs) == 0 {
			return sdkprotocol.HookOutput{}, nil
		}
		items, snapshot, err := s.inputQueue.DispatchGuidance(location, runningRoundIDs...)
		if err != nil {
			return sdkprotocol.HookOutput{}, err
		}
		if len(items) == 0 {
			return sdkprotocol.HookOutput{}, nil
		}

		s.broadcastInputQueueSnapshot(ctx, sessionKey, snapshot)
		inputs := make([]runtimectx.GuidedInput, 0, len(items))
		for _, item := range items {
			sourceRoundID := "queue_" + strings.TrimSpace(item.ID)
			targetRoundID := firstNonEmpty(item.RootRoundID, runningRoundIDs[0])
			inputs = append(inputs, runtimectx.GuidedInput{
				RoundID: sourceRoundID,
				Content: item.Content,
			})
			s.broadcastGuidanceMessage(ctx, sessionItem, targetRoundID, sourceRoundID, item.Content)
		}

		return sdkprotocol.HookOutput{
			HookSpecificOutput: map[string]any{
				"hookEventName":     string(sdkprotocol.HookEventPostToolUse),
				"additionalContext": runtimectx.FormatGuidanceAdditionalContext(inputs),
			},
		}, nil
	}
}
