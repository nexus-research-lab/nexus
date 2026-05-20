package dm

import (
	"context"
	"strings"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
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
) sdkhook.Callback {
	return func(ctx context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		if input.EventName != "" && input.EventName != sdkhook.EventPostToolUse {
			return sdkhook.Output{}, nil
		}
		runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
		if len(runningRoundIDs) == 0 {
			return sdkhook.Output{}, nil
		}
		items, snapshot, err := s.inputQueue.DispatchGuidance(location, runningRoundIDs...)
		if err != nil {
			return sdkhook.Output{}, err
		}
		if len(items) == 0 {
			return sdkhook.Output{}, nil
		}

		s.broadcastInputQueueSnapshot(ctx, sessionKey, snapshot)
		inputs := make([]runtimectx.GuidedInput, 0, len(items))
		for _, item := range items {
			sourceRoundID := "queue_" + strings.TrimSpace(item.ID)
			targetRoundID := dmdomain.FirstNonEmpty(item.RootRoundID, runningRoundIDs[0])
			runtimeContent, renderErr := s.renderRuntimeContentWithAttachments(ctx, item.Content, item.Attachments)
			if renderErr != nil {
				return sdkhook.Output{}, renderErr
			}
			inputs = append(inputs, runtimectx.GuidedInput{
				RoundID: sourceRoundID,
				Content: runtimeContent.PlainText(),
			})
			s.broadcastGuidanceMessage(ctx, sessionItem, targetRoundID, sourceRoundID, item.Content)
		}

		return sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName:     sdkhook.EventPostToolUse,
				AdditionalContext: runtimectx.FormatGuidanceAdditionalContext(inputs),
			},
		}, nil
	}
}
