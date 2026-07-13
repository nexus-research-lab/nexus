package automation

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
)

const heartbeatExplicitTargetUnsupportedMessage = "heartbeat target_mode=explicit is not supported in Task 6 runtime"

func (s *Service) ensureHeartbeatState(ctx context.Context, agentID string) (*automationexec.HeartbeatRuntimeState, error) {
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
		defaultValue := automationdomain.DefaultHeartbeatConfig(agentID)
		sanitizedConfig, deliveryError := sanitizeHeartbeatConfig(defaultValue)
		state = &automationexec.HeartbeatRuntimeState{
			Config:          sanitizedConfig,
			NextRunAt:       s.computeHeartbeatNext(sanitizedConfig, s.nowFn()),
			LastHeartbeatAt: cloneTimePointer(lastHeartbeatAt),
			LastAckAt:       cloneTimePointer(lastAckAt),
			DeliveryError:   cloneStringPointer(deliveryError),
		}
	} else {
		normalized, deliveryError := sanitizeHeartbeatConfig(configValue.Normalized())
		state = &automationexec.HeartbeatRuntimeState{
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

func (s *Service) computeHeartbeatNext(configValue automationdomain.HeartbeatConfig, now time.Time) *time.Time {
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
	configValue := automationdomain.HeartbeatConfig{}
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

func (s *Service) snapshotHeartbeatState(agentID string) (automationexec.HeartbeatRuntimeState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.heartbeatState[strings.TrimSpace(agentID)]
	if state == nil {
		return automationexec.HeartbeatRuntimeState{}, false
	}
	return automationexec.HeartbeatRuntimeState{
		Config:          state.Config,
		Running:         state.Running,
		PendingWake:     state.PendingWake,
		NextRunAt:       cloneTimePointer(state.NextRunAt),
		LastHeartbeatAt: cloneTimePointer(state.LastHeartbeatAt),
		LastAckAt:       cloneTimePointer(state.LastAckAt),
		DeliveryError:   cloneStringPointer(state.DeliveryError),
	}, true
}

func (s *Service) persistHeartbeatTimes(ctx context.Context, agentID string, lastHeartbeatAt *time.Time, lastAckAt *time.Time) error {
	if _, err := s.ensureHeartbeatState(ctx, agentID); err != nil {
		return err
	}
	snapshot, ok := s.snapshotHeartbeatState(agentID)
	if !ok {
		return errors.New("heartbeat state not found")
	}
	return s.repository.UpsertHeartbeatState(ctx, s.idFactory("hb"), snapshot.Config, lastHeartbeatAt, lastAckAt)
}

func (s *Service) recordWakeRequest(agentID string, sessionKey string, wakeMode string, text *string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionKey = strings.TrimSpace(sessionKey)
	request := automationexec.HeartbeatWakeRequest{
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
	sessionKey := automationexec.BuildMainSessionKey(agentID)
	for _, item := range s.wakeRequests[sessionKey] {
		if strings.TrimSpace(item.AgentID) == strings.TrimSpace(agentID) && item.WakeMode == automationdomain.WakeModeNow {
			return true
		}
	}
	return false
}

func (s *Service) takeWakeRequests(agentID string, sessionKey string) ([]automationexec.HeartbeatWakeRequest, []automationexec.HeartbeatWakeRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionKey = strings.TrimSpace(sessionKey)
	items := slices.Clone(s.wakeRequests[sessionKey])
	delete(s.wakeRequests, sessionKey)

	immediate := make([]automationexec.HeartbeatWakeRequest, 0, len(items))
	deferred := make([]automationexec.HeartbeatWakeRequest, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.AgentID) != strings.TrimSpace(agentID) {
			continue
		}
		switch item.WakeMode {
		case automationdomain.WakeModeNow:
			immediate = append(immediate, item)
		case automationdomain.WakeModeNextHeartbeat:
			deferred = append(deferred, item)
		}
	}
	return immediate, deferred
}

func sanitizeHeartbeatConfig(configValue automationdomain.HeartbeatConfig) (automationdomain.HeartbeatConfig, *string) {
	result := configValue
	if strings.TrimSpace(result.TargetMode) != automationdomain.HeartbeatTargetExplicit {
		return result, nil
	}
	result.TargetMode = automationdomain.HeartbeatTargetNone
	return result, stringPointer(heartbeatExplicitTargetUnsupportedMessage)
}
