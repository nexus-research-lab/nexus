// INPUT: WorkGraph 交给主窗口的宿主恢复输入（执行恢复、显式工作恢复、定时工作、评审派发、续跑）、外部 IM 回执，以及用户的停止与开口。
// OUTPUT: 主窗口 inbox 的 host_recovery / im_reply 事件与暂停闸门；派发时先查陈旧与重复，再起隐藏的恢复轮或可见的 IM 回执轮。
// POS: 主窗口对 orchestration 与 IM 回执的唯一入口。暂停、放行、合并与陈旧检查沿用 JSONL 队列的同一套语义，状态落在 inbox 表上。
package dm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	inboxstore "github.com/nexus-research-lab/nexus/internal/storage/inbox"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// mainInboxExecutionOrigin 标出由主窗口 inbox 起的轮次：不经过运行中路由，也不触碰 JSONL 队列。
const mainInboxExecutionOrigin = "inbox"

// resumeMainInbox 在用户开口时恢复暂停闸门。未暂停时不推进代数，免得无故作废在途的续跑计划。
func (s *Service) resumeMainInbox(ctx context.Context, target mainInboxTarget) error {
	gate, err := s.inbox.Gate(ctx, target.sessionKey)
	if err != nil || !gate.Paused {
		return err
	}
	_, err = s.inbox.SetPaused(ctx, target.owner(), target.sessionKey, false)
	return err
}

// pauseMainInbox 记下用户停止：之后的宿主恢复输入不再自动派发，用户输入与定时事件不受影响。
func (s *Service) pauseMainInbox(ctx context.Context, target mainInboxTarget) error {
	_, err := s.inbox.SetPaused(ctx, target.owner(), target.sessionKey, true)
	return err
}

// mainInboxForExecution 解析 WorkGraph 指向的会话是否主窗口；ok=false 时调用方继续走 JSONL 队列。
func (s *Service) mainInboxForExecution(ctx context.Context, sessionKey string, agentID string) (mainInboxTarget, bool, error) {
	if s.inbox == nil {
		return mainInboxTarget{}, false, nil
	}
	return s.resolveMainInboxTarget(ctx, sessionKey, agentID)
}

// enqueueMainInboxRecovery 幂等写入一条宿主恢复输入，返回承接它的事件。
//   - 同一入队键重复提交返回原事件；意图不同即冲突。
//   - 续跑计划读到的闸门代数已被取代（期间有过停止或恢复）即拒绝。
//   - 带显式目标的输入在入队时获准越过当前暂停；之后再有停止或恢复即失效。
//   - 同一 execution、同一目标与绑定的恢复还在等待时只保留那一条，本次只留一行入队记录供按键查找。
func (s *Service) enqueueMainInboxRecovery(ctx context.Context, target mainInboxTarget, key string, content string, source inboxstore.RecoverySource) (inboxstore.Event, error) {
	dedupeKey := hashedInboxDedupeKey("host_recovery", key)
	existing, found, err := s.inbox.FindByDedupe(ctx, target.sessionKey, dedupeKey)
	if err != nil || found {
		if err == nil && !sameRecoveryIntent(existing, content, source) {
			err = fmt.Errorf("%w: recovery key %q", workspacestore.ErrInputQueueIdempotencyConflict, key)
		}
		return existing, err
	}
	if err = protocol.ValidateInputQueueCapabilityEnvelope(recoveryQueueItem(target, content, source)); err != nil {
		return inboxstore.Event{}, err
	}
	gate, err := s.inbox.Gate(ctx, target.sessionKey)
	if err != nil {
		return inboxstore.Event{}, err
	}
	if source.PauseRevision != nil && *source.PauseRevision != gate.Revision {
		return inboxstore.Event{}, workspacestore.ErrContinuationStopped
	}
	if source.Target != nil {
		revision := gate.Revision
		source.AdmittedRevision = &revision
	}
	event := inboxstore.Event{
		EventID:     newInboxEventID(),
		OwnerUserID: target.owner(),
		InboxKey:    target.sessionKey,
		Kind:        inboxstore.KindHostRecovery,
		DedupeKey:   dedupeKey,
		Payload:     inboxstore.Payload{Content: content, Recovery: &source},
	}
	coalesced, ok, err := s.pendingRecoveryLike(ctx, target, source, gate)
	if err != nil {
		return inboxstore.Event{}, err
	}
	if ok {
		event.Status = inboxstore.StatusConsumed
		if _, _, err = s.inbox.Append(ctx, event); err != nil {
			return inboxstore.Event{}, err
		}
		return coalesced, nil
	}
	stored, _, err := s.inbox.Append(ctx, event)
	if err != nil {
		return inboxstore.Event{}, err
	}
	if !runtimehost.RecoveryDispatchDeferred(ctx) {
		s.wakeMainInbox(target)
	}
	return stored, nil
}

// pendingRecoveryLike 找可以合并的待处理恢复：多个终态回执常需要同一次协调者检查。
// 被闸门挡住的显式恢复不能吸收新的显式请求，否则用户这次的放行会丢失。
func (s *Service) pendingRecoveryLike(ctx context.Context, target mainInboxTarget, source inboxstore.RecoverySource, gate inboxstore.Gate) (inboxstore.Event, bool, error) {
	open, err := s.inbox.ListOpen(ctx, target.sessionKey, time.Now().UTC())
	if err != nil {
		return inboxstore.Event{}, false, err
	}
	for _, event := range pendingEvents(open) {
		existing := event.Payload.Recovery
		if event.Kind != inboxstore.KindHostRecovery || existing == nil || existing.ExecutionID != source.ExecutionID ||
			!reflect.DeepEqual(existing.Target, source.Target) ||
			!reflect.DeepEqual(existing.Work, source.Work) || !reflect.DeepEqual(existing.Review, source.Review) {
			continue
		}
		if source.Target == nil || len(dispatchableEvents([]inboxstore.Event{event}, gate)) == 1 {
			return event, true, nil
		}
	}
	return inboxstore.Event{}, false, nil
}

// sameRecoveryIntent 比较两次入队是否同一意图；获准越过闸门的代数是入队时的派生值，不参与比较。
func sameRecoveryIntent(existing inboxstore.Event, content string, source inboxstore.RecoverySource) bool {
	if existing.Kind != inboxstore.KindHostRecovery || existing.Payload.Recovery == nil || existing.Payload.Content != content {
		return false
	}
	stored := *existing.Payload.Recovery
	stored.AdmittedRevision, source.AdmittedRevision = nil, nil
	return reflect.DeepEqual(stored, source)
}

// findMainInboxRecovery 按入队键找回恢复目标；合并掉的入队也留有一行记录。
func (s *Service) findMainInboxRecovery(ctx context.Context, target mainInboxTarget, key string) (*protocol.ExecutionRecoveryTarget, error) {
	event, found, err := s.inbox.FindByDedupe(ctx, target.sessionKey, hashedInboxDedupeKey("host_recovery", key))
	if err != nil || !found || event.Payload.Recovery == nil {
		return nil, err
	}
	return event.Payload.Recovery.Target, nil
}

// startRecoveryRound 起一轮隐藏的恢复轮。目标已陈旧、或上一进程已为它起过轮（轮次 ID 由 event_id 决定），
// 就只把事件记为 consumed 并继续取下一批，不重复执行。
func (s *Service) startRecoveryRound(ctx context.Context, target mainInboxTarget, roundID string, batch []inboxstore.Event) inboxStart {
	event := batch[0]
	recovery := event.Payload.Recovery
	current := recovery != nil
	var err error
	if current {
		current, err = s.RecoveryInputCurrent(ctx, recoveryQueueItem(target, event.Payload.Content, *recovery))
	}
	if err == nil && current {
		var persisted bool
		persisted, err = s.History.ForOwner(target.owner()).HasRoundMarker(target.agent.WorkspacePath, target.sessionKey, roundID)
		current = !persisted
	}
	if err != nil {
		s.releaseMainInbox(ctx, target, roundID)
		s.reportMainInboxFailure(ctx, target, err)
		return inboxStartNone
	}
	if !current {
		s.consumeMainInbox(ctx, target, roundID, batch)
		return inboxStartNext
	}
	return s.startMainInboxBatch(ctx, target, batch, Request{
		SessionKey:          target.sessionKey,
		AgentID:             target.agent.AgentID,
		Content:             event.Payload.Content,
		RoundID:             roundID,
		Internal:            true,
		ExecutionID:         recovery.ExecutionID,
		ExecutionOrigin:     mainInboxExecutionOrigin,
		queuedWorkBinding:   recovery.Work.Clone(),
		queuedReviewBinding: recovery.Review.Clone(),
	})
}

// recoveryQueueItem 把恢复事件还原成 JSONL 队列项的形状：信封校验与陈旧检查两边共用同一套规则。
func recoveryQueueItem(target mainInboxTarget, content string, source inboxstore.RecoverySource) protocol.InputQueueItem {
	return protocol.InputQueueItem{
		Scope:                     protocol.InputQueueScopeDM,
		SessionKey:                target.sessionKey,
		AgentID:                   target.agent.AgentID,
		TargetAgentIDs:            []string{target.agent.AgentID},
		OwnerUserID:               target.owner(),
		Source:                    protocol.InputQueueSourceHostRecovery,
		Content:                   content,
		DeliveryPolicy:            protocol.ChatDeliveryPolicyQueue,
		RecoveryExecutionID:       source.ExecutionID,
		RecoveryTarget:            source.Target,
		WorkBinding:               source.Work,
		ReviewBinding:             source.Review,
		ContinuationPauseRevision: source.PauseRevision,
	}
}

// acceptMainInboxIMReply 把 IM 回执写进主窗口 inbox。入队已存在却不再待处理，说明上一次可能已执行过，
// 不能推断可以再跑一次，交给人处理。
func (s *Service) acceptMainInboxIMReply(ctx context.Context, target mainInboxTarget, reply imdelivery.Reply) error {
	event, _, err := s.inbox.Append(ctx, inboxstore.Event{
		EventID:     newInboxEventID(),
		OwnerUserID: target.owner(),
		InboxKey:    target.sessionKey,
		Kind:        inboxstore.KindIMReply,
		DedupeKey:   hashedInboxDedupeKey("im_reply", reply.ID),
		Payload: inboxstore.Payload{
			Content: reply.ForwardedContent,
			IMReply: &inboxstore.IMReplySource{ReplyID: reply.ID},
		},
	})
	if err != nil {
		return err
	}
	if _, err = s.imReplies.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "pending", "accepted"); err != nil {
		return err
	}
	if event.Status != inboxstore.StatusPending {
		_, err = s.imReplies.TransitionReply(ctx, reply.OwnerUserID, reply.ID, "accepted", "needs_attention")
		return err
	}
	s.wakeMainInbox(target)
	return nil
}

// startIMReplyRound 先把事件记为 consumed 再起轮（至多一次）；起轮失败的回执交给人处理，继续取下一批。
func (s *Service) startIMReplyRound(ctx context.Context, target mainInboxTarget, roundID string, batch []inboxstore.Event) inboxStart {
	event := batch[0]
	if moved := s.consumeMainInbox(ctx, target, roundID, batch); len(moved) == 0 {
		return inboxStartNone
	}
	if event.Payload.IMReply == nil {
		return inboxStartNext
	}
	replyID := event.Payload.IMReply.ReplyID
	started, err := s.dispatchIMDeliveryReply(ctx, imReplyDispatch{
		sessionKey:  target.sessionKey,
		agentID:     target.agent.AgentID,
		ownerUserID: target.owner(),
		replyID:     replyID,
		content:     event.Payload.Content,
	})
	if err != nil {
		s.LoggerFor(ctx).Error("IM feedback dispatch requires attention", "reply_id", replyID, "err", err)
		_, _ = s.imReplies.TransitionReply(ctx, target.owner(), replyID, "accepted", "needs_attention")
	}
	if started {
		return inboxStartRound
	}
	return inboxStartNext
}

// consumeMainInbox 把已认领却不必起轮的事件收口为 consumed，返回真正迁移成功的 event_id。
func (s *Service) consumeMainInbox(ctx context.Context, target mainInboxTarget, roundID string, batch []inboxstore.Event) []string {
	moved, err := s.inbox.Transition(ctx, target.sessionKey, eventIDs(batch), inboxstore.StatusDelivering, inboxstore.StatusConsumed, roundID)
	if err != nil {
		s.LoggerFor(ctx).Error("主窗口 inbox 事件收口失败", "round_id", roundID, "err", err)
	}
	return moved
}

// hashedInboxDedupeKey 把任意长度的外部键压成定长，适配 dedupe_key 列宽，并按来源分出命名空间。
func hashedInboxDedupeKey(namespace string, key string) string {
	sum := sha256.Sum256([]byte(key))
	return namespace + ":" + hex.EncodeToString(sum[:16])
}
