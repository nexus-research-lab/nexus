// INPUT: 已提交的 DM Session Connector 选择、配置版本与现有 SDK transcript。
// OUTPUT: latest-wins 的后台工具面 fork；不创建消息，失败由下一次真实输入同步兜底。
// POS: Session 配置控制面与 DM runtime 启动事务之间的异步预备边界。
package dm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

const (
	defaultConnectorPreparationDelay   = 200 * time.Millisecond
	defaultConnectorPreparationTimeout = 30 * time.Second
)

var errConnectorPreparationSuperseded = errors.New("Connector runtime preparation superseded")

type connectorRuntimePreparation struct {
	ctx     context.Context
	cancel  context.CancelFunc
	timer   *time.Timer
	session protocol.Session
}

// ScheduleRuntimeSettingsPreparation 在 Connector 选择提交后合并后台预备请求。
// 同一 Session 只保留最后一次配置；真实消息会取消尚未提交的预备并走同一启动主链。
func (s *Service) ScheduleRuntimeSettingsPreparation(ctx context.Context, session protocol.Session) {
	if !eligibleConnectorPreparationSession(session) {
		return
	}
	delay := s.connectorPreparationDelay
	if delay <= 0 {
		delay = defaultConnectorPreparationDelay
	}
	prepCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		defaultConnectorPreparationTimeout,
	)
	preparation := &connectorRuntimePreparation{
		ctx:     prepCtx,
		cancel:  cancel,
		session: session,
	}
	key := connectorPreparationKey(authctx.OwnerUserID(ctx), session.SessionKey)

	s.connectorPreparationMu.Lock()
	if previous := s.connectorPreparations[key]; previous != nil {
		if previous.timer != nil {
			previous.timer.Stop()
		}
		previous.cancel()
	}
	s.connectorPreparations[key] = preparation
	preparation.timer = time.AfterFunc(delay, func() {
		s.runConnectorRuntimePreparation(key, preparation)
	})
	s.connectorPreparationMu.Unlock()
}

func eligibleConnectorPreparationSession(session protocol.Session) bool {
	parsed := protocol.ParseSessionKey(session.SessionKey)
	return parsed.IsStructured &&
		parsed.Kind == protocol.SessionKeyKindAgent &&
		protocol.NormalizeSessionChatType(parsed.ChatType) == protocol.RoomTypeDM
}

func connectorPreparationKey(ownerUserID string, sessionKey string) string {
	return strings.TrimSpace(ownerUserID) + "\x00" + strings.TrimSpace(sessionKey)
}

func (s *Service) runConnectorRuntimePreparation(
	key string,
	preparation *connectorRuntimePreparation,
) {
	s.connectorPreparationMu.Lock()
	if s.connectorPreparations[key] != preparation {
		s.connectorPreparationMu.Unlock()
		preparation.cancel()
		return
	}
	s.connectorPreparationMu.Unlock()

	runner := s.connectorPreparationRun
	if runner == nil {
		runner = s.prepareConnectorRuntime
	}
	err := runner(preparation.ctx, preparation.session)

	s.connectorPreparationMu.Lock()
	if s.connectorPreparations[key] == preparation {
		delete(s.connectorPreparations, key)
	}
	s.connectorPreparationMu.Unlock()
	preparation.cancel()

	if err == nil || errors.Is(err, errConnectorPreparationSuperseded) ||
		errors.Is(err, context.Canceled) {
		return
	}
	s.loggerFor(preparation.ctx).Warn(
		"后台预备 Connector runtime 失败，保留下一轮同步兜底",
		"session_key", preparation.session.SessionKey,
		"configuration_version", preparation.session.ConfigurationVersion,
		"err", err,
	)
}

// cancelConnectorRuntimePreparation 让真实输入跳过 debounce，并阻止旧预备晚于输入提交。
func (s *Service) cancelConnectorRuntimePreparation(ownerUserID string, sessionKey string) {
	key := connectorPreparationKey(ownerUserID, sessionKey)
	s.connectorPreparationMu.Lock()
	preparation := s.connectorPreparations[key]
	if preparation != nil {
		delete(s.connectorPreparations, key)
		if preparation.timer != nil {
			preparation.timer.Stop()
		}
		preparation.cancel()
	}
	s.connectorPreparationMu.Unlock()
}

func (s *Service) prepareConnectorRuntime(
	ctx context.Context,
	snapshot protocol.Session,
) error {
	parsed := protocol.ParseSessionKey(snapshot.SessionKey)
	agentValue, err := s.agents.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return err
	}
	ctx = contextWithExactOwner(ctx, agentValue.OwnerUserID)
	expectedSelection := protocol.SessionConnectorSelectionFromOptions(snapshot.Options)
	current, err := s.ensureSession(ctx, agentValue, parsed, snapshot.SessionKey)
	if err != nil {
		return err
	}
	if !protocol.SessionConnectorSelectionFromOptions(current.Options).Equal(expectedSelection) {
		return fmt.Errorf("%w: Connector selection changed before startup", errConnectorPreparationSuperseded)
	}
	if !eligibleConnectorPreparationSession(current) {
		return nil
	}
	sourceSessionID := ""
	if current.SessionID != nil {
		sourceSessionID = strings.TrimSpace(*current.SessionID)
	}
	if sourceSessionID == "" {
		return nil
	}
	decision := sessionresumesvc.NewPolicy(
		s.history.ForOwner(agentValue.OwnerUserID),
	).CanResume(agentValue.WorkspacePath, sourceSessionID)
	if !decision.Allowed {
		if decision.Err != nil {
			return fmt.Errorf("检查 Connector 预备源 transcript: %w", decision.Err)
		}
		return nil
	}
	roundID := protocol.NewRoundID()
	_, err = s.ensureClient(ctx, snapshot.SessionKey, agentValue, current, Request{
		SessionKey:                   snapshot.SessionKey,
		AgentID:                      agentValue.AgentID,
		RoundID:                      roundID,
		AgentRoundID:                 protocol.NewAgentRoundID(),
		Internal:                     true,
		runtimePreparationOnly:       true,
		expectedConfigurationVersion: current.ConfigurationVersion,
		expectedConnectorSelection:   &expectedSelection,
	})
	if errors.Is(err, workspacestore.ErrSessionConfigurationVersionConflict) {
		return fmt.Errorf("%w: %v", errConnectorPreparationSuperseded, err)
	}
	return err
}
