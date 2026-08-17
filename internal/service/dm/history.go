// INPUT: DM Session、SDK transcript identity、runtime/tool-surface fingerprint 与 round marker。
// OUTPUT: 持久 Session runtime 状态、完整 transcript lineage 和可恢复历史投影。
// POS: DM Session 历史与 SDK identity 的唯一写回事务。
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
	files := s.files.ForOwner(agentValue.OwnerUserID)
	item, _, err := files.FindSession([]string{agentValue.WorkspacePath}, sessionKey)
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
				updated, updateErr := files.UpsertSession(agentValue.WorkspacePath, merged)
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
		updated, updateErr := files.UpsertSession(
			agentValue.WorkspacePath,
			closePersistedSessionMeta(*roomSession),
		)
		if updateErr != nil {
			return protocol.Session{}, updateErr
		}
		if updated == nil {
			return protocol.Session{}, fmt.Errorf("创建 room 成员会话失败: %s", sessionKey)
		}
		return *updated, nil
	}

	now := time.Now().UTC()
	created, err := files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
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
	return s.appendRuntimeHistoryMessageForOwner(
		"",
		workspacePath,
		sessionValue,
		message,
	)
}

func (s *Service) appendRuntimeHistoryMessageForOwner(
	ownerUserID string,
	workspacePath string,
	sessionValue protocol.Session,
	message protocol.Message,
) error {
	if protocol.IsTranscriptNativeMessage(protocol.Message(message)) {
		return nil
	}
	return s.history.ForOwner(ownerUserID).AppendOverlayMessage(
		workspacePath,
		sessionValue.SessionKey,
		message,
	)
}

func (s *Service) refreshSessionMetaAfterRoundMarker(
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	return s.refreshSessionMetaAfterRoundMarkerForOwner("", workspacePath, current)
}

func (s *Service) refreshSessionMetaAfterRoundMarkerForOwner(
	ownerUserID string,
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	current = closePersistedSessionMeta(current)
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	var err error
	current, err = s.preservePersistedSessionTitleForOwner(ownerUserID, workspacePath, current)
	if err != nil {
		return nil, err
	}
	return s.files.ForOwner(ownerUserID).PatchSessionRuntime(workspacePath, current)
}

func (s *Service) refreshSessionMetaAfterMessage(
	workspacePath string,
	current protocol.Session,
	message protocol.Message,
) (*protocol.Session, error) {
	return s.refreshSessionMetaAfterMessageForOwner("", workspacePath, current, message)
}

func (s *Service) refreshSessionMetaAfterMessageForOwner(
	ownerUserID string,
	workspacePath string,
	current protocol.Session,
	message protocol.Message,
) (*protocol.Session, error) {
	nextSessionID := s.preferPersistableMessageSessionIDForOwner(
		ownerUserID,
		context.Background(),
		workspacePath,
		current,
		dmdomain.NormalizeString(message["session_id"]),
	)
	nextSessionIDValue := ""
	if nextSessionID != nil {
		nextSessionIDValue = *nextSessionID
	}
	current.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
		current.TranscriptSessionIDs,
		protocol.SessionTranscriptIDs(current),
		[]string{nextSessionIDValue},
	)
	current.SessionID = nextSessionID
	current = closePersistedSessionMeta(current)
	current.LastActivity = time.Now().UTC()
	current.MessageCount++
	var err error
	current, err = s.preservePersistedSessionTitleForOwner(ownerUserID, workspacePath, current)
	if err != nil {
		return nil, err
	}
	return s.files.ForOwner(ownerUserID).PatchSessionRuntime(workspacePath, current)
}

func (s *Service) preferPersistableMessageSessionID(
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	messageSessionID string,
) *string {
	return s.preferPersistableMessageSessionIDForOwner(
		"",
		ctx,
		workspacePath,
		current,
		messageSessionID,
	)
}

func (s *Service) preferPersistableMessageSessionIDForOwner(
	ownerUserID string,
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	messageSessionID string,
) *string {
	trimmedSessionID := strings.TrimSpace(messageSessionID)
	if trimmedSessionID == "" {
		return current.SessionID
	}
	if !s.canPersistSDKSessionIDForOwner(
		ownerUserID,
		ctx,
		workspacePath,
		current,
		trimmedSessionID,
	) {
		return current.SessionID
	}
	return &trimmedSessionID
}

func (s *Service) refreshSessionMetaRuntimeState(
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	return s.refreshSessionMetaRuntimeStateForOwner("", workspacePath, current)
}

func (s *Service) refreshSessionMetaRuntimeStateForOwner(
	ownerUserID string,
	workspacePath string,
	current protocol.Session,
) (*protocol.Session, error) {
	current = closePersistedSessionMeta(current)
	current.LastActivity = time.Now().UTC()
	var err error
	current, err = s.preservePersistedSessionTitleForOwner(ownerUserID, workspacePath, current)
	if err != nil {
		return nil, err
	}
	return s.files.ForOwner(ownerUserID).PatchSessionRuntime(workspacePath, current)
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
	item, _, err := s.files.ForOwner(agentValue.OwnerUserID).FindSession(
		[]string{agentValue.WorkspacePath},
		sessionKey,
	)
	if err != nil {
		return err
	}
	if item == nil {
		return nil
	}
	_, err = s.refreshSessionMetaRuntimeStateForOwner(
		agentValue.OwnerUserID,
		agentValue.WorkspacePath,
		*item,
	)
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
	return s.recordRoundMarkerWithOptionsForOwner(
		"",
		workspacePath,
		sessionValue,
		roundID,
		content,
		options,
	)
}

func (s *Service) recordRoundMarkerWithOptionsForOwner(
	ownerUserID string,
	workspacePath string,
	sessionValue protocol.Session,
	roundID string,
	content string,
	options workspacestore.RoundMarkerOptions,
) error {
	return s.history.ForOwner(ownerUserID).AppendRoundMarkerWithOptions(
		workspacePath,
		sessionValue.SessionKey,
		roundID,
		content,
		time.Now().UnixMilli(),
		options,
	)
}

func (s *Service) syncSDKSessionIDForOwner(
	ctx context.Context,
	ownerUserID string,
	workspacePath string,
	current protocol.Session,
	sessionID string,
	runtimeKind string,
	runtimeProvider string,
	runtimeModel string,
	toolSurfaceFingerprint string,
) (protocol.Session, error) {
	sync := sdkSessionSync{
		service:       s,
		ctx:           ctx,
		ownerUserID:   strings.TrimSpace(ownerUserID),
		workspacePath: workspacePath,
		current:       current,
		nextSessionID: strings.TrimSpace(sessionID),
		nextFingerprint: sessionRuntimeFingerprint{
			kind:        strings.TrimSpace(runtimeKind),
			provider:    strings.TrimSpace(runtimeProvider),
			model:       strings.TrimSpace(runtimeModel),
			toolSurface: strings.TrimSpace(toolSurfaceFingerprint),
		},
	}
	return sync.run()
}

type sessionRuntimeFingerprint struct {
	kind        string
	provider    string
	model       string
	toolSurface string
}

func runtimeFingerprintFromSession(session protocol.Session) sessionRuntimeFingerprint {
	kind, _ := session.Options[protocol.OptionRuntimeKind].(string)
	provider, _ := session.Options[protocol.OptionRuntimeProvider].(string)
	model, _ := session.Options[protocol.OptionRuntimeModel].(string)
	toolSurface, _ := session.Options[protocol.OptionRuntimeToolSurfaceFingerprint].(string)
	return sessionRuntimeFingerprint{
		kind:        strings.TrimSpace(kind),
		provider:    strings.TrimSpace(provider),
		model:       strings.TrimSpace(model),
		toolSurface: strings.TrimSpace(toolSurface),
	}
}

func (f sessionRuntimeFingerprint) apply(options map[string]any) {
	options[protocol.OptionRuntimeKind] = f.kind
	options[protocol.OptionRuntimeProvider] = f.provider
	options[protocol.OptionRuntimeModel] = f.model
	options[protocol.OptionRuntimeToolSurfaceFingerprint] = f.toolSurface
}

type sdkSessionSync struct {
	service            *Service
	ctx                context.Context
	ownerUserID        string
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
	s.canPersistSession = !s.sessionIDChanged || s.service.canPersistSDKSessionIDForOwner(
		s.ownerUserID,
		s.ctx,
		s.workspacePath,
		s.current,
		s.nextSessionID,
	)
}

func (s *sdkSessionSync) apply() {
	if s.canPersistSession {
		currentSessionID := strings.TrimSpace(dmdomain.StringPointerValue(s.current.SessionID))
		s.current.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
			s.current.TranscriptSessionIDs,
			[]string{currentSessionID, s.nextSessionID},
		)
		s.current.SessionID = &s.nextSessionID
	}
	if s.current.Options == nil {
		s.current.Options = map[string]any{}
	}
	nextFingerprint := s.nextFingerprint
	if s.sessionIDChanged && !s.canPersistSession {
		// 新 transcript 尚不可恢复时，不能把它的工具面基线提交到旧 SDK session；
		// 否则下一轮会把旧 K3 identity 误判为已经采用新工具面。
		nextFingerprint.toolSurface = runtimeFingerprintFromSession(s.current).toolSurface
	}
	nextFingerprint.apply(s.current.Options)
}

func (s *sdkSessionSync) persist() (protocol.Session, error) {
	current, err := s.service.preservePersistedSessionTitleForOwner(
		s.ownerUserID,
		s.workspacePath,
		s.current,
	)
	if err != nil {
		return protocol.Session{}, err
	}
	updated, err := s.service.files.ForOwner(s.ownerUserID).PatchSessionRuntime(
		s.workspacePath,
		current,
	)
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

func (s *Service) canPersistSDKSessionIDForOwner(
	ownerUserID string,
	ctx context.Context,
	workspacePath string,
	current protocol.Session,
	sessionID string,
) bool {
	decision := sessionresumesvc.NewPolicy(
		s.history.ForOwner(ownerUserID),
	).CanPersist(workspacePath, sessionID)
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
	current.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
		current.TranscriptSessionIDs,
		protocol.SessionTranscriptIDs(current),
	)
	current.SessionID = nil
	current = closePersistedSessionMeta(current)
	var err error
	current, err = s.preservePersistedSessionTitleForOwner(
		authctx.OwnerUserID(ctx),
		workspacePath,
		current,
	)
	if err != nil {
		return protocol.Session{}, err
	}
	updated, err := s.files.ForOwner(authctx.OwnerUserID(ctx)).PatchSessionRuntime(
		workspacePath,
		current,
	)
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
	return s.preservePersistedSessionTitleForOwner("", workspacePath, current)
}

func (s *Service) preservePersistedSessionTitleForOwner(
	ownerUserID string,
	workspacePath string,
	current protocol.Session,
) (protocol.Session, error) {
	if s == nil || s.files == nil ||
		strings.TrimSpace(workspacePath) == "" ||
		strings.TrimSpace(current.SessionKey) == "" {
		return current, nil
	}
	persisted, _, err := s.files.ForOwner(ownerUserID).FindSession(
		[]string{workspacePath},
		current.SessionKey,
	)
	if err != nil {
		return protocol.Session{}, err
	}
	if persisted == nil || strings.TrimSpace(persisted.Title) == "" {
		return current, nil
	}
	current.Title = persisted.Title
	current.ConfigurationVersion = persisted.ConfigurationVersion
	return current, nil
}
