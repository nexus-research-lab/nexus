package dm

import (
	"context"
	"fmt"
	"strings"
	"time"

	dmdomain "github.com/nexus-research-lab/nexus/internal/chat/dm"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sessionresumesvc "github.com/nexus-research-lab/nexus/internal/service/sessionresume"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func (s *Service) ensureSession(
	ctx context.Context,
	agentValue *protocol.Agent,
	parsed protocol.SessionKey,
	sessionKey string,
) (protocol.Session, error) {
	item, _, err := s.files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return protocol.Session{}, err
	}
	roomSession, err := s.lookupRoomSession(ctx, parsed)
	if err != nil {
		return protocol.Session{}, err
	}

	if item != nil {
		if roomSession != nil {
			merged := dmdomain.MergeRoomBackedSession(*item, *roomSession)
			merged = closePersistedSessionMeta(merged)
			if !dmdomain.SessionsEqual(*item, merged) {
				updated, updateErr := s.files.UpsertSession(agentValue.WorkspacePath, merged)
				if updateErr != nil {
					return protocol.Session{}, updateErr
				}
				if updated != nil {
					item = updated
				} else {
					item = &merged
				}
			}
		}
		return *item, nil
	}

	if roomSession != nil {
		updated, updateErr := s.files.UpsertSession(agentValue.WorkspacePath, closePersistedSessionMeta(*roomSession))
		if updateErr != nil {
			return protocol.Session{}, updateErr
		}
		if updated == nil {
			return protocol.Session{}, fmt.Errorf("创建 room 成员会话失败: %s", sessionKey)
		}
		return *updated, nil
	}

	now := time.Now().UTC()
	created, err := s.files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      agentValue.AgentID,
		ChannelType:  protocol.NormalizeStoredChannelType(parsed.Channel),
		ChatType:     protocol.NormalizeSessionChatType(parsed.ChatType),
		Status:       "closed",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "New Chat",
		Options:      map[string]any{},
		IsActive:     false,
	})
	if err != nil {
		return protocol.Session{}, err
	}
	if created == nil {
		return protocol.Session{}, fmt.Errorf("创建 session 失败: %s", sessionKey)
	}
	return *created, nil
}

func (s *Service) lookupRoomSession(
	ctx context.Context,
	parsed protocol.SessionKey,
) (*protocol.Session, error) {
	if s.roomStore == nil {
		return nil, nil
	}
	return s.roomStore.GetRoomSessionByKey(ctx, authctx.OwnerUserID(ctx), parsed)
}

func (s *Service) appendRuntimeHistoryMessage(
	workspacePath string,
	sessionValue protocol.Session,
	message protocol.Message,
) error {
	if protocol.IsTranscriptNativeMessage(protocol.Message(message)) {
		return nil
	}
	return s.history.AppendOverlayMessage(workspacePath, sessionValue.SessionKey, message)
}

func (s *Service) refreshSessionMetaAfterRoundMarker(
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	current = closePersistedSessionMeta(current)
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	var err error
	current, err = s.preservePersistedSessionTitle(workspacePath, current)
	if err != nil {
		return nil, err
	}
	return s.files.UpsertSession(workspacePath, current)
}

func (s *Service) refreshSessionMetaAfterMessage(
	workspacePath string,
	current protocol.Session,
	message protocol.Message,
) (*protocol.Session, error) {
	current.SessionID = s.preferPersistableMessageSessionID(
		context.Background(),
		workspacePath,
		current,
		dmdomain.NormalizeString(message["session_id"]),
	)
	current = closePersistedSessionMeta(current)
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	var err error
	current, err = s.preservePersistedSessionTitle(workspacePath, current)
	if err != nil {
		return nil, err
	}
	return s.files.UpsertSession(workspacePath, current)
}

func (s *Service) preferPersistableMessageSessionID(
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	messageSessionID string,
) *string {
	trimmedSessionID := strings.TrimSpace(messageSessionID)
	if trimmedSessionID == "" {
		return current.SessionID
	}
	if !s.canPersistSDKSessionID(ctx, workspacePath, current, trimmedSessionID) {
		return current.SessionID
	}
	return &trimmedSessionID
}

func (s *Service) refreshSessionMetaRuntimeState(
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	current = closePersistedSessionMeta(current)
	current.LastActivity = time.Now().UTC()
	var err error
	current, err = s.preservePersistedSessionTitle(workspacePath, current)
	if err != nil {
		return nil, err
	}
	return s.files.UpsertSession(workspacePath, current)
}

func (s *Service) refreshSessionMetaRuntimeStateByKey(ctx context.Context, sessionKey string) error {
	parsed := protocol.ParseSessionKey(sessionKey)
	if strings.TrimSpace(parsed.AgentID) == "" {
		return nil
	}
	agentValue, err := s.agents.GetAgent(ctx, parsed.AgentID)
	if err != nil {
		return err
	}
	item, _, err := s.files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}
	_, err = s.refreshSessionMetaRuntimeState(agentValue.WorkspacePath, *item)
	return err
}

func closePersistedSessionMeta(current protocol.Session) protocol.Session {
	current.Status = "closed"
	current.IsActive = false
	return current
}

func (s *Service) recordRoundMarkerWithOptions(
	workspacePath string,
	sessionValue protocol.Session,
	roundID string,
	content string,
	options workspacestore.RoundMarkerOptions,
) error {
	return s.history.AppendRoundMarkerWithOptions(
		workspacePath,
		sessionValue.SessionKey,
		roundID,
		content,
		time.Now().UnixMilli(),
		options,
	)
}

func (s *Service) syncSDKSessionID(
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	sessionID string,
	runtimeKind string,
	runtimeProvider string,
	runtimeModel string,
) (protocol.Session, error) {
	sync := sdkSessionSync{
		service:       s,
		ctx:           ctx,
		workspacePath: workspacePath,
		current:       current,
		nextSessionID: strings.TrimSpace(sessionID),
		nextFingerprint: sessionRuntimeFingerprint{
			kind:     strings.TrimSpace(runtimeKind),
			provider: strings.TrimSpace(runtimeProvider),
			model:    strings.TrimSpace(runtimeModel),
		},
	}
	return sync.run()
}

type sessionRuntimeFingerprint struct {
	kind     string
	provider string
	model    string
}

func runtimeFingerprintFromSession(session protocol.Session) sessionRuntimeFingerprint {
	kind, _ := session.Options[protocol.OptionRuntimeKind].(string)
	provider, _ := session.Options[protocol.OptionRuntimeProvider].(string)
	model, _ := session.Options[protocol.OptionRuntimeModel].(string)
	return sessionRuntimeFingerprint{
		kind:     strings.TrimSpace(kind),
		provider: strings.TrimSpace(provider),
		model:    strings.TrimSpace(model),
	}
}

func (f sessionRuntimeFingerprint) apply(options map[string]any) {
	options[protocol.OptionRuntimeKind] = f.kind
	options[protocol.OptionRuntimeProvider] = f.provider
	options[protocol.OptionRuntimeModel] = f.model
}

type sdkSessionSync struct {
	service            *Service
	ctx                context.Context
	workspacePath      string
	current            protocol.Session
	nextSessionID      string
	nextFingerprint    sessionRuntimeFingerprint
	sessionIDChanged   bool
	fingerprintChanged bool
	canPersistSession  bool
}

func (s *sdkSessionSync) run() (protocol.Session, error) {
	if !s.prepare() {
		return s.current, nil
	}
	s.decideSessionPersistence()
	if !s.canPersistSession && !s.fingerprintChanged {
		return s.current, nil
	}
	s.apply()
	return s.persist()
}

func (s *sdkSessionSync) prepare() bool {
	if s.nextSessionID == "" {
		return false
	}
	currentSessionID := strings.TrimSpace(dmdomain.StringPointerValue(s.current.SessionID))
	s.sessionIDChanged = currentSessionID != s.nextSessionID
	s.fingerprintChanged = runtimeFingerprintFromSession(s.current) != s.nextFingerprint
	return s.sessionIDChanged || s.fingerprintChanged
}

func (s *sdkSessionSync) decideSessionPersistence() {
	s.canPersistSession = !s.sessionIDChanged || s.service.canPersistSDKSessionID(
		s.ctx,
		s.workspacePath,
		s.current,
		s.nextSessionID,
	)
}

func (s *sdkSessionSync) apply() {
	if s.canPersistSession {
		s.current.SessionID = &s.nextSessionID
	}
	if s.current.Options == nil {
		s.current.Options = map[string]any{}
	}
	s.nextFingerprint.apply(s.current.Options)
}

func (s *sdkSessionSync) persist() (protocol.Session, error) {
	current, err := s.service.preservePersistedSessionTitle(s.workspacePath, s.current)
	if err != nil {
		return protocol.Session{}, err
	}
	updated, err := s.service.files.UpsertSession(s.workspacePath, current)
	if err != nil {
		return protocol.Session{}, err
	}
	if updated == nil {
		return current, nil
	}
	if err = s.syncRoomSession(*updated); err != nil {
		return protocol.Session{}, err
	}
	return *updated, nil
}

func (s *sdkSessionSync) syncRoomSession(updated protocol.Session) error {
	if !s.canPersistSession || !s.sessionIDChanged || s.service.roomStore == nil || updated.RoomSessionID == nil {
		return nil
	}
	roomSessionID := strings.TrimSpace(*updated.RoomSessionID)
	if roomSessionID == "" {
		return nil
	}
	return s.service.roomStore.UpdateRoomSessionSDKSessionID(s.ctx, roomSessionID, s.nextSessionID)
}

func (s *Service) canPersistSDKSessionID(
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	sessionID string,
) bool {
	decision := sessionresumesvc.NewPolicy(s.history).CanPersist(workspacePath, sessionID)
	if decision.Allowed {
		return true
	}
	if decision.Err != nil {
		s.loggerFor(ctx).Warn("检查 SDK session transcript 失败，暂不持久化 resume",
			"session_key", current.SessionKey,
			"workspace_path", workspacePath,
			"sdk_session_id", decision.SessionID,
			"reason", string(decision.Reason),
			"err", decision.Err,
		)
		return false
	}
	s.loggerFor(ctx).Warn("SDK session transcript 尚未落盘，暂不持久化 resume",
		"session_key", current.SessionKey,
		"workspace_path", workspacePath,
		"sdk_session_id", decision.SessionID,
		"reason", string(decision.Reason),
	)
	return false
}

func (s *Service) clearReusableSDKSessionID(
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
) (protocol.Session, error) {
	current.SessionID = nil
	current = closePersistedSessionMeta(current)
	var err error
	current, err = s.preservePersistedSessionTitle(workspacePath, current)
	if err != nil {
		return protocol.Session{}, err
	}
	updated, err := s.files.UpsertSession(workspacePath, current)
	if err != nil {
		return protocol.Session{}, err
	}
	if updated != nil {
		current = *updated
	}
	if err := s.clearRoomSDKSessionID(ctx, current); err != nil {
		return protocol.Session{}, err
	}
	return current, nil
}

func (s *Service) clearRoomSDKSessionID(ctx context.Context, current protocol.Session) error {
	if s.roomStore == nil || current.RoomSessionID == nil {
		return nil
	}
	roomSessionID := strings.TrimSpace(*current.RoomSessionID)
	if roomSessionID == "" {
		return nil
	}
	return s.roomStore.UpdateRoomSessionSDKSessionID(ctx, roomSessionID, "")
}

func (s *Service) preservePersistedSessionTitle(
	workspacePath string,
	current protocol.Session,
) (protocol.Session, error) {
	if s == nil || s.files == nil ||
		strings.TrimSpace(workspacePath) == "" ||
		strings.TrimSpace(current.SessionKey) == "" {
		return current, nil
	}
	persisted, _, err := s.files.FindSession([]string{workspacePath}, current.SessionKey)
	if err != nil {
		return protocol.Session{}, err
	}
	if persisted == nil || strings.TrimSpace(persisted.Title) == "" {
		return current, nil
	}
	current.Title = persisted.Title
	return current, nil
}
