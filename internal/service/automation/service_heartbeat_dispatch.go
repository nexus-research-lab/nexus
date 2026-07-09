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
	if s.workspace != nil {
		file, err := s.workspace.GetFile(ctx, agentID, "HEARTBEAT.md")
		if err != nil && !errors.Is(err, workspacepkg.ErrFileNotFound) {
			return "", err
		}
		if file != nil && strings.TrimSpace(file.Content) != "" {
			tasks := automationexec.ParseHeartbeatTasks(file.Content)
			if len(tasks) > 0 {
				taskLines := make([]string, 0, len(tasks))
				for _, item := range tasks {
					line := firstNonEmpty(
						strings.TrimSpace(item.Prompt),
						strings.TrimSpace(item.Name),
						strings.TrimSpace(item.Interval),
					)
					if line != "" {
						taskLines = append(taskLines, line)
					}
				}
				if len(taskLines) > 0 {
					sections = append(sections, "Heartbeat tasks:\n- "+strings.Join(taskLines, "\n- "))
				}
			} else {
				sections = append(sections, strings.TrimSpace(file.Content))
			}
		}
	}

	eventLines := make([]string, 0, len(events))
	for _, item := range events {
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(item.Payload), &payload)
		text := strings.TrimSpace(anyString(payload["text"]))
		if text != "" {
			eventLines = append(eventLines, text)
			continue
		}
		eventLines = append(eventLines, item.EventType)
	}
	if len(eventLines) > 0 {
		sections = append(sections, "System events:\n- "+strings.Join(eventLines, "\n- "))
	}

	existingLines := make(map[string]struct{}, len(eventLines))
	for _, item := range eventLines {
		existingLines[item] = struct{}{}
	}
	wakeLines := make([]string, 0, len(immediateWakeRequests)+len(deferredWakeRequests))
	appendWakeLine := func(request automationexec.HeartbeatWakeRequest) {
		text := strings.TrimSpace(request.Text)
		if text != "" {
			if _, duplicated := existingLines[text]; duplicated {
				return
			}
			wakeLines = append(wakeLines, text)
			existingLines[text] = struct{}{}
			return
		}
		fallback := "wake request (" + strings.TrimSpace(request.WakeMode) + ")"
		if strings.TrimSpace(request.WakeMode) == "" {
			fallback = "wake request (unknown)"
		}
		if _, duplicated := existingLines[fallback]; duplicated {
			return
		}
		wakeLines = append(wakeLines, fallback)
		existingLines[fallback] = struct{}{}
	}
	for _, item := range immediateWakeRequests {
		appendWakeLine(item)
	}
	for _, item := range deferredWakeRequests {
		appendWakeLine(item)
	}
	if len(wakeLines) > 0 {
		sections = append(sections, "Wake requests:\n- "+strings.Join(wakeLines, "\n- "))
	}
	if summary := s.describeScheduledTasksSection(ctx, agentID); summary != "" {
		sections = append(sections, summary)
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n")), nil
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
