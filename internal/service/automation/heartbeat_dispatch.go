package automation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/service/workspace"
)

func (s *Service) dispatchHeartbeat(agentID string, reason string) {
	ctx := context.Background()
	logger := s.loggerFor(ctx).With("agent_id", agentID, "reason", reason)
	sessionKey := automationexec.BuildMainSessionKey(agentID)
	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		logger.Error("heartbeat 状态初始化失败", "err", err)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		return
	}

	s.mu.Lock()
	if runtime := s.heartbeatState[agentID]; runtime != nil {
		if runtime.Running {
			s.mu.Unlock()
			logger.Warn("heartbeat 已在运行中，跳过重复触发")
			return
		}
		runtime.Running = true
		runtime.PendingWake = false
		state = runtime
	}
	s.mu.Unlock()
	immediateWakeRequests, deferredWakeRequests := s.takeWakeRequests(agentID, sessionKey)

	events, err := s.claimSystemEvents(ctx, agentID)
	if err != nil {
		logger.Error("heartbeat 拉取系统事件失败", "err", err)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		return
	}

	instruction, err := s.buildHeartbeatInstruction(ctx, agentID, events, immediateWakeRequests, deferredWakeRequests)
	if err != nil {
		logger.Error("heartbeat 构建指令失败", "event_count", len(events), "err", err)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		s.failEvents(events)
		return
	}
	if strings.TrimSpace(instruction) == "" {
		logger.Info("heartbeat 无可执行内容", "event_count", len(events))
		s.markEventsProcessed(events)
		s.finishHeartbeatRuntime(agentID, nil, nil, nil)
		return
	}

	roundID := s.idFactory("hbround")
	sink := automationexec.NewExecutionSink("heartbeat:" + agentID + ":" + roundID)
	cleanup := s.bindSink(sessionKey, sink)
	if err = s.dispatchToSession(ctx, sessionKey, roundID, agentID, instruction); err != nil {
		cleanup()
		sink.Close()
		s.failEvents(events)
		s.finishHeartbeatRuntime(agentID, nil, nil, errorPointer(err))
		logger.Error("heartbeat 下发失败",
			"session_key", sessionKey,
			"round_id", roundID,
			"event_count", len(events),
			"err", err,
		)
		return
	}
	logger.Info("heartbeat 已下发",
		"session_key", sessionKey,
		"round_id", roundID,
		"event_count", len(events),
	)

	startedAt := s.nowFn()
	s.mu.Lock()
	if runtime := s.heartbeatState[agentID]; runtime != nil {
		runtime.LastHeartbeatAt = cloneTimePointer(&startedAt)
		runtime.DeliveryError = nil
	}
	s.mu.Unlock()
	_ = s.persistHeartbeatTimes(ctx, agentID, &startedAt, nil)

	go func() {
		defer cleanup()
		defer sink.Close()

		waitCtx, cancel := context.WithTimeout(context.Background(), automationexec.WaitTimeout(0))
		defer cancel()
		observation := sink.WaitForRound(waitCtx, roundID)
		if observation.Status == automationdomain.RunStatusSucceeded {
			finishedAt := s.nowFn()
			s.markEventsProcessed(events)
			deliveryError := s.deliverHeartbeatObservation(agentID, state.Config, observation)
			if deliveryError != nil {
				logger.Error("heartbeat 执行完成但投递失败",
					"status", observation.Status,
					"message_count", observation.MessageCount,
					"delivery_error", *deliveryError,
				)
			} else {
				logger.Info("heartbeat 执行成功",
					"status", observation.Status,
					"message_count", observation.MessageCount,
				)
			}
			s.finishHeartbeatRuntime(agentID, &startedAt, &finishedAt, deliveryError)
			return
		}
		s.failEvents(events)
		if observation.ErrorMessage != nil {
			logger.Error("heartbeat 执行失败",
				"status", observation.Status,
				"message_count", observation.MessageCount,
				"err", *observation.ErrorMessage,
			)
		} else {
			logger.Error("heartbeat 执行失败",
				"status", observation.Status,
				"message_count", observation.MessageCount,
			)
		}
		s.finishHeartbeatRuntime(agentID, &startedAt, nil, observation.ErrorMessage)
	}()

	_ = reason
}

func (s *Service) buildHeartbeatInstruction(
	ctx context.Context,
	agentID string,
	events []automationdomain.SystemEvent,
	immediateWakeRequests []automationexec.HeartbeatWakeRequest,
	deferredWakeRequests []automationexec.HeartbeatWakeRequest,
) (string, error) {
	sections := make([]string, 0, 3)
	heartbeatSection, err := s.loadHeartbeatTaskSection(ctx, agentID)
	if err != nil {
		return "", err
	}
	sections = appendNonEmptySection(sections, heartbeatSection)
	eventLines := heartbeatEventLines(events)
	sections = appendBulletSection(sections, "System events", eventLines)
	wakeRequests := append(append([]automationexec.HeartbeatWakeRequest{}, immediateWakeRequests...), deferredWakeRequests...)
	sections = appendBulletSection(sections, "Wake requests", heartbeatWakeLines(eventLines, wakeRequests))
	sections = appendNonEmptySection(sections, s.describeScheduledTasksSection(ctx, agentID))
	return strings.TrimSpace(strings.Join(sections, "\n\n")), nil
}

func (s *Service) loadHeartbeatTaskSection(ctx context.Context, agentID string) (string, error) {
	if s.workspace == nil {
		return "", nil
	}
	file, err := s.workspace.GetFile(ctx, agentID, "HEARTBEAT.md")
	if err != nil && !errors.Is(err, workspacepkg.ErrFileNotFound) {
		return "", err
	}
	if file == nil || strings.TrimSpace(file.Content) == "" {
		return "", nil
	}
	tasks := automationexec.ParseHeartbeatTasks(file.Content)
	if len(tasks) == 0 {
		return strings.TrimSpace(file.Content), nil
	}
	return bulletSection("Heartbeat tasks", heartbeatTaskLines(tasks)), nil
}

func heartbeatTaskLines(tasks []automationexec.HeartbeatTask) []string {
	lines := make([]string, 0, len(tasks))
	for _, task := range tasks {
		line := firstNonEmpty(
			strings.TrimSpace(task.Prompt),
			strings.TrimSpace(task.Name),
			strings.TrimSpace(task.Interval),
		)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func heartbeatEventLines(events []automationdomain.SystemEvent) []string {
	lines := make([]string, 0, len(events))
	for _, event := range events {
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(event.Payload), &payload)
		lines = append(lines, firstNonEmpty(strings.TrimSpace(anyString(payload["text"])), event.EventType))
	}
	return lines
}

func heartbeatWakeLines(
	eventLines []string,
	requests []automationexec.HeartbeatWakeRequest,
) []string {
	seen := make(map[string]struct{}, len(eventLines)+len(requests))
	for _, line := range eventLines {
		seen[line] = struct{}{}
	}
	lines := make([]string, 0, len(requests))
	for _, request := range requests {
		line := heartbeatWakeLine(request)
		if _, exists := seen[line]; exists {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func heartbeatWakeLine(request automationexec.HeartbeatWakeRequest) string {
	if text := strings.TrimSpace(request.Text); text != "" {
		return text
	}
	mode := firstNonEmpty(strings.TrimSpace(request.WakeMode), "unknown")
	return "wake request (" + mode + ")"
}

func appendNonEmptySection(sections []string, section string) []string {
	if section := strings.TrimSpace(section); section != "" {
		return append(sections, section)
	}
	return sections
}

func appendBulletSection(sections []string, title string, lines []string) []string {
	return appendNonEmptySection(sections, bulletSection(title, lines))
}

func bulletSection(title string, lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(title) + ":\n- " + strings.Join(lines, "\n- ")
}

func (s *Service) claimSystemEvents(ctx context.Context, agentID string) ([]automationdomain.SystemEvent, error) {
	items, err := s.repository.ListNewSystemEventsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if markErr := s.repository.MarkSystemEventStatus(ctx, item.EventID, "processing"); markErr != nil {
			return nil, markErr
		}
	}
	return items, nil
}

func (s *Service) markEventsProcessed(items []automationdomain.SystemEvent) {
	for _, item := range items {
		_ = s.repository.MarkSystemEventStatus(context.Background(), item.EventID, "processed")
	}
}

func (s *Service) failEvents(items []automationdomain.SystemEvent) {
	for _, item := range items {
		_ = s.repository.MarkSystemEventStatus(context.Background(), item.EventID, "failed")
	}
}
