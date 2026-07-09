package automation

import (
	"context"
	"errors"
	"strings"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
)

// GetHeartbeatStatus 返回 heartbeat 状态。
func (s *Service) GetHeartbeatStatus(ctx context.Context, agentID string) (*automationdomain.HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	if _, err := s.ensureHeartbeatState(ctx, agentID); err != nil {
		return nil, err
	}
	snapshot, ok := s.snapshotHeartbeatState(agentID)
	if !ok {
		return nil, errors.New("heartbeat state not found")
	}
	return &automationdomain.HeartbeatStatus{
		AgentID:         snapshot.Config.AgentID,
		Enabled:         snapshot.Config.Enabled,
		EverySeconds:    snapshot.Config.EverySeconds,
		TargetMode:      snapshot.Config.TargetMode,
		AckMaxChars:     snapshot.Config.AckMaxChars,
		Running:         snapshot.Running,
		PendingWake:     snapshot.PendingWake,
		NextRunAt:       cloneTimePointer(snapshot.NextRunAt),
		LastHeartbeatAt: cloneTimePointer(snapshot.LastHeartbeatAt),
		LastAckAt:       cloneTimePointer(snapshot.LastAckAt),
		DeliveryError:   cloneStringPointer(snapshot.DeliveryError),
	}, nil
}

// UpdateHeartbeat 更新 heartbeat 配置。
func (s *Service) UpdateHeartbeat(ctx context.Context, agentID string, input automationdomain.HeartbeatUpdateInput) (*automationdomain.HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	configValue := automationdomain.HeartbeatConfig{
		AgentID:      strings.TrimSpace(agentID),
		Enabled:      input.Enabled,
		EverySeconds: input.EverySeconds,
		TargetMode:   strings.TrimSpace(input.TargetMode),
		AckMaxChars:  input.AckMaxChars,
	}.Normalized()
	if configValue.TargetMode == automationdomain.HeartbeatTargetExplicit {
		return nil, automationdomain.ErrHeartbeatConfigInvalid
	}
	if err := configValue.Validate(); err != nil {
		return nil, err
	}

	state, err := s.ensureHeartbeatState(ctx, configValue.AgentID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	state.Config = configValue
	state.NextRunAt = s.computeHeartbeatNext(configValue, s.nowFn())
	state.DeliveryError = nil
	if !configValue.Enabled {
		state.PendingWake = false
		state.Running = false
	}
	lastHeartbeatAt := cloneTimePointer(state.LastHeartbeatAt)
	lastAckAt := cloneTimePointer(state.LastAckAt)
	s.mu.Unlock()
	if err = s.repository.UpsertHeartbeatState(
		ctx,
		s.idFactory("hb"),
		configValue,
		lastHeartbeatAt,
		lastAckAt,
	); err != nil {
		return nil, err
	}
	return s.GetHeartbeatStatus(ctx, configValue.AgentID)
}

// WakeHeartbeat 手动登记一次 heartbeat 唤醒。
func (s *Service) WakeHeartbeat(ctx context.Context, agentID string, request automationdomain.HeartbeatWakeInput) (*automationdomain.HeartbeatWakeResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	mode := strings.TrimSpace(request.Mode)
	if mode == "" {
		mode = automationdomain.WakeModeNow
	}
	if mode != automationdomain.WakeModeNow && mode != automationdomain.WakeModeNextHeartbeat {
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
	sessionKey := automationexec.BuildMainSessionKey(state.Config.AgentID)
	s.recordWakeRequest(state.Config.AgentID, sessionKey, mode, request.Text)

	s.mu.Lock()
	switch mode {
	case automationdomain.WakeModeNow:
		if state.Running {
			state.PendingWake = true
			s.mu.Unlock()
			return &automationdomain.HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: true}, nil
		}
		state.PendingWake = true
		s.mu.Unlock()
		s.dispatchHeartbeat(state.Config.AgentID, "wake-now")
		return &automationdomain.HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: true}, nil
	default:
		state.PendingWake = true
		s.mu.Unlock()
		return &automationdomain.HeartbeatWakeResult{AgentID: state.Config.AgentID, Mode: mode, Scheduled: false}, nil
	}
}
