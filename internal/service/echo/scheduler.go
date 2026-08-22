// INPUT: 成功 DM round、Echo durable attempt、会话历史和当前 runtime 活跃状态。
// OUTPUT: 到期判断、轻量 Gate、无工具后台生成与最终单消息提交。
// POS: Echo 的事件驱动调度与发送闭环。
package echo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	echodomain "github.com/nexus-research-lab/nexus/internal/echo"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	messageutil "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/service/llm"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

const (
	echoPurpose       = "echo_followup"
	echoNoReplyMarker = "<nexus_echo_no_reply/>"
	echoHistoryLimit  = 12
	echoHistoryRunes  = 24_000
	echoRetryDelay    = 5 * time.Minute
	echoAttemptMaxAge = 7 * 24 * time.Hour

	gateFollowUp = "follow_up"
	gateSkip     = "skip"
)

type gateDecision struct {
	Decision   string `json:"decision"`
	ReasonCode string `json:"reason_code"`
	Focus      string `json:"focus"`
}

type eligibility struct {
	policy       echodomain.Policy
	history      []protocol.Message
	rescheduleAt *time.Time
	status       string
	reason       string
}

// OnTerminal 为一次成功且用户可见的 DM round 建立唯一空闲尝试。
func (s *Service) OnTerminal(ctx context.Context, terminal dmsvc.EchoTerminalRound) {
	ctx = ownerContext(context.WithoutCancel(ctx), terminal.OwnerUserID)
	policy, enabled, err := s.effectivePolicy(ctx)
	if err != nil {
		s.logger.Warn("Echo 读取策略失败", "session_key", terminal.SessionKey, "err", err)
		return
	}
	if !enabled {
		return
	}
	finishedAt := terminal.FinishedAt.UTC()
	if finishedAt.IsZero() {
		finishedAt = s.nowFn()
	}
	dueAt, err := nextActiveTime(finishedAt.Add(time.Duration(policy.IdleDelaySeconds)*time.Second), policy)
	if err != nil {
		s.logger.Warn("Echo 计算下次判断时间失败", "agent_id", terminal.AgentID, "err", err)
		return
	}
	created, err := s.repository.InsertAttempt(ctx, echodomain.Attempt{
		AttemptID:        newAttemptID(),
		OwnerUserID:      strings.TrimSpace(terminal.OwnerUserID),
		AgentID:          strings.TrimSpace(terminal.AgentID),
		SessionKey:       strings.TrimSpace(terminal.SessionKey),
		TriggerKind:      echodomain.TriggerConversationIdle,
		AnchorRoundID:    strings.TrimSpace(terminal.RoundID),
		AnchorMessageID:  strings.TrimSpace(terminal.AssistantID),
		AnchorFinishedAt: finishedAt,
		DueAt:            dueAt,
		ExpiresAt:        finishedAt.Add(echoAttemptMaxAge),
	})
	if err != nil {
		s.logger.Warn("Echo 保存待判断对话失败", "session_key", terminal.SessionKey, "err", err)
		return
	}
	if created {
		s.loop.Notify()
	}
}

func (s *Service) reconcile(ctx context.Context, now time.Time) (duework.Result, error) {
	attempt, err := s.repository.ClaimDue(ctx, now)
	if err != nil {
		return duework.Result{}, err
	}
	if attempt != nil {
		if err = s.processAttempt(ownerContext(ctx, attempt.OwnerUserID), *attempt, now); err != nil {
			s.logger.Warn("Echo 尝试处理失败", "attempt_id", attempt.AttemptID, "err", err)
		}
	}
	nextDueAt, err := s.repository.NextDueAt(ctx)
	if err != nil {
		return duework.Result{}, err
	}
	return duework.Result{
		HasMore:   nextDueAt != nil && !nextDueAt.After(s.nowFn()),
		NextDueAt: nextDueAt,
	}, nil
}

func (s *Service) processAttempt(
	ctx context.Context,
	attempt echodomain.Attempt,
	now time.Time,
) error {
	check, err := s.checkEligibility(ctx, attempt, now, false)
	if err != nil {
		return s.finishAttemptError(ctx, attempt.AttemptID, err)
	}
	if check.rescheduleAt != nil {
		if !attempt.ExpiresAt.IsZero() && !check.rescheduleAt.Before(attempt.ExpiresAt) {
			return s.repository.FinishWithoutDelivery(
				ctx,
				attempt.AttemptID,
				echodomain.StatusCancelled,
				"expired",
				"",
			)
		}
		return s.repository.Reschedule(ctx, attempt.AttemptID, *check.rescheduleAt, check.reason)
	}
	if check.status != "" {
		return s.repository.FinishWithoutDelivery(ctx, attempt.AttemptID, check.status, check.reason, "")
	}
	decision, err := s.evaluateConversation(ctx, attempt, check.history)
	if err != nil {
		s.logger.Warn("Echo Gate 不可用", "attempt_id", attempt.AttemptID)
		return s.repository.FinishWithoutDelivery(
			ctx,
			attempt.AttemptID,
			echodomain.StatusFailed,
			"gate_unavailable",
			"",
		)
	}
	if decision.Decision != gateFollowUp {
		return s.repository.FinishWithoutDelivery(
			ctx,
			attempt.AttemptID,
			echodomain.StatusSuppressed,
			decision.ReasonCode,
			"",
		)
	}
	roundID := protocol.NewRoundID()
	marked, err := s.repository.MarkRunning(
		ctx,
		attempt.AttemptID,
		roundID,
		decision.ReasonCode,
		decision.Focus,
	)
	if err != nil {
		return err
	}
	if !marked {
		return nil
	}
	return s.startFollowUp(ctx, attempt, roundID, decision.Focus)
}

func (s *Service) startFollowUp(
	ctx context.Context,
	attempt echodomain.Attempt,
	roundID string,
	focus string,
) error {
	ownerCtx := ownerContext(context.WithoutCancel(ctx), attempt.OwnerUserID)
	request := dmsvc.Request{
		SessionKey:      attempt.SessionKey,
		AgentID:         attempt.AgentID,
		Content:         buildFollowUpPrompt(focus),
		RoundID:         roundID,
		Internal:        true,
		ExecutionOrigin: "echo",
		InputOptions: sdkprotocol.OutboundMessageOptions{
			HiddenFromUser:  true,
			Synthetic:       true,
			Purpose:         echoPurpose,
			ToolAccess:      "none",
			MaxOutputTokens: 400,
			Metadata: map[string]string{
				"echo_attempt_id":      attempt.AttemptID,
				"echo_anchor_round_id": attempt.AnchorRoundID,
			},
		},
		StartAdmission: func(context.Context) error {
			return s.admitStart(ownerCtx, attempt, roundID)
		},
		DeferredAssistant: &dmsvc.DeferredAssistantHooks{
			Admit: func(_ context.Context, candidate dmsvc.DeferredAssistantCandidate) (protocol.Message, bool, error) {
				return s.admitMessage(ownerCtx, attempt, roundID, candidate)
			},
			Complete: func(_ context.Context, outcome dmsvc.DeferredAssistantOutcome) {
				s.completeAttempt(ownerCtx, attempt.AttemptID, outcome)
			},
		},
	}
	if err := s.dm.HandleChat(ownerCtx, request); err != nil {
		return s.finishAttemptError(ownerCtx, attempt.AttemptID, err)
	}
	return nil
}

func (s *Service) admitStart(
	ctx context.Context,
	attempt echodomain.Attempt,
	roundID string,
) error {
	current, err := s.runningAttempt(ctx, attempt.AttemptID, roundID)
	if err != nil {
		return err
	}
	check, err := s.checkEligibility(ctx, *current, s.nowFn(), true)
	if err != nil {
		return err
	}
	if check.status != "" || check.rescheduleAt != nil {
		return echodomain.ErrAttemptNotAdmitted
	}
	return nil
}

func (s *Service) admitMessage(
	ctx context.Context,
	attempt echodomain.Attempt,
	roundID string,
	candidate dmsvc.DeferredAssistantCandidate,
) (protocol.Message, bool, error) {
	if candidate.TerminalStatus != "finished" ||
		(candidate.ResultSubtype != "" && candidate.ResultSubtype != "success") {
		return nil, false, fmt.Errorf("Echo 生成失败: %s", firstNonEmpty(candidate.ErrorMessage, candidate.ResultSubtype))
	}
	text := messageutil.ExtractAssistantDisplayText(candidate.Message)
	if strings.TrimSpace(text) == echoNoReplyMarker || strings.TrimSpace(text) == "" {
		err := s.repository.FinishWithoutDelivery(
			ctx,
			attempt.AttemptID,
			echodomain.StatusSuppressed,
			"generation_declined",
			"",
		)
		return nil, false, err
	}
	current, err := s.runningAttempt(ctx, attempt.AttemptID, roundID)
	if err != nil {
		return nil, false, err
	}
	check, err := s.checkEligibility(ctx, *current, s.nowFn(), true)
	if err != nil {
		return nil, false, err
	}
	if check.status != "" || check.rescheduleAt != nil {
		status := firstNonEmpty(check.status, echodomain.StatusSuppressed)
		if finishErr := s.repository.FinishWithoutDelivery(ctx, attempt.AttemptID, status, check.reason, ""); finishErr != nil {
			return nil, false, finishErr
		}
		return nil, false, nil
	}
	message := protocol.Clone(candidate.Message)
	messageID := messageString(message, "message_id")
	if messageID == "" {
		messageID = protocol.NewAssistantMessageID()
		message["message_id"] = messageID
	}
	metadata := make(map[string]any)
	if currentMetadata, ok := message["metadata"].(map[string]any); ok {
		for key, value := range currentMetadata {
			metadata[key] = value
		}
	}
	metadata["source"] = "echo"
	metadata["echo_attempt_id"] = attempt.AttemptID
	metadata["echo_anchor_round_id"] = attempt.AnchorRoundID
	message["metadata"] = metadata
	admitted, err := s.repository.AdmitCommit(ctx, attempt.AttemptID, messageID)
	if err != nil {
		return nil, false, err
	}
	if !admitted {
		return nil, false, echodomain.ErrAttemptNotAdmitted
	}
	return message, true, nil
}

func (s *Service) runningAttempt(
	ctx context.Context,
	attemptID string,
	roundID string,
) (*echodomain.Attempt, error) {
	current, err := s.repository.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}
	if current == nil || current.Status != echodomain.StatusRunning || current.RuntimeRoundID != roundID {
		return nil, echodomain.ErrAttemptNotAdmitted
	}
	return current, nil
}

func (s *Service) completeAttempt(
	ctx context.Context,
	attemptID string,
	outcome dmsvc.DeferredAssistantOutcome,
) {
	var err error
	if outcome.Status == echodomain.StatusDelivered {
		err = s.repository.FinishCommit(ctx, attemptID, nil)
	} else {
		reason := firstNonEmpty(outcome.Status, echodomain.StatusFailed)
		errorCode := ""
		if outcome.Error != nil {
			errorCode = "runtime_failed"
		}
		err = s.repository.FinishWithoutDelivery(ctx, attemptID, reason, reason, errorCode)
		if err == nil && outcome.Status == echodomain.StatusFailed {
			err = s.repository.FinishCommit(ctx, attemptID, outcome.Error)
		}
	}
	if err != nil {
		s.logger.Warn("Echo 尝试收口失败", "attempt_id", attemptID, "err", err)
	}
}

func (s *Service) finishAttemptError(ctx context.Context, attemptID string, attemptErr error) error {
	if attemptErr == nil {
		return nil
	}
	if err := s.repository.FinishWithoutDelivery(
		ctx,
		attemptID,
		echodomain.StatusFailed,
		"internal_error",
		"internal_error",
	); err != nil {
		return errors.Join(attemptErr, err)
	}
	return attemptErr
}

func (s *Service) checkEligibility(
	ctx context.Context,
	attempt echodomain.Attempt,
	now time.Time,
	allowOwnRound bool,
) (eligibility, error) {
	if !attempt.ExpiresAt.IsZero() && !now.Before(attempt.ExpiresAt) {
		return eligibility{status: echodomain.StatusCancelled, reason: "expired"}, nil
	}
	policy, enabled, err := s.effectivePolicy(ctx)
	if err != nil {
		return eligibility{}, err
	}
	if !enabled {
		return eligibility{status: echodomain.StatusCancelled, reason: "disabled"}, nil
	}
	active, nextActiveAt, err := activeWindow(now, policy)
	if err != nil {
		return eligibility{}, err
	}
	if !active {
		if allowOwnRound {
			return eligibility{status: echodomain.StatusSuppressed, reason: "outside_active_hours"}, nil
		}
		return eligibility{policy: policy, rescheduleAt: &nextActiveAt, reason: "outside_active_hours"}, nil
	}
	session, err := s.sessions.GetSession(ctx, attempt.SessionKey)
	if err != nil {
		return eligibility{}, err
	}
	if session == nil || session.AgentID != attempt.AgentID ||
		protocol.NormalizeSessionKeyChannelSegment(session.ChannelType) != protocol.SessionChannelWebSocketSegment ||
		strings.TrimSpace(session.ChatType) != protocol.RoomTypeDM {
		return eligibility{status: echodomain.StatusCancelled, reason: "unsupported_session"}, nil
	}
	history, err := s.sessions.GetSessionMessages(ctx, attempt.SessionKey)
	if err != nil {
		return eligibility{}, err
	}
	running := s.runtime.GetRunningRoundIDs(attempt.SessionKey)
	if allowOwnRound {
		if len(running) != 1 || running[0] != strings.TrimSpace(attempt.RuntimeRoundID) ||
			s.runtime.CountRunningRounds(attempt.AgentID) != 1 {
			return eligibility{status: echodomain.StatusCancelled, reason: "agent_busy"}, nil
		}
	} else if len(running) > 0 || s.runtime.CountRunningRounds(attempt.AgentID) > 0 {
		retryAt := now.Add(echoRetryDelay)
		return eligibility{policy: policy, rescheduleAt: &retryAt, reason: "agent_busy"}, nil
	}
	otherActive, err := s.repository.HasOtherActiveForAgent(ctx, attempt.OwnerUserID, attempt.AgentID, attempt.AttemptID)
	if err != nil {
		return eligibility{}, err
	}
	if otherActive {
		if allowOwnRound {
			return eligibility{status: echodomain.StatusCancelled, reason: "another_echo_active"}, nil
		}
		retryAt := now.Add(echoRetryDelay)
		return eligibility{policy: policy, rescheduleAt: &retryAt, reason: "another_echo_active"}, nil
	}
	lastDeliveredAt, err := s.repository.LastDeliveredAtForSession(ctx, attempt.OwnerUserID, attempt.SessionKey)
	if err != nil {
		return eligibility{}, err
	}
	if lastDeliveredAt != nil {
		cooldownEndsAt := lastDeliveredAt.Add(time.Duration(policy.CooldownSeconds) * time.Second)
		if cooldownEndsAt.After(now) {
			if allowOwnRound {
				return eligibility{status: echodomain.StatusSuppressed, reason: "cooldown"}, nil
			}
			dueAt, timeErr := nextActiveTime(cooldownEndsAt, policy)
			if timeErr != nil {
				return eligibility{}, timeErr
			}
			return eligibility{policy: policy, rescheduleAt: &dueAt, reason: "cooldown"}, nil
		}
	}
	dayStart, nextDay, err := localDayBounds(now, policy.Timezone)
	if err != nil {
		return eligibility{}, err
	}
	deliveredToday, err := s.repository.CountDeliveredSince(ctx, attempt.OwnerUserID, dayStart)
	if err != nil {
		return eligibility{}, err
	}
	if deliveredToday >= policy.DailyLimit {
		if allowOwnRound {
			return eligibility{status: echodomain.StatusSuppressed, reason: "daily_limit"}, nil
		}
		dueAt, timeErr := nextActiveTime(nextDay, policy)
		if timeErr != nil {
			return eligibility{}, timeErr
		}
		return eligibility{policy: policy, rescheduleAt: &dueAt, reason: "daily_limit"}, nil
	}
	return eligibility{policy: policy, history: history}, nil
}

func (s *Service) effectivePolicy(
	ctx context.Context,
) (echodomain.Policy, bool, error) {
	policy, err := s.globalPolicy(ctx, authctx.OwnerUserID(ctx))
	if err != nil {
		return echodomain.Policy{}, false, err
	}
	return policy, policy.Enabled, nil
}

func (s *Service) evaluateConversation(
	ctx context.Context,
	attempt echodomain.Attempt,
	history []protocol.Message,
) (gateDecision, error) {
	config, err := s.resolveGateConfig(ctx, attempt.OwnerUserID, attempt.AgentID)
	if err != nil {
		return gateDecision{}, err
	}
	raw, err := s.llmClient.GenerateText(ctx, llm.GenerateTextRequest{
		Config: config,
		System: "你是 Echo 的发送门。判断这段私聊是否存在自然、具体、对用户有价值且值得现在主动跟进的一件事。避免泛泛问候、催促、重复答案、营销语气和无新信息的提醒。涉及高风险或敏感建议、需要工具或外部信息时必须跳过。只返回严格 JSON。",
		Messages: []llm.Message{{
			Role: "user",
			Content: "最近对话：\n" + buildGateHistory(history) +
				"\n\n只返回三个字段：decision 为 follow_up 或 skip；follow_up 的 reason_code 只能是 awaiting_answer、promised_followup、unfinished_decision、requested_check_in，且 focus 必须是一句不超过 160 字的具体关注点；skip 的 reason_code 只能是 concluded、no_new_value、would_repeat、social_only、too_ambiguous、sensitive_context、needs_tool，focus 为空字符串。",
		}},
		MaxTokens:        220,
		Temperature:      0,
		DisableReasoning: true,
	})
	if err != nil {
		return gateDecision{}, err
	}
	decision, err := parseGateDecision(raw)
	if err != nil {
		return gateDecision{Decision: gateSkip, ReasonCode: "invalid_output"}, nil
	}
	return decision, nil
}

func (s *Service) resolveGateConfig(
	ctx context.Context,
	ownerUserID string,
	agentID string,
) (*clientopts.RuntimeConfig, error) {
	prefs, err := s.prefs.Get(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	provider := strings.TrimSpace(prefs.DefaultBackgroundModelSelection.Provider)
	model := strings.TrimSpace(prefs.DefaultBackgroundModelSelection.Model)
	if provider == "" || model == "" {
		agent, agentErr := s.agents.GetAgent(ctx, agentID)
		if agentErr != nil {
			return nil, agentErr
		}
		provider = strings.TrimSpace(agent.Options.Provider)
		model = strings.TrimSpace(agent.Options.Model)
	}
	return s.providers.ResolveLLMConfig(ctx, provider, model)
}

func parseGateDecision(raw string) (gateDecision, error) {
	var decision gateDecision
	decoder := json.NewDecoder(strings.NewReader(trimGateJSONFence(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decision); err != nil {
		return gateDecision{}, fmt.Errorf("Echo Gate 返回格式无效: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return gateDecision{}, errors.New("Echo Gate 包含多余内容")
	}
	decision.Decision = strings.TrimSpace(decision.Decision)
	decision.ReasonCode = strings.TrimSpace(decision.ReasonCode)
	decision.Focus = strings.TrimSpace(decision.Focus)
	if !validGateReason(decision.Decision, decision.ReasonCode) {
		return gateDecision{}, errors.New("Echo Gate 决定或原因无效")
	}
	if decision.Decision == gateFollowUp {
		if decision.Focus == "" {
			return gateDecision{}, errors.New("Echo Gate 缺少具体关注点")
		}
		if len([]rune(decision.Focus)) > 160 {
			return gateDecision{}, errors.New("Echo Gate 关注点过长")
		}
	} else {
		decision.Focus = ""
	}
	return decision, nil
}

func trimGateJSONFence(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, prefix := range []string{"```json\n", "```\n"} {
		if strings.HasPrefix(raw, prefix) && strings.HasSuffix(raw, "\n```") {
			return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, prefix), "\n```"))
		}
	}
	return raw
}

func validGateReason(decision string, reason string) bool {
	switch decision {
	case gateFollowUp:
		switch reason {
		case "awaiting_answer", "promised_followup", "unfinished_decision", "requested_check_in":
			return true
		}
	case gateSkip:
		switch reason {
		case "concluded", "no_new_value", "would_repeat", "social_only", "too_ambiguous", "sensitive_context", "needs_tool":
			return true
		}
	}
	return false
}

func buildGateHistory(messages []protocol.Message) string {
	lines := make([]string, 0, echoHistoryLimit)
	for index := len(messages) - 1; index >= 0 && len(lines) < echoHistoryLimit; index-- {
		message := messages[index]
		role := protocol.MessageRole(message)
		if role != "user" && role != "assistant" || message["hidden_from_user"] == true {
			continue
		}
		content := messageText(message)
		if content == "" {
			continue
		}
		label := "用户"
		if role == "assistant" {
			label = "Agent"
		}
		lines = append(lines, label+"："+content)
	}
	for left, right := 0, len(lines)-1; left < right; left, right = left+1, right-1 {
		lines[left], lines[right] = lines[right], lines[left]
	}
	value := strings.Join(lines, "\n")
	runes := []rune(value)
	if len(runes) > echoHistoryRunes {
		value = string(runes[len(runes)-echoHistoryRunes:])
	}
	return value
}

func messageText(message protocol.Message) string {
	if value, ok := message["content"].(string); ok {
		return strings.TrimSpace(value)
	}
	var parts []string
	switch content := message["content"].(type) {
	case []map[string]any:
		for _, block := range content {
			if text := messageString(block, "text"); text != "" {
				parts = append(parts, text)
			}
		}
	case []any:
		for _, raw := range content {
			if block, ok := raw.(map[string]any); ok {
				if text := messageString(block, "text"); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func buildFollowUpPrompt(focus string) string {
	return fmt.Sprintf(`你正在为当前私聊生成一次 Echo 主动跟进。会话已经安静下来；这不是对用户新消息的即时回复，而是你主动把一件仍值得继续的事自然接回来。

内部关注点（只作方向，不要照抄，也不要当成用户事实）：
%s

请像一个真正记得前文的人重新开口：
- 开头先自然承接上文。可以用一句贴合语境的轻量寒暄、回想或过渡，例如“对了……”“顺着刚才那件事……”“我又想了一下……”，但不要固定套用模板。
- 随后主动带来一个具体的新价值，例如更清楚的判断、可执行的小建议、能推动决定的具体问题，或对先前承诺的自然兑现。
- 保持该 Agent 的人物性格和会话语言；语气亲切、松弛，不像通知、催办或客服回访。
- 只发送一条简短消息，通常 2 到 5 句；没有必要时不要使用标题、清单或总结腔。
- 不要说“你还没回复”，不要责备或制造紧迫感，不要泛泛询问“还需要帮助吗”。不要提及 Echo、后台判断、等待时长或系统规则；不要执行任何工具或操作，也不要编造新事实。

如果不能在不依赖工具和新信息的前提下自然增加价值，或只能生硬地重新提问，只输出 %s`, strings.TrimSpace(focus), echoNoReplyMarker)
}

func activeWindow(now time.Time, policy echodomain.Policy) (bool, time.Time, error) {
	location, err := time.LoadLocation(policy.Timezone)
	if err != nil {
		return false, time.Time{}, err
	}
	startMinute, err := echodomain.ParseClock(policy.ActiveStart)
	if err != nil {
		return false, time.Time{}, err
	}
	endMinute, err := echodomain.ParseClock(policy.ActiveEnd)
	if err != nil {
		return false, time.Time{}, err
	}
	local := now.In(location)
	startToday := time.Date(local.Year(), local.Month(), local.Day(), startMinute/60, startMinute%60, 0, 0, location)
	endToday := time.Date(local.Year(), local.Month(), local.Day(), endMinute/60, endMinute%60, 0, 0, location)
	if startMinute == endMinute {
		return true, now.UTC(), nil
	}
	if startMinute < endMinute {
		if !local.Before(startToday) && local.Before(endToday) {
			return true, now.UTC(), nil
		}
		if local.Before(startToday) {
			return false, startToday.UTC(), nil
		}
		return false, startToday.AddDate(0, 0, 1).UTC(), nil
	}
	if !local.Before(startToday) || local.Before(endToday) {
		return true, now.UTC(), nil
	}
	return false, startToday.UTC(), nil
}

func nextActiveTime(candidate time.Time, policy echodomain.Policy) (time.Time, error) {
	active, next, err := activeWindow(candidate, policy)
	if err != nil {
		return time.Time{}, err
	}
	if active {
		return candidate.UTC(), nil
	}
	return next.UTC(), nil
}

func localDayBounds(now time.Time, timezone string) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

func newAttemptID() string {
	return "echo_" + strings.TrimPrefix(protocol.NewRoundID(), "round_")
}

func messageString(message map[string]any, key string) string {
	value, _ := message[key].(string)
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
