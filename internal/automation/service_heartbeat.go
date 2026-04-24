package automation

import (
	"context"
	"encoding/json"
	"errors"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/workspace"
	"strings"
	"time"
)

const heartbeatExplicitTargetUnsupportedMessage = "heartbeat target_mode=explicit is not supported in Task 6 runtime"

// GetHeartbeatStatus 返回 heartbeat 状态。
func (s *Service) GetHeartbeatStatus(ctx context.Context, agentID string) (*HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	return &HeartbeatStatus{
		AgentID:         state.Config.AgentID,
		Enabled:         state.Config.Enabled,
		EverySeconds:    state.Config.EverySeconds,
		TargetMode:      state.Config.TargetMode,
		AckMaxChars:     state.Config.AckMaxChars,
		Running:         state.Running,
		PendingWake:     state.PendingWake,
		NextRunAt:       cloneTimePointer(state.NextRunAt),
		LastHeartbeatAt: cloneTimePointer(state.LastHeartbeatAt),
		LastAckAt:       cloneTimePointer(state.LastAckAt),
		DeliveryError:   cloneStringPointer(state.DeliveryError),
	}, nil
}

// UpdateHeartbeat 更新 heartbeat 配置。
func (s *Service) UpdateHeartbeat(ctx context.Context, agentID string, input HeartbeatUpdateInput) (*HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	configValue := HeartbeatConfig{
		AgentID:      strings.TrimSpace(agentID),
		Enabled:      input.Enabled,
		EverySeconds: input.EverySeconds,
		TargetMode:   strings.TrimSpace(input.TargetMode),
		AckMaxChars:  input.AckMaxChars,
	}.Normalized()
	if configValue.TargetMode == HeartbeatTargetExplicit {
		return nil, ErrHeartbeatConfigInvalid
	}
	if err := configValue.Validate(); err != nil {
		return nil, err
	}

	state, err := s.ensureHeartbeatState(ctx, configValue.AgentID)
	if err != nil {
		return nil, err
	}
	state.Config = configValue
	state.NextRunAt = s.computeHeartbeatNext(configValue, s.nowFn())
	state.DeliveryError = nil
	if !configValue.Enabled {
		state.PendingWake = false
		state.Running = false
	}
	if err = s.repository.UpsertHeartbeatState(
		ctx,
		s.idFactory("hb"),
		configValue,
		state.LastHeartbeatAt,
		state.LastAckAt,
	); err != nil {
		return nil, err
	}
	return s.GetHeartbeatStatus(ctx, configValue.AgentID)
}

// WakeHeartbeat 手动登记一次 heartbeat 唤醒。
func (s *Service) WakeHeartbeat(ctx context.Context, agentID string, request HeartbeatWakeRequest) (*HeartbeatWakeResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	mode := strings.TrimSpace(request.Mode)
	if mode == "" {
		mode = WakeModeNow
	}
	if mode != WakeModeNow && mode != WakeModeNextHeartbeat {
		return nil, errors.New("mode must be one of now, next-heartbeat")
	}

	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if request.Text != nil && strings.TrimSpace(*request.Text) != "" {
		if err = s.repository.InsertSystemEvent(
			ctx,
			s.idFactory("evt"),
			"heartbeat.wake",
			"heartbeat",
			state.Config.AgentID,
			map[string]any{
				"agent_id":  state.Config.AgentID,
				"text":      strings.TrimSpace(*request.Text),
				"wake_mode": mode,
			},
		); err != nil {
			return nil, err
		}
	}
	sessionKey := buildMainSessionKey(state.Config.AgentID)
	s.recordWakeRequest(state.Config.AgentID, sessionKey, mode, request.Text)

	s.mu.Lock()
	switch mode {
	case WakeModeNow:
		if state.Running {
			state.PendingWake = true
			s.mu.Unlock()
			return &HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: true}, nil
		}
		state.PendingWake = true
		s.mu.Unlock()
		s.dispatchHeartbeat(state.Config.AgentID, "wake-now")
		return &HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: true}, nil
	default:
		state.PendingWake = true
		s.mu.Unlock()
		return &HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: false}, nil
	}
}

func (s *Service) dispatchHeartbeat(agentID string, reason string) {
	ctx := context.Background()
	logger := s.loggerFor(ctx).With("agent_id", agentID, "reason", reason)
	sessionKey := buildMainSessionKey(agentID)
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
	sink := newExecutionSink("heartbeat:" + agentID + ":" + roundID)
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

		waitCtx, cancel := context.WithTimeout(context.Background(), waitTimeout(0))
		defer cancel()
		observation := sink.WaitForRound(waitCtx, roundID)
		if observation.Status == RunStatusSucceeded {
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
	events []SystemEvent,
	immediateWakeRequests []heartbeatWakeRequest,
	deferredWakeRequests []heartbeatWakeRequest,
) (string, error) {
	sections := make([]string, 0, 3)
	if s.workspace != nil {
		file, err := s.workspace.GetFile(ctx, agentID, "HEARTBEAT.md")
		if err != nil && !errors.Is(err, workspacepkg.ErrFileNotFound) {
			return "", err
		}
		if file != nil && strings.TrimSpace(file.Content) != "" {
			tasks := parseHeartbeatTasks(file.Content)
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
	appendWakeLine := func(request heartbeatWakeRequest) {
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

func (s *Service) claimSystemEvents(ctx context.Context, agentID string) ([]SystemEvent, error) {
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

func (s *Service) markEventsProcessed(items []SystemEvent) {
	for _, item := range items {
		_ = s.repository.MarkSystemEventStatus(context.Background(), item.EventID, "processed")
	}
}

func (s *Service) failEvents(items []SystemEvent) {
	for _, item := range items {
		_ = s.repository.MarkSystemEventStatus(context.Background(), item.EventID, "failed")
	}
}

func (s *Service) ensureHeartbeatState(ctx context.Context, agentID string) (*heartbeatRuntimeState, error) {
	s.mu.Lock()
	state := s.heartbeatState[strings.TrimSpace(agentID)]
	s.mu.Unlock()
	if state != nil {
		return state, nil
	}

	configValue, lastHeartbeatAt, lastAckAt, err := s.repository.GetHeartbeatState(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if configValue == nil {
		defaultValue := DefaultHeartbeatConfig(agentID)
		sanitizedConfig, deliveryError := sanitizeHeartbeatConfig(defaultValue)
		state = &heartbeatRuntimeState{
			Config:          sanitizedConfig,
			NextRunAt:       s.computeHeartbeatNext(sanitizedConfig, s.nowFn()),
			LastHeartbeatAt: cloneTimePointer(lastHeartbeatAt),
			LastAckAt:       cloneTimePointer(lastAckAt),
			DeliveryError:   cloneStringPointer(deliveryError),
		}
	} else {
		normalized, deliveryError := sanitizeHeartbeatConfig(configValue.Normalized())
		state = &heartbeatRuntimeState{
			Config:          normalized,
			NextRunAt:       s.computeHeartbeatNext(normalized, s.nowFn()),
			LastHeartbeatAt: cloneTimePointer(lastHeartbeatAt),
			LastAckAt:       cloneTimePointer(lastAckAt),
			DeliveryError:   cloneStringPointer(deliveryError),
		}
	}

	s.mu.Lock()
	s.heartbeatState[state.Config.AgentID] = state
	s.mu.Unlock()
	return state, nil
}

func (s *Service) computeHeartbeatNext(configValue HeartbeatConfig, now time.Time) *time.Time {
	if !configValue.Enabled {
		return nil
	}
	next := now.UTC().Add(time.Duration(configValue.EverySeconds) * time.Second)
	return &next
}

func (s *Service) finishHeartbeatRuntime(agentID string, startedAt *time.Time, ackAt *time.Time, deliveryError *string) {
	s.mu.Lock()
	state := s.heartbeatState[strings.TrimSpace(agentID)]
	if state != nil {
		state.Running = false
		state.NextRunAt = s.computeHeartbeatNext(state.Config, s.nowFn())
		if startedAt != nil {
			state.LastHeartbeatAt = cloneTimePointer(startedAt)
		}
		if ackAt != nil {
			state.LastAckAt = cloneTimePointer(ackAt)
		}
		state.DeliveryError = cloneStringPointer(deliveryError)
	}
	configValue := HeartbeatConfig{}
	lastHeartbeatAt := (*time.Time)(nil)
	lastAckAt := (*time.Time)(nil)
	if state != nil {
		configValue = state.Config
		lastHeartbeatAt = cloneTimePointer(state.LastHeartbeatAt)
		lastAckAt = cloneTimePointer(state.LastAckAt)
	}
	s.mu.Unlock()

	if configValue.AgentID != "" {
		_ = s.repository.UpsertHeartbeatState(context.Background(), s.idFactory("hb"), configValue, lastHeartbeatAt, lastAckAt)
	}
}

func (s *Service) persistHeartbeatTimes(ctx context.Context, agentID string, lastHeartbeatAt *time.Time, lastAckAt *time.Time) error {
	state, err := s.ensureHeartbeatState(ctx, agentID)
	if err != nil {
		return err
	}
	return s.repository.UpsertHeartbeatState(ctx, s.idFactory("hb"), state.Config, lastHeartbeatAt, lastAckAt)
}

func (s *Service) recordWakeRequest(agentID string, sessionKey string, wakeMode string, text *string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionKey = strings.TrimSpace(sessionKey)
	request := heartbeatWakeRequest{
		AgentID:    strings.TrimSpace(agentID),
		SessionKey: sessionKey,
		WakeMode:   strings.TrimSpace(wakeMode),
		Text:       strings.TrimSpace(anyStringPointer(text)),
	}
	s.wakeRequests[sessionKey] = append(s.wakeRequests[sessionKey], request)
	if state := s.heartbeatState[request.AgentID]; state != nil {
		state.PendingWake = true
	}
}

func (s *Service) hasImmediateWakeRequestLocked(agentID string) bool {
	sessionKey := buildMainSessionKey(agentID)
	for _, item := range s.wakeRequests[sessionKey] {
		if strings.TrimSpace(item.AgentID) == strings.TrimSpace(agentID) && item.WakeMode == WakeModeNow {
			return true
		}
	}
	return false
}

func (s *Service) takeWakeRequests(agentID string, sessionKey string) ([]heartbeatWakeRequest, []heartbeatWakeRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionKey = strings.TrimSpace(sessionKey)
	items := append([]heartbeatWakeRequest(nil), s.wakeRequests[sessionKey]...)
	delete(s.wakeRequests, sessionKey)

	immediate := make([]heartbeatWakeRequest, 0, len(items))
	deferred := make([]heartbeatWakeRequest, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.AgentID) != strings.TrimSpace(agentID) {
			continue
		}
		switch item.WakeMode {
		case WakeModeNow:
			immediate = append(immediate, item)
		case WakeModeNextHeartbeat:
			deferred = append(deferred, item)
		}
	}
	return immediate, deferred
}

func sanitizeHeartbeatConfig(configValue HeartbeatConfig) (HeartbeatConfig, *string) {
	result := configValue
	if strings.TrimSpace(result.TargetMode) != HeartbeatTargetExplicit {
		return result, nil
	}
	result.TargetMode = HeartbeatTargetNone
	return result, stringPointer(heartbeatExplicitTargetUnsupportedMessage)
}
