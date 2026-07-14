// INPUT: DM 持久引导队列、当前运行 round 与最新 session 元数据。
// OUTPUT: 等待可观察 continuation 确认的 PostToolUse 上下文，以及确认后归入实际回复的 durable user 消息。
// POS: DM 引导消息的唯一注入与确认入口；确认前或失败时必须保留 durable 队列项。
package dm

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
) agentclient.Options {
	return runtimectx.WithPostToolUseGuidanceHook(options, s.inputQueueGuidanceHook(sessionKey, location))
}

func (s *Service) inputQueueGuidanceHook(
	sessionKey string,
	location workspacestore.InputQueueLocation,
) sdkhook.Callback {
	return func(ctx context.Context, input sdkhook.Input, _ string) (sdkhook.Output, error) {
		if input.EventName != "" && input.EventName != sdkhook.EventPostToolUse {
			return sdkhook.Output{}, nil
		}
		runningRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey)
		if len(runningRoundIDs) == 0 {
			return sdkhook.Output{}, nil
		}
		for _, roundID := range runningRoundIDs {
			if err := s.confirmPendingInputQueueGuidance(ctx, sessionKey, location, roundID); err != nil {
				return sdkhook.Output{}, err
			}
		}
		items, err := s.inputQueue.SnapshotGuidance(location, runningRoundIDs...)
		if err != nil {
			return sdkhook.Output{}, err
		}
		if len(items) == 0 {
			return sdkhook.Output{}, nil
		}
		if _, err := s.currentGuidanceSession(location, sessionKey); err != nil {
			return sdkhook.Output{}, err
		}
		prepared := make(map[string]preparedDMGuidance, len(items))
		for _, item := range items {
			sourceRoundID := inputQueueItemRoundID(item)
			targetRoundID := dmdomain.FirstNonEmpty(item.RootRoundID, runningRoundIDs[0])
			runtimeContent, renderErr := s.renderRuntimeContentWithAttachments(ctx, item.Content, item.Attachments)
			if renderErr != nil {
				return sdkhook.Output{}, renderErr
			}
			prepared[item.ID] = preparedDMGuidance{
				item:          item,
				sourceRoundID: sourceRoundID,
				targetRoundID: targetRoundID,
				content:       runtimeContent.PlainText(),
			}
		}
		pending := s.registerPendingInputQueueGuidance(sessionKey, items, prepared)
		if len(pending) == 0 {
			return sdkhook.Output{}, nil
		}
		if activeRoundIDs := s.runtime.GetRunningRoundIDs(sessionKey); !slices.Equal(activeRoundIDs, runningRoundIDs) {
			for _, guidance := range pending {
				s.clearPendingInputQueueGuidance(sessionKey, guidance.targetRoundID)
			}
			return sdkhook.Output{}, nil
		}

		inputs := make([]runtimectx.GuidedInput, 0, len(pending))
		for _, guidance := range pending {
			inputs = append(inputs, runtimectx.GuidedInput{RoundID: guidance.sourceRoundID, Content: guidance.content})
		}

		return sdkhook.Output{
			SpecificOutput: &sdkhook.SpecificOutput{
				HookEventName:     sdkhook.EventPostToolUse,
				AdditionalContext: runtimectx.FormatGuidanceAdditionalContext(inputs),
			},
		}, nil
	}
}

func (s *Service) registerPendingInputQueueGuidance(
	sessionKey string,
	items []protocol.InputQueueItem,
	prepared map[string]preparedDMGuidance,
) []preparedDMGuidance {
	s.inputQueueGuidanceMu.Lock()
	defer s.inputQueueGuidanceMu.Unlock()
	if s.inputQueueGuidancePending == nil {
		s.inputQueueGuidancePending = make(map[string][]preparedDMGuidance)
	}
	registered := make([]preparedDMGuidance, 0, len(prepared))
	for _, item := range items {
		guidance, ok := prepared[item.ID]
		if !ok {
			continue
		}
		key := pendingDMGuidanceKey(sessionKey, guidance.targetRoundID)
		if slices.ContainsFunc(s.inputQueueGuidancePending[key], func(item preparedDMGuidance) bool {
			return item.item.ID == guidance.item.ID
		}) {
			continue
		}
		s.inputQueueGuidancePending[key] = append(s.inputQueueGuidancePending[key], guidance)
		registered = append(registered, guidance)
	}
	return registered
}

func (s *Service) confirmPendingInputQueueGuidance(
	ctx context.Context,
	sessionKey string,
	location workspacestore.InputQueueLocation,
	roundID string,
) error {
	key := pendingDMGuidanceKey(sessionKey, roundID)
	s.inputQueueGuidanceMu.Lock()
	defer s.inputQueueGuidanceMu.Unlock()
	pending := s.inputQueueGuidancePending[key]
	if len(pending) == 0 {
		return nil
	}
	sessionItem, err := s.currentGuidanceSession(location, sessionKey)
	if err != nil {
		return err
	}
	items := make([]protocol.InputQueueItem, 0, len(pending))
	prepared := make(map[string]preparedDMGuidance, len(pending))
	for _, guidance := range pending {
		items = append(items, guidance.item)
		prepared[guidance.item.ID] = guidance
	}
	claimed, snapshot, err := s.inputQueue.DispatchPreparedGuidance(location, items, roundID)
	if err != nil {
		return err
	}
	if len(claimed) == 0 {
		delete(s.inputQueueGuidancePending, key)
		return nil
	}
	for _, item := range claimed {
		guidance := prepared[item.ID]
		updatedSession, persistErr := s.persistConsumedGuidanceUserMessage(
			ctx,
			location,
			sessionItem,
			guidance.item,
			guidance.sourceRoundID,
			guidance.targetRoundID,
		)
		if persistErr != nil {
			restored, restoreErr := s.restorePendingInputQueueGuidance(location, claimed)
			if restoreErr == nil {
				restoredByID := make(map[string]protocol.InputQueueItem, len(restored))
				for _, restoredItem := range restored {
					restoredByID[restoredItem.ID] = restoredItem
				}
				for index := range pending {
					if restoredItem, ok := restoredByID[pending[index].item.ID]; ok {
						pending[index].item = restoredItem
					}
				}
				s.inputQueueGuidancePending[key] = pending
			}
			return errors.Join(persistErr, restoreErr)
		}
		sessionItem = updatedSession
	}
	delete(s.inputQueueGuidancePending, key)
	s.broadcastInputQueueSnapshot(ctx, sessionKey, snapshot)
	return nil
}

func (s *Service) restorePendingInputQueueGuidance(
	location workspacestore.InputQueueLocation,
	items []protocol.InputQueueItem,
) ([]protocol.InputQueueItem, error) {
	entries := make([]workspacestore.InputQueueEnqueue, 0, len(items))
	for _, item := range items {
		entries = append(entries, workspacestore.InputQueueEnqueue{Location: location, Item: item})
	}
	return s.inputQueue.EnqueueBatchWithItems(entries)
}

func (s *Service) clearPendingInputQueueGuidance(sessionKey string, roundID string) {
	s.inputQueueGuidanceMu.Lock()
	defer s.inputQueueGuidanceMu.Unlock()
	delete(s.inputQueueGuidancePending, pendingDMGuidanceKey(sessionKey, roundID))
}

func pendingDMGuidanceKey(sessionKey string, roundID string) string {
	return strings.TrimSpace(sessionKey) + "\x00" + strings.TrimSpace(roundID)
}

type preparedDMGuidance struct {
	item          protocol.InputQueueItem
	sourceRoundID string
	targetRoundID string
	content       string
}

func (s *Service) currentGuidanceSession(
	location workspacestore.InputQueueLocation,
	sessionKey string,
) (protocol.Session, error) {
	item, _, err := s.files.FindSession([]string{location.WorkspacePath}, sessionKey)
	if err != nil {
		return protocol.Session{}, err
	}
	if item == nil {
		return protocol.Session{}, fmt.Errorf("DM guidance session not found: %s", sessionKey)
	}
	return *item, nil
}

func (s *Service) persistConsumedGuidanceUserMessage(
	ctx context.Context,
	location workspacestore.InputQueueLocation,
	sessionItem protocol.Session,
	item protocol.InputQueueItem,
	sourceRoundID string,
	targetRoundID string,
) (protocol.Session, error) {
	userMessageID := strings.TrimSpace(item.SourceMessageID)
	if userMessageID == "" {
		userMessageID = "msg_user_" + strings.TrimSpace(sourceRoundID)
	}
	messageValue := dmdomain.BuildUserRoundMarker(
		sessionItem,
		targetRoundID,
		sourceRoundID,
		userMessageID,
		item.Content,
		protocol.ChatDeliveryPolicyGuide,
		item.Attachments,
	)
	if err := s.history.AppendOverlayMessage(location.WorkspacePath, sessionItem.SessionKey, messageValue); err != nil {
		return sessionItem, err
	}
	updatedSession, metaErr := s.refreshSessionMetaAfterMessage(location.WorkspacePath, sessionItem, messageValue)
	if updatedSession != nil {
		sessionItem = *updatedSession
	}
	event := dmdomain.WrapSessionMessageEvent(sessionItem, messageValue, "durable", targetRoundID)
	s.broadcastEventWithTimeout(ctx, sessionItem.SessionKey, event)
	return sessionItem, metaErr
}
