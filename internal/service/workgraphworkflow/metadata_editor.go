// INPUT: exact WorkGraph Draft、Nexus 主智能体隐藏 Session 与模型提交的完整草图版本。
// OUTPUT: 可恢复编辑对话、不可变版本历史、版本选择、CAS revision，以及应用前的 DAG/交付语义校验。
// POS: 对话式草图编辑边界；普通 DM 负责消息/流式 UI，本服务拥有 Draft 版本和受限 CLI 修改授权。
package workgraphworkflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

const (
	workGraphEditorMaxNodes = 64
	workGraphEditorMaxEdges = 256
)

// EditorSessionManager 由组合层把 WorkGraph Draft 连接到隐藏的 Nexus 主智能体 DM Session。
type EditorSessionManager interface {
	CreateWorkGraphEditorSession(context.Context, EditorSessionCreateRequest) (*protocol.Session, error)
	DeleteWorkGraphEditorSession(context.Context, string) error
}

// EditorSessionCreateRequest 不暴露给 HTTP；所有 identity 都来自已验证 preview/source Execution。
type EditorSessionCreateRequest struct {
	AgentID               string
	TargetSessionKey      string
	DisplayAfterUnixMilli int64
}

type workflowEditorRecord struct {
	ownerUserID           string
	sourceSessionKey      string
	previewID             string
	language              string
	revision              int64
	selectedRevision      int64
	agentID               string
	sessionKey            string
	displayAfterUnixMilli int64
	preview               protocol.WorkGraphWorkflowPreview
	versions              []protocol.WorkGraphWorkflowPreviewVersion
	unavailableSlashNames []string
	expiresAt             time.Time
}

// StartMetadataEditor 创建或恢复同一 Draft 的 Nexus 主智能体隐藏编辑 Session。
func (s *Service) StartMetadataEditor(
	ctx context.Context,
	ownerUserID string,
	request protocol.StartWorkGraphWorkflowEditorRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.PreviewID = strings.TrimSpace(request.PreviewID)
	request.OutputLanguage = normalizeWorkflowOutputLanguage(request.OutputLanguage)
	if ownerUserID == "" || request.SourceSessionKey == "" || request.PreviewID == "" || request.OutputLanguage == "" {
		return nil, fmt.Errorf("%w: editor scope is incomplete", ErrInvalidInput)
	}
	if s == nil || s.editorSessions == nil {
		return nil, errors.New("workgraph editor Session manager is unavailable")
	}
	loadedDraft, err := s.loadDraftByID(ctx, ownerUserID, request.PreviewID)
	if err != nil {
		return nil, err
	}

	s.previewMu.Lock()
	s.cleanupExpiredPreviews(s.now().UTC())
	previewRecord, ok := s.previews[previewCacheKey(ownerUserID, request.PreviewID)]
	if existingKey := s.editorByPreview[previewCacheKey(ownerUserID, request.PreviewID)]; existingKey != "" {
		if existing, exists := s.editors[existingKey]; exists && existing.sourceSessionKey == request.SourceSessionKey {
			existingID := editorIDFromCacheKey(existingKey)
			s.previewMu.Unlock()
			return editorSession(existingID, existing), nil
		}
	}
	s.previewMu.Unlock()
	if !ok || previewRecord.ownerUserID != ownerUserID || previewRecord.preview.SourceSessionKey != request.SourceSessionKey {
		return nil, ErrNotFound
	}
	preview := cloneWorkflowPreview(previewRecord.preview)
	if slashName := normalizeSlashName(request.SlashName); slashName != "" {
		preview.SlashName = slashName
	}
	if title := strings.TrimSpace(request.Title); title != "" {
		preview.Title = title
	}
	if description := strings.TrimSpace(request.Description); description != "" {
		preview.Description = description
	}
	if err := validateWorkflowMetadata(preview.SlashName, preview.Title, preview.Description); err != nil {
		return nil, err
	}
	if loadedDraft != nil &&
		(loadedDraft.Preview.SlashName != preview.SlashName ||
			loadedDraft.Preview.Title != preview.Title ||
			loadedDraft.Preview.Description != preview.Description) {
		if drafts, supported := s.repository.(DraftRepository); supported {
			now := s.now().UTC()
			loadedDraft, err = drafts.AppendDraftVersion(
				ctx, ownerUserID, request.PreviewID, loadedDraft.HeadRevision,
				preview, now, now.Add(workflowPreviewTTL),
			)
			if err != nil {
				return nil, fmt.Errorf("%w: Draft head revision changed", ErrInvalidInput)
			}
			s.hydrateDraft(*loadedDraft)
			preview = cloneWorkflowPreview(loadedDraft.Preview)
		}
	}
	if previewRecord.sourceAgentID == "" {
		return nil, fmt.Errorf("%w: source Execution has no coordinator Agent", ErrInvalidInput)
	}
	workflows, err := s.repository.List(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	editorAgentID := previewRecord.sourceAgentID
	if s.agents != nil {
		mainAgent, resolveErr := s.agents.GetDefaultAgent(ctx)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve Nexus main Agent for WorkGraph editor: %w", resolveErr)
		}
		if mainAgent == nil || strings.TrimSpace(mainAgent.OwnerUserID) != ownerUserID || strings.TrimSpace(mainAgent.AgentID) == "" {
			return nil, errors.New("Nexus main Agent is unavailable for WorkGraph editor")
		}
		editorAgentID = strings.TrimSpace(mainAgent.AgentID)
	}

	editorID := newWorkflowEditorID()
	targetSessionKey := protocol.BuildAgentSessionKey(
		editorAgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		editorID,
		"",
	)
	now := s.now().UTC()
	displayAfter := now.UnixMilli()
	session, err := s.editorSessions.CreateWorkGraphEditorSession(ctx, EditorSessionCreateRequest{
		AgentID:               editorAgentID,
		TargetSessionKey:      targetSessionKey,
		DisplayAfterUnixMilli: displayAfter,
	})
	if err != nil {
		return nil, fmt.Errorf("create WorkGraph editor Session: %w", err)
	}
	expiresAt := preview.ExpiresAt
	if maxExpiry := now.Add(workflowPreviewTTL); expiresAt.After(maxExpiry) {
		expiresAt = maxExpiry
	}
	headRevision := int64(1)
	selectedRevision := int64(1)
	versions := []protocol.WorkGraphWorkflowPreviewVersion{{
		Revision: 1, Preview: cloneWorkflowPreview(preview), CreatedAt: now,
	}}
	if loadedDraft != nil {
		headRevision = loadedDraft.HeadRevision
		selectedRevision = loadedDraft.SelectedRevision
		versions = cloneWorkflowPreviewVersions(loadedDraft.Versions)
	}
	record := workflowEditorRecord{
		ownerUserID:           ownerUserID,
		sourceSessionKey:      request.SourceSessionKey,
		previewID:             request.PreviewID,
		language:              request.OutputLanguage,
		revision:              headRevision,
		selectedRevision:      selectedRevision,
		agentID:               editorAgentID,
		sessionKey:            session.SessionKey,
		displayAfterUnixMilli: displayAfter,
		preview:               preview,
		versions:              versions,
		unavailableSlashNames: unavailableWorkflowSlashNames(workflows),
		expiresAt:             expiresAt,
	}
	s.previewMu.Lock()
	editorKey := previewCacheKey(ownerUserID, editorID)
	s.editors[editorKey] = record
	s.editorBySession[record.sessionKey] = editorKey
	s.editorByPreview[previewCacheKey(ownerUserID, request.PreviewID)] = editorKey
	s.previewMu.Unlock()
	if drafts, supported := s.repository.(DraftRepository); supported {
		draft, bindErr := drafts.BindDraftEditor(
			ctx, ownerUserID, request.PreviewID, editorID, editorAgentID,
			record.sessionKey, displayAfter, now,
		)
		if bindErr != nil {
			return nil, bindErr
		}
		if draft != nil {
			s.hydrateDraft(*draft)
			s.previewMu.Lock()
			record = s.editors[editorKey]
			record.unavailableSlashNames = unavailableWorkflowSlashNames(workflows)
			s.editors[editorKey] = record
			s.previewMu.Unlock()
		}
	}
	return editorSession(editorID, record), nil
}

// GetMetadataEditor 返回隐藏专用 DM 的当前选中草图和不可变版本目录；消息仍由普通 Session API/WS 读取。
func (s *Service) GetMetadataEditor(
	ownerUserID string,
	request protocol.GetWorkGraphWorkflowEditorRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	record, editorID, err := s.getEditorRecord(ownerUserID, request.SourceSessionKey, request.EditorID)
	if err != nil {
		return nil, err
	}
	return editorSession(editorID, record), nil
}

// RuntimeEditorPolicy 为 exact 隐藏编辑 Session 提供完整当前草图与唯一允许的 Skill + 结构化命令路径。
func (s *Service) RuntimeEditorPolicy(
	ownerUserID string,
	sessionKey string,
) (protocol.ScopedSessionRuntimePolicy, bool, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || ownerUserID == "" || sessionKey == "" {
		return protocol.ScopedSessionRuntimePolicy{}, false, nil
	}
	if err := s.loadDraftByEditorSession(ownerUserID, sessionKey); err != nil {
		return protocol.ScopedSessionRuntimePolicy{}, false, err
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(s.now().UTC())
	key := s.editorBySession[sessionKey]
	record, ok := s.editors[key]
	if !ok || record.ownerUserID != ownerUserID || record.sessionKey != sessionKey {
		return protocol.ScopedSessionRuntimePolicy{}, false, nil
	}
	payload, err := json.Marshal(editorRevisionRequest(record.revision, record.preview))
	if err != nil {
		return protocol.ScopedSessionRuntimePolicy{}, false, err
	}
	unavailableNames, err := json.Marshal(record.unavailableSlashNames)
	if err != nil {
		return protocol.ScopedSessionRuntimePolicy{}, false, err
	}
	versionDirectory, err := json.Marshal(previewVersionSummaries(record.versions, record.selectedRevision))
	if err != nil {
		return protocol.ScopedSessionRuntimePolicy{}, false, err
	}
	languageRule := "title、description、objective、completion_criteria 以及节点 subject、objective、deliverable、acceptance_criteria 均使用简洁自然的中文；可翻译的标题不要保留纯英文。最终回复也使用中文"
	if record.language == "en" {
		languageRule = "Write title, description, objective, completion criteria, every node's subject/objective/deliverable/acceptance criteria, and the final reply in concise, natural English"
	}
	prompt := fmt.Sprintf(`你正在 Nexus 主智能体的隐藏 WorkGraph 草图编辑 Session 中。只处理这张草图，不执行图中任务，也不读取 workspace；来源会话只以当前草图及其来源 WorkGraph 事实提供，不继承来源聊天权限。
先加载 execution-orchestrator Skill，并阅读其中的 WorkGraph authoring 说明。需要修改时，通过 nexus_runtime.command 读取 revise_workgraph_preview 的 fresh execution contract，按 contract 提交带 head_revision 的完整草图；不能只提交差异，也不能调用 execution inspect。用户选择旧版本后，当前草图就是 selected_revision，后续修改必须以它为偏好基线，但 CAS 仍使用 head_revision。command 成功后右侧预览会由宿主自动刷新；只有用户询问展示位置或刷新状态时才说明这个界面事实，正常修改回复不必反复提右侧。不要在左侧复述完整节点；用户只是提问时可以直接回答，也不能声称未发生的更新。
%s。不要在回复中输出内部 objective JSON、工具参数或源 Execution identity。

当前草图（head_revision=%d，selected_revision=%d）：
%s

版本目录：%s

unavailable_slash_names：%s`, languageRule, record.revision, record.selectedRevision, payload, versionDirectory, unavailableNames)
	return protocol.ScopedSessionRuntimePolicy{
		SystemPrompt: prompt,
		ToolPolicy: protocol.RuntimeToolPolicy{
			AllowedTools: []string{"Skill", "nexus_runtime", "mcp__nexus_runtime__command"},
			DisallowedTools: []string{
				"Agent", "Edit", "Glob", "Grep", "Task", "WebFetch", "WebSearch",
				"nexus_visualize", "nexus_imagegen",
			},
		},
		AllowedSkillNames: []string{"execution-orchestrator"},
		DisableSkills:     false,
		DisableConnectors: true,
	}, true, nil
}

// RuntimeEditorActive 判断 exact owner/session 是否仍拥有未过期的草图修改授权。
func (s *Service) RuntimeEditorActive(ownerUserID string, sessionKey string) bool {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || ownerUserID == "" || sessionKey == "" {
		return false
	}
	if err := s.loadDraftByEditorSession(ownerUserID, sessionKey); err != nil {
		return false
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(s.now().UTC())
	key := s.editorBySession[sessionKey]
	record, ok := s.editors[key]
	return ok && record.ownerUserID == ownerUserID && record.sessionKey == sessionKey
}

// ReviseEditorPreview 接收受限 Execution CLI 的完整草图提交，并以 revision CAS 追加 durable 版本。
func (s *Service) ReviseEditorPreview(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	request protocol.ReviseWorkGraphWorkflowPreviewRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if ownerUserID == "" || sessionKey == "" || request.Revision <= 0 {
		return nil, fmt.Errorf("%w: editor mutation scope is incomplete", ErrInvalidInput)
	}
	s.previewMu.Lock()
	s.cleanupExpiredPreviews(s.now().UTC())
	key := s.editorBySession[sessionKey]
	record, ok := s.editors[key]
	s.previewMu.Unlock()
	if !ok || record.ownerUserID != ownerUserID || record.sessionKey != sessionKey {
		return nil, ErrNotFound
	}
	if record.revision != request.Revision {
		return nil, fmt.Errorf("%w: editor revision changed", ErrInvalidInput)
	}
	next, err := normalizeAndValidateEditorPreview(record.preview, request)
	if err != nil {
		return nil, err
	}
	savedID := ""
	if draft, draftErr := s.loadDraftByID(ctx, ownerUserID, record.previewID); draftErr != nil {
		return nil, draftErr
	} else {
		savedID = savedWorkflowID(draft)
	}
	if s.repository != nil {
		existing, lookupErr := s.repository.GetBySlashName(ctx, ownerUserID, next.SlashName)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if _, reserved := reservedWorkflowSlashNames[next.SlashName]; reserved &&
			!canKeepLegacyBuiltinSlashName(next.SlashName, existing, savedID) {
			return nil, fmt.Errorf("%w: /%s", ErrNameConflict, next.SlashName)
		}
		if existing != nil && existing.ID != savedID {
			return nil, fmt.Errorf("%w: /%s", ErrNameConflict, next.SlashName)
		}
	}
	s.previewMu.Lock()
	s.cleanupExpiredPreviews(s.now().UTC())
	record, ok = s.editors[key]
	if !ok || record.revision != request.Revision || record.sessionKey != sessionKey {
		s.previewMu.Unlock()
		return nil, fmt.Errorf("%w: editor revision changed", ErrInvalidInput)
	}
	s.previewMu.Unlock()
	now := s.now().UTC()
	if drafts, supported := s.repository.(DraftRepository); supported {
		draft, appendErr := drafts.AppendDraftVersion(
			ctx, ownerUserID, record.previewID, request.Revision, next,
			now, now.Add(workflowPreviewTTL),
		)
		if appendErr != nil {
			return nil, fmt.Errorf("%w: editor revision changed", ErrInvalidInput)
		}
		s.hydrateDraft(*draft)
		record = s.editors[key]
	} else {
		s.previewMu.Lock()
		latest, exists := s.editors[key]
		if !exists || latest.revision != request.Revision {
			s.previewMu.Unlock()
			return nil, fmt.Errorf("%w: editor revision changed", ErrInvalidInput)
		}
		record = latest
		record.preview = next
		record.revision++
		record.selectedRevision = record.revision
		record.versions = append(record.versions, protocol.WorkGraphWorkflowPreviewVersion{
			Revision: record.revision, Preview: cloneWorkflowPreview(next), CreatedAt: now,
		})
		s.editors[key] = record
		s.previewMu.Unlock()
	}
	editorID := editorIDFromCacheKey(key)
	return editorSession(editorID, record), nil
}

// SelectMetadataEditorVersion 选择既有不可变版本作为当前编辑与应用基线。
func (s *Service) SelectMetadataEditorVersion(
	ctx context.Context,
	ownerUserID string,
	request protocol.SelectWorkGraphWorkflowEditorVersionRequest,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	record, editorID, err := s.getEditorRecord(
		ownerUserID,
		request.SourceSessionKey,
		request.EditorID,
	)
	if err != nil {
		return nil, err
	}
	if request.Revision != record.revision || request.SelectedRevision <= 0 {
		return nil, fmt.Errorf("%w: editor revision changed", ErrInvalidInput)
	}
	if drafts, supported := s.repository.(DraftRepository); supported {
		draft, selectErr := drafts.SelectDraftVersion(
			ctx, strings.TrimSpace(ownerUserID), record.previewID,
			record.revision, request.SelectedRevision, s.now().UTC(),
		)
		if selectErr != nil {
			return nil, fmt.Errorf("%w: selected editor version is unavailable", ErrInvalidInput)
		}
		s.hydrateDraft(*draft)
		updated, _, getErr := s.getEditorRecord(ownerUserID, request.SourceSessionKey, request.EditorID)
		if getErr != nil {
			return nil, getErr
		}
		return editorSession(editorID, updated), nil
	}
	selected, ok := previewVersion(record.versions, request.SelectedRevision)
	if !ok {
		return nil, fmt.Errorf("%w: selected editor version is unavailable", ErrInvalidInput)
	}
	s.previewMu.Lock()
	key := previewCacheKey(strings.TrimSpace(ownerUserID), editorID)
	record = s.editors[key]
	record.selectedRevision = request.SelectedRevision
	record.preview = cloneWorkflowPreview(selected.Preview)
	s.editors[key] = record
	s.previewMu.Unlock()
	return editorSession(editorID, record), nil
}

// SelectEditorVersionBySession 是隐藏编辑 Session 的 host-bound CLI 入口。
func (s *Service) SelectEditorVersionBySession(
	ctx context.Context,
	ownerUserID string,
	sessionKey string,
	headRevision int64,
	selectedRevision int64,
) (*protocol.WorkGraphWorkflowEditorSession, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if err := s.loadDraftByEditorSession(ownerUserID, sessionKey); err != nil {
		return nil, err
	}
	s.previewMu.Lock()
	key := s.editorBySession[sessionKey]
	record, ok := s.editors[key]
	s.previewMu.Unlock()
	if !ok || record.ownerUserID != ownerUserID {
		return nil, ErrNotFound
	}
	return s.SelectMetadataEditorVersion(ctx, ownerUserID, protocol.SelectWorkGraphWorkflowEditorVersionRequest{
		SourceSessionKey: record.sourceSessionKey,
		EditorID:         editorIDFromCacheKey(key), Revision: headRevision,
		SelectedRevision: selectedRevision,
	})
}

// ApplyMetadataEditor 把 exact editor 选中版本投影回当前 UI preview；不会改写源 Execution 或源聊天。
func (s *Service) ApplyMetadataEditor(
	ownerUserID string,
	request protocol.ApplyWorkGraphWorkflowEditorRequest,
) (*protocol.WorkGraphWorkflowPreview, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	request.SourceSessionKey = strings.TrimSpace(request.SourceSessionKey)
	request.EditorID = strings.TrimSpace(request.EditorID)
	if ownerUserID == "" || request.SourceSessionKey == "" || request.EditorID == "" || request.Revision <= 0 {
		return nil, fmt.Errorf("%w: editor apply request is incomplete", ErrInvalidInput)
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(s.now().UTC())
	editorKey := previewCacheKey(ownerUserID, request.EditorID)
	record, ok := s.editors[editorKey]
	if !ok || record.sourceSessionKey != request.SourceSessionKey {
		return nil, ErrNotFound
	}
	if record.revision != request.Revision {
		return nil, fmt.Errorf("%w: editor revision changed", ErrInvalidInput)
	}
	previewKey := previewCacheKey(ownerUserID, record.previewID)
	previewRecord, ok := s.previews[previewKey]
	if !ok || previewRecord.preview.SourceSessionKey != request.SourceSessionKey || previewRecord.saveScheduled {
		return nil, ErrNotFound
	}
	previewRecord.preview = cloneWorkflowPreview(record.preview)
	s.previews[previewKey] = previewRecord
	result := cloneWorkflowPreview(record.preview)
	return &result, nil
}

// CloseMetadataEditor 是显式丢弃隐藏编辑 Session 的管理入口；普通关闭页面不会调用它。
func (s *Service) CloseMetadataEditor(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
	editorID string,
) (bool, error) {
	record, normalizedEditorID, err := s.getEditorRecord(ownerUserID, sourceSessionKey, editorID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if s.editorSessions == nil {
		return false, errors.New("workgraph editor Session manager is unavailable")
	}
	if err = s.editorSessions.DeleteWorkGraphEditorSession(ctx, record.sessionKey); err != nil {
		return false, err
	}
	s.previewMu.Lock()
	delete(s.editorBySession, record.sessionKey)
	delete(s.editorByPreview, previewCacheKey(strings.TrimSpace(ownerUserID), record.previewID))
	delete(s.editors, previewCacheKey(strings.TrimSpace(ownerUserID), normalizedEditorID))
	s.previewMu.Unlock()
	return true, nil
}

func (s *Service) getEditorRecord(ownerUserID string, sourceSessionKey string, editorID string) (workflowEditorRecord, string, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sourceSessionKey = strings.TrimSpace(sourceSessionKey)
	editorID = strings.TrimSpace(editorID)
	if s == nil || ownerUserID == "" || sourceSessionKey == "" || editorID == "" {
		return workflowEditorRecord{}, "", fmt.Errorf("%w: editor scope is incomplete", ErrInvalidInput)
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(s.now().UTC())
	record, ok := s.editors[previewCacheKey(ownerUserID, editorID)]
	if !ok || record.ownerUserID != ownerUserID || record.sourceSessionKey != sourceSessionKey {
		return workflowEditorRecord{}, "", ErrNotFound
	}
	return cloneEditorRecord(record), editorID, nil
}

func editorSession(editorID string, record workflowEditorRecord) *protocol.WorkGraphWorkflowEditorSession {
	return &protocol.WorkGraphWorkflowEditorSession{
		EditorID:              editorID,
		Revision:              record.revision,
		SelectedRevision:      record.selectedRevision,
		AgentID:               record.agentID,
		SessionKey:            record.sessionKey,
		DisplayAfterUnixMilli: record.displayAfterUnixMilli,
		Preview:               cloneWorkflowPreview(record.preview),
		Versions:              previewVersionSummaries(record.versions, record.selectedRevision),
		ExpiresAt:             record.expiresAt,
	}
}

func cloneEditorRecord(record workflowEditorRecord) workflowEditorRecord {
	record.preview = cloneWorkflowPreview(record.preview)
	record.versions = cloneWorkflowPreviewVersions(record.versions)
	record.unavailableSlashNames = slices.Clone(record.unavailableSlashNames)
	return record
}

func previewVersion(
	versions []protocol.WorkGraphWorkflowPreviewVersion,
	revision int64,
) (protocol.WorkGraphWorkflowPreviewVersion, bool) {
	for _, version := range versions {
		if version.Revision == revision {
			version.Preview = cloneWorkflowPreview(version.Preview)
			return version, true
		}
	}
	return protocol.WorkGraphWorkflowPreviewVersion{}, false
}

func previewVersionSummaries(
	versions []protocol.WorkGraphWorkflowPreviewVersion,
	selectedRevision int64,
) []protocol.WorkGraphWorkflowPreviewVersionSummary {
	result := make([]protocol.WorkGraphWorkflowPreviewVersionSummary, 0, len(versions))
	for _, version := range versions {
		result = append(result, protocol.WorkGraphWorkflowPreviewVersionSummary{
			Revision: version.Revision, SlashName: version.Preview.SlashName,
			Title: version.Preview.Title, NodeCount: len(version.Preview.Nodes),
			DependencyCount: len(version.Preview.Dependencies),
			Selected:        version.Revision == selectedRevision, CreatedAt: version.CreatedAt,
		})
	}
	return result
}

func editorIDFromCacheKey(key string) string {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func editorRevisionRequest(revision int64, preview protocol.WorkGraphWorkflowPreview) protocol.ReviseWorkGraphWorkflowPreviewRequest {
	nodes := cloneWorkflowNodes(preview.Nodes)
	for index := range nodes {
		nodes[index].SourceWorkItemID = ""
	}
	return protocol.ReviseWorkGraphWorkflowPreviewRequest{
		Revision:           revision,
		SlashName:          preview.SlashName,
		Title:              preview.Title,
		Description:        preview.Description,
		Objective:          preview.Objective,
		CompletionCriteria: slices.Clone(preview.CompletionCriteria),
		Nodes:              nodes,
		Dependencies:       slices.Clone(preview.Dependencies),
	}
}

func normalizeAndValidateEditorPreview(
	current protocol.WorkGraphWorkflowPreview,
	request protocol.ReviseWorkGraphWorkflowPreviewRequest,
) (protocol.WorkGraphWorkflowPreview, error) {
	request.SlashName = normalizeSlashName(request.SlashName)
	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)
	request.Objective = strings.TrimSpace(request.Objective)
	request.CompletionCriteria = cleanStrings(request.CompletionCriteria)
	if err := validateWorkflowMetadata(request.SlashName, request.Title, request.Description); err != nil {
		return protocol.WorkGraphWorkflowPreview{}, err
	}
	if request.Objective == "" || len([]rune(request.Objective)) > 1000 || len(request.Nodes) == 0 || len(request.Nodes) > workGraphEditorMaxNodes || len(request.Dependencies) > workGraphEditorMaxEdges {
		return protocol.WorkGraphWorkflowPreview{}, fmt.Errorf("%w: edited graph is incomplete or too large", ErrInvalidInput)
	}
	dependenciesByNode := make(map[string][]orchestrationsvc.PlanDependencyDraft, len(request.Nodes))
	for _, edge := range request.Dependencies {
		logicalKey := strings.TrimSpace(edge.LogicalKey)
		dependenciesByNode[logicalKey] = append(
			dependenciesByNode[logicalKey],
			orchestrationsvc.PlanDependencyDraft{
				LogicalKey: strings.TrimSpace(edge.DependsOnLogicalKey),
				Kind:       edge.Kind,
			},
		)
	}
	draft := orchestrationsvc.PlanDraft{
		RevisionReason: "workgraph preview edit",
		Items:          make([]orchestrationsvc.PlanWorkItemDraft, 0, len(request.Nodes)),
	}
	roleByKey := make(map[string]protocol.WorkGraphWorkflowNodeRole, len(request.Nodes))
	for _, node := range request.Nodes {
		key := strings.TrimSpace(node.LogicalKey)
		if node.Role != protocol.WorkGraphWorkflowNodeKey && node.Role != protocol.WorkGraphWorkflowNodeCollaboration {
			return protocol.WorkGraphWorkflowPreview{}, fmt.Errorf("%w: edited node role must be key or collaboration", ErrInvalidInput)
		}
		if len([]rune(node.Subject)) > 200 || len([]rune(node.Objective)) > 1000 || len([]rune(node.Deliverable)) > 1000 || len(node.AcceptanceCriteria) > 32 {
			return protocol.WorkGraphWorkflowPreview{}, fmt.Errorf("%w: edited node content is too large", ErrInvalidInput)
		}
		roleByKey[key] = node.Role
		draft.Items = append(draft.Items, orchestrationsvc.PlanWorkItemDraft{
			LogicalKey:         key,
			Kind:               node.Kind,
			Subject:            node.Subject,
			Objective:          node.Objective,
			Deliverable:        node.Deliverable,
			AcceptanceCriteria: node.AcceptanceCriteria,
			Required:           node.Required,
			Terminal:           node.Terminal,
			ParentLogicalKey:   node.ParentLogicalKey,
			DependsOn:          dependenciesByNode[key],
		})
	}
	normalized, err := orchestrationsvc.NormalizeAndValidatePlanDraft(draft)
	if err != nil {
		return protocol.WorkGraphWorkflowPreview{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if err = validateEditorParentAcyclic(normalized.Items); err != nil {
		return protocol.WorkGraphWorkflowPreview{}, err
	}
	existingSourceID := make(map[string]string, len(current.Nodes))
	for _, node := range current.Nodes {
		existingSourceID[node.LogicalKey] = node.SourceWorkItemID
	}
	nodes := make([]protocol.WorkGraphWorkflowNode, 0, len(normalized.Items))
	hasKey := false
	hasTerminal := false
	for index, item := range normalized.Items {
		role := roleByKey[item.LogicalKey]
		hasKey = hasKey || role == protocol.WorkGraphWorkflowNodeKey
		hasTerminal = hasTerminal || item.Terminal
		nodes = append(nodes, protocol.WorkGraphWorkflowNode{
			LogicalKey:         item.LogicalKey,
			SourceWorkItemID:   existingSourceID[item.LogicalKey],
			Role:               role,
			Kind:               item.Kind,
			Subject:            item.Subject,
			Objective:          item.Objective,
			Deliverable:        item.Deliverable,
			AcceptanceCriteria: slices.Clone(item.AcceptanceCriteria),
			Required:           item.Required,
			Terminal:           item.Terminal,
			ParentLogicalKey:   item.ParentLogicalKey,
			Position:           index,
		})
	}
	if !hasKey || !hasTerminal {
		return protocol.WorkGraphWorkflowPreview{}, fmt.Errorf("%w: edited graph requires a key path and terminal delivery", ErrInvalidInput)
	}
	dependencies := make([]protocol.WorkGraphWorkflowDependency, 0, len(request.Dependencies))
	for _, item := range normalized.Items {
		for _, edge := range item.DependsOn {
			dependencies = append(dependencies, protocol.WorkGraphWorkflowDependency{
				LogicalKey:          item.LogicalKey,
				DependsOnLogicalKey: edge.LogicalKey,
				Kind:                edge.Kind,
			})
		}
	}
	current.SlashName = request.SlashName
	current.Title = request.Title
	current.Description = request.Description
	current.Objective = request.Objective
	current.CompletionCriteria = request.CompletionCriteria
	current.Nodes = nodes
	current.Dependencies = dependencies
	return current, nil
}

func validateWorkflowMetadata(slashName string, title string, description string) error {
	if !workflowSlashNamePattern.MatchString(strings.TrimSpace(slashName)) || strings.TrimSpace(title) == "" || strings.TrimSpace(description) == "" || len([]rune(title)) > 120 || len([]rune(description)) > 500 {
		return fmt.Errorf("%w: workflow metadata is invalid", ErrInvalidInput)
	}
	return nil
}

func validateEditorParentAcyclic(items []orchestrationsvc.PlanWorkItemDraft) error {
	parents := make(map[string]string, len(items))
	for _, item := range items {
		parents[item.LogicalKey] = item.ParentLogicalKey
	}
	for key := range parents {
		seen := map[string]struct{}{}
		for current := key; current != ""; current = parents[current] {
			if _, duplicate := seen[current]; duplicate {
				return fmt.Errorf("%w: parent graph contains a cycle", ErrInvalidInput)
			}
			seen[current] = struct{}{}
		}
	}
	return nil
}
