// INPUT: exact WorkGraph preview、源 transcript identity 与临时 DM 中模型提交的完整草图版本。
// OUTPUT: 隐藏目录的短期 fork Session、CAS preview revision，以及应用前的 DAG/交付语义校验。
// POS: 对话式草图编辑边界；普通 DM 负责消息/流式 UI，本服务只拥有草图状态与受限修改授权。
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
	workGraphEditorToolName = "mcp__nexus_workgraph_editor__revise_workgraph_preview"
	workGraphEditorMaxNodes = 64
	workGraphEditorMaxEdges = 256
)

// EditorSessionManager 由组合层把 WorkGraph 状态连接到真实 DM transcript fork 与 Session 删除主链。
type EditorSessionManager interface {
	CreateWorkGraphEditorSession(context.Context, EditorSessionCreateRequest) (*protocol.Session, error)
	DeleteWorkGraphEditorSession(context.Context, string) error
}

// EditorSessionCreateRequest 不暴露给 HTTP；所有 identity 都来自已验证 preview/source Execution。
type EditorSessionCreateRequest struct {
	AgentID               string
	SourceSessionKey      string
	TargetSessionKey      string
	DisplayAfterUnixMilli int64
}

type workflowEditorRecord struct {
	ownerUserID           string
	sourceSessionKey      string
	previewID             string
	language              string
	revision              int64
	agentID               string
	sessionKey            string
	displayAfterUnixMilli int64
	preview               protocol.WorkGraphWorkflowPreview
	unavailableSlashNames []string
	expiresAt             time.Time
}

// StartMetadataEditor 从 preview 的源 Execution transcript 创建隐藏临时 DM 分支。
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

	s.previewMu.Lock()
	s.cleanupExpiredPreviews(s.now().UTC())
	previewRecord, ok := s.previews[previewCacheKey(ownerUserID, request.PreviewID)]
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
	if previewRecord.sourceAgentID == "" {
		return nil, fmt.Errorf("%w: source Execution has no coordinator Agent", ErrInvalidInput)
	}
	workflows, err := s.repository.List(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	sourceRuntimeSessionKey := preview.SourceSessionKey
	if protocol.IsRoomSharedSessionKey(sourceRuntimeSessionKey) {
		conversationID := previewRecord.sourceConversationID
		if conversationID == "" {
			conversationID = protocol.ParseRoomConversationID(sourceRuntimeSessionKey)
		}
		sourceRuntimeSessionKey = protocol.BuildRoomAgentSessionKey(
			conversationID,
			previewRecord.sourceAgentID,
			"group",
		)
	}

	editorID := newWorkflowEditorID()
	targetSessionKey := protocol.BuildAgentSessionKey(
		previewRecord.sourceAgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		editorID,
		"",
	)
	now := s.now().UTC()
	displayAfter := now.UnixMilli()
	session, err := s.editorSessions.CreateWorkGraphEditorSession(ctx, EditorSessionCreateRequest{
		AgentID:               previewRecord.sourceAgentID,
		SourceSessionKey:      sourceRuntimeSessionKey,
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
	record := workflowEditorRecord{
		ownerUserID:           ownerUserID,
		sourceSessionKey:      request.SourceSessionKey,
		previewID:             request.PreviewID,
		language:              request.OutputLanguage,
		revision:              1,
		agentID:               previewRecord.sourceAgentID,
		sessionKey:            session.SessionKey,
		displayAfterUnixMilli: displayAfter,
		preview:               preview,
		unavailableSlashNames: unavailableWorkflowSlashNames(workflows),
		expiresAt:             expiresAt,
	}
	s.previewMu.Lock()
	editorKey := previewCacheKey(ownerUserID, editorID)
	s.editors[editorKey] = record
	s.editorBySession[record.sessionKey] = editorKey
	s.previewMu.Unlock()
	return editorSession(editorID, record), nil
}

// GetMetadataEditor 返回临时 DM 已提交的最新草图版本；消息仍由普通 Session API/WS 读取。
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

// RuntimeEditorPolicy 为 exact 临时 Session 提供完整当前草图与唯一允许的修改工具。
func (s *Service) RuntimeEditorPolicy(
	ownerUserID string,
	sessionKey string,
) (protocol.ScopedSessionRuntimePolicy, bool, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sessionKey = strings.TrimSpace(sessionKey)
	if s == nil || ownerUserID == "" || sessionKey == "" {
		return protocol.ScopedSessionRuntimePolicy{}, false, nil
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
	languageRule := "title、description、objective、completion_criteria 以及节点 subject、objective、deliverable、acceptance_criteria 均使用简洁自然的中文；可翻译的标题不要保留纯英文。最终回复也使用中文"
	if record.language == "en" {
		languageRule = "Write title, description, objective, completion criteria, every node's subject/objective/deliverable/acceptance criteria, and the final reply in concise, natural English"
	}
	prompt := fmt.Sprintf(`你正在一个短期 WorkGraph 草图编辑 Session 中。只处理用户对这张草图的修改要求，不执行草图中的任务，也不读写 workspace。
允许修改 slash_name、title、description、objective、completion_criteria、nodes、父子结构与 dependencies；可以新增、删除、合并或拆分节点。slash_name 和 logical_key 使用英文标识，其余面向用户的字段遵循当前界面语言。slash_name 先尝试所有语义准确的单词候选，默认只使用一个简短、可辨识的英文词；只有这些单词都冲突时才使用两个短词，不得使用三个及以上词，也不能与下方 unavailable_slash_names 重复。
每次确认修改时，必须调用 revise_workgraph_preview，并提交修改后的完整草图；不能只提交差异。工具成功后再简短说明改了什么。若用户只是提问且不要求修改，可以直接回答。
%s。不要在回复中输出内部 objective JSON、工具参数或源 Execution identity。

当前草图（revision=%d）：
%s

unavailable_slash_names：%s`, languageRule, record.revision, payload, unavailableNames)
	return protocol.ScopedSessionRuntimePolicy{
		SystemPrompt: prompt,
		ToolPolicy: protocol.RuntimeToolPolicy{
			AllowedTools:    []string{workGraphEditorToolName},
			DisallowedTools: []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep", "Task", "WebFetch", "WebSearch"},
		},
		DisableSkills:     true,
		DisableConnectors: true,
	}, true, nil
}

// ReviseEditorPreview 接收受限 MCP 的完整草图提交，并以 revision CAS 推进临时版本。
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
	if _, reserved := reservedWorkflowSlashNames[next.SlashName]; reserved {
		return nil, fmt.Errorf("%w: /%s", ErrNameConflict, next.SlashName)
	}
	if s.repository != nil {
		existing, lookupErr := s.repository.GetBySlashName(ctx, ownerUserID, next.SlashName)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if existing != nil {
			return nil, fmt.Errorf("%w: /%s", ErrNameConflict, next.SlashName)
		}
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(s.now().UTC())
	record, ok = s.editors[key]
	if !ok || record.revision != request.Revision || record.sessionKey != sessionKey {
		return nil, fmt.Errorf("%w: editor revision changed", ErrInvalidInput)
	}
	record.preview = next
	record.revision++
	s.editors[key] = record
	editorID := editorIDFromCacheKey(key)
	return editorSession(editorID, record), nil
}

// ApplyMetadataEditor 把 exact editor revision 应用到原 preview；取消则不会改变原 preview。
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

// CloseMetadataEditor 删除临时 DM Session 与未应用的草图版本。
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
		AgentID:               record.agentID,
		SessionKey:            record.sessionKey,
		DisplayAfterUnixMilli: record.displayAfterUnixMilli,
		Preview:               cloneWorkflowPreview(record.preview),
		ExpiresAt:             record.expiresAt,
	}
}

func cloneEditorRecord(record workflowEditorRecord) workflowEditorRecord {
	record.preview = cloneWorkflowPreview(record.preview)
	record.unavailableSlashNames = slices.Clone(record.unavailableSlashNames)
	return record
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
