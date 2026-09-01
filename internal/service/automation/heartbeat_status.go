// INPUT: owner-scoped Agent、heartbeat 配置 CAS 与手动 wake intent。
// OUTPUT: 配置状态，或先 durable acceptance 再异步 dispatch 的 wake receipt。
// POS: Heartbeat 人工控制入口；不持控制锁跨 runtime dispatch。
package automation

import (
	"context"
	"errors"
	"strings"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

// GetHeartbeatStatus 返回 heartbeat 状态。
func (s *Service) GetHeartbeatStatus(ctx context.Context, agentID string) (*automationdomain.HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	if _, err := s.requireAgent(ctx, agentID); err != nil {
		return nil, err
	}
	if _, err := s.refreshHeartbeatState(ctx, agentID); err != nil {
		return nil, err
	}
	snapshot, ok := s.snapshotHeartbeatState(agentID)
	if !ok {
		return nil, errors.New("heartbeat state not found")
	}
	return &automationdomain.HeartbeatStatus{
		AgentID:              snapshot.Config.AgentID,
		Enabled:              snapshot.Config.Enabled,
		EverySeconds:         snapshot.Config.EverySeconds,
		TargetMode:           snapshot.Config.TargetMode,
		AckMaxChars:          snapshot.Config.AckMaxChars,
		Running:              snapshot.Running,
		PendingWake:          snapshot.PendingWake,
		NextRunAt:            cloneTimePointer(snapshot.NextRunAt),
		LastHeartbeatAt:      cloneTimePointer(snapshot.LastHeartbeatAt),
		LastAckAt:            cloneTimePointer(snapshot.LastAckAt),
		DeliveryError:        cloneStringPointer(snapshot.DeliveryError),
		ConfigurationVersion: snapshot.Config.ConfigurationVersion,
	}, nil
}

// UpdateHeartbeat 更新 heartbeat 配置。
func (s *Service) UpdateHeartbeat(ctx context.Context, agentID string, input automationdomain.HeartbeatUpdateInput) (*automationdomain.HeartbeatStatus, error) {
	return s.updateHeartbeat(ctx, agentID, input, nil)
}

// UpdateHeartbeatAtVersion 仅更新调用方在读取阶段看到的 heartbeat 配置版本。
func (s *Service) UpdateHeartbeatAtVersion(
	ctx context.Context,
	agentID string,
	expectedVersion int64,
	input automationdomain.HeartbeatUpdateInput,
) (*automationdomain.HeartbeatStatus, error) {
	return s.updateHeartbeat(ctx, agentID, input, &expectedVersion)
}

func (s *Service) updateHeartbeat(
	ctx context.Context,
	agentID string,
	input automationdomain.HeartbeatUpdateInput,
	expectedVersion *int64,
) (*automationdomain.HeartbeatStatus, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.heartbeatControlMu.Lock()
	defer s.heartbeatControlMu.Unlock()

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

	state, err := s.refreshHeartbeatState(ctx, configValue.AgentID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	currentVersion := state.Config.ConfigurationVersion
	lastHeartbeatAt := cloneTimePointer(state.LastHeartbeatAt)
	lastAckAt := cloneTimePointer(state.LastAckAt)
	s.mu.Unlock()
	if expectedVersion != nil && (*expectedVersion < 0 || currentVersion != *expectedVersion) {
		return nil, automationdomain.ErrConfigurationVersionConflict
	}
	if expectedVersion == nil {
		err = s.repository.UpsertHeartbeatState(
			ctx,
			s.idFactory("hb"),
			configValue,
			lastHeartbeatAt,
			lastAckAt,
		)
	} else {
		err = s.repository.UpsertHeartbeatStateAtVersion(
			ctx,
			s.idFactory("hb"),
			configValue,
			lastHeartbeatAt,
			lastAckAt,
			*expectedVersion,
		)
	}
	if err != nil {
		return nil, err
	}
	persisted, persistedHeartbeatAt, persistedAckAt, err := s.repository.GetHeartbeatState(ctx, configValue.AgentID)
	if err != nil {
		return nil, err
	}
	if persisted == nil {
		return nil, errors.New("heartbeat configuration was not persisted")
	}
	normalized, deliveryError := sanitizeHeartbeatConfig(persisted.Normalized())
	s.mu.Lock()
	state.Config = normalized
	state.NextRunAt = s.computeHeartbeatNext(normalized, s.nowFn())
	state.LastHeartbeatAt = cloneTimePointer(persistedHeartbeatAt)
	state.LastAckAt = cloneTimePointer(persistedAckAt)
	state.DeliveryError = cloneStringPointer(deliveryError)
	s.mu.Unlock()
	s.wakeScheduler()
	return s.GetHeartbeatStatus(ctx, configValue.AgentID)
}

// WakeHeartbeat 手动登记一次 heartbeat 唤醒。
func (s *Service) WakeHeartbeat(ctx context.Context, agentID string, request automationdomain.HeartbeatWakeInput) (*automationdomain.HeartbeatWakeResult, error) {
	return s.wakeHeartbeat(ctx, agentID, request, nil, heartbeatWakeIdentity{})
}

// WakeHeartbeatAtVersion 只按 plan 阶段核对过的 heartbeat 配置登记唤醒。
func (s *Service) WakeHeartbeatAtVersion(
	ctx context.Context,
	agentID string,
	expectedVersion int64,
	request automationdomain.HeartbeatWakeInput,
) (*automationdomain.HeartbeatWakeResult, error) {
	return s.wakeHeartbeat(ctx, agentID, request, &expectedVersion, heartbeatWakeIdentity{})
}

type heartbeatWakeIdentity struct {
	ownerUserID  string
	requestID    string
	intentDigest string
}

func (s *Service) wakeHeartbeat(
	ctx context.Context,
	agentID string,
	request automationdomain.HeartbeatWakeInput,
	expectedVersion *int64,
	identity heartbeatWakeIdentity,
) (*automationdomain.HeartbeatWakeResult, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	s.heartbeatControlMu.Lock()
	targetAgentID := strings.TrimSpace(agentID)
	agentValue, err := s.requireAgent(ctx, targetAgentID)
	if err != nil {
		s.heartbeatControlMu.Unlock()
		return nil, err
	}
	identity.ownerUserID = strings.TrimSpace(identity.ownerUserID)
	identity.requestID = strings.TrimSpace(identity.requestID)
	identity.intentDigest = strings.TrimSpace(identity.intentDigest)
	ownerUserID, scoped := scopedOwnerUserID(ctx)
	if identity.ownerUserID == "" {
		identity.ownerUserID = strings.TrimSpace(ownerUserID)
	} else if scoped && strings.TrimSpace(ownerUserID) != identity.ownerUserID {
		s.heartbeatControlMu.Unlock()
		return nil, automationdomain.ErrHeartbeatWakeRequestConflict
	}
	if agentValue != nil && identity.ownerUserID != "" &&
		strings.TrimSpace(agentValue.OwnerUserID) != identity.ownerUserID {
		s.heartbeatControlMu.Unlock()
		return nil, automationdomain.ErrHeartbeatWakeRequestConflict
	}
	mode := strings.TrimSpace(request.Mode)
	if mode == "" {
		mode = automationdomain.WakeModeNow
	}
	if mode != automationdomain.WakeModeNow && mode != automationdomain.WakeModeNextHeartbeat {
		s.heartbeatControlMu.Unlock()
		return nil, errors.New("mode must be one of now, next-heartbeat")
	}

	state, err := s.ensureHeartbeatState(ctx, targetAgentID)
	if err != nil {
		s.heartbeatControlMu.Unlock()
		return nil, err
	}
	accepted, err := s.repository.AcceptHeartbeatWake(ctx, automationstore.HeartbeatWakeAcceptanceInput{
		EventID: s.idFactory("evt"), AgentID: targetAgentID,
		OwnerUserID: identity.ownerUserID, RequestID: identity.requestID,
		IntentDigest: identity.intentDigest, Mode: mode, Text: request.Text,
		ExpectedConfigurationVersion: expectedVersion, AcceptedAt: s.nowFn(),
	})
	if err != nil {
		s.heartbeatControlMu.Unlock()
		return nil, err
	}

	s.mu.Lock()
	state.PendingWake = accepted.Event.Status == "new" || accepted.Event.Status == "processing"
	running := state.Running
	s.mu.Unlock()
	s.heartbeatControlMu.Unlock()

	s.wakeScheduler()
	result := &automationdomain.HeartbeatWakeResult{
		AgentID: targetAgentID, Mode: mode, Scheduled: mode == automationdomain.WakeModeNow,
	}
	if !accepted.Replayed && mode == automationdomain.WakeModeNow && !running {
		s.dispatchHeartbeat(targetAgentID, "wake-now")
	}
	return result, nil
}
