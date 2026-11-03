// INPUT: 生产 durable Draft repository，或仅测试替身使用的进程内记录。
// OUTPUT: 同一 source Execution 复用、按 preview/session 直接查询、可续期空闲 lease 与请求内版本快照。
// POS: HTTP、对话 CLI 与隐藏编辑 Session 共用的 WorkGraph Draft 读取投影。
package workgraphworkflow

import (
	"context"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *Service) findReusableDraft(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
	sourceExecutionID string,
) (*protocol.WorkGraphWorkflowDraft, error) {
	if drafts, ok := s.repository.(DraftRepository); ok {
		draft, err := drafts.GetDraftBySource(ctx, ownerUserID, sourceSessionKey, sourceExecutionID)
		if err != nil || draft == nil {
			return draft, err
		}
		if err = s.renewDraftLease(ctx, drafts, draft); err != nil {
			return nil, err
		}
		return cloneWorkflowDraft(draft), nil
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	for _, record := range s.previews {
		if record.ownerUserID != ownerUserID ||
			record.preview.SourceSessionKey != sourceSessionKey ||
			record.preview.SourceExecutionID != sourceExecutionID {
			continue
		}
		return &protocol.WorkGraphWorkflowDraft{
			PreviewID: record.preview.PreviewID, OwnerUserID: ownerUserID,
			SourceExecutionID: sourceExecutionID, SourceSessionKey: sourceSessionKey,
			SourceAgentID: record.sourceAgentID, SourceConversationID: record.sourceConversationID,
			OutputLanguage: record.outputLanguage, HeadRevision: 1, SelectedRevision: 1,
			Preview: cloneWorkflowPreview(record.preview), ExpiresAt: record.preview.ExpiresAt,
			SaveScheduled: record.saveScheduled, SavedWorkflowID: record.savedWorkflowID,
			SavedRevision: record.savedRevision,
		}, nil
	}
	return nil, nil
}

func (s *Service) loadDraftByID(
	ctx context.Context,
	ownerUserID string,
	previewID string,
) (*protocol.WorkGraphWorkflowDraft, error) {
	if drafts, ok := s.repository.(DraftRepository); ok {
		draft, err := drafts.GetDraftByID(ctx, ownerUserID, previewID)
		if err != nil || draft == nil {
			return draft, err
		}
		if err = s.renewDraftLease(ctx, drafts, draft); err != nil {
			return nil, err
		}
		return cloneWorkflowDraft(draft), nil
	}
	return nil, nil
}

// editorRecordBySession resolves a fresh snapshot; a missing durable row must
// never resurrect an old process-local editor authority.
func (s *Service) editorRecordBySession(ctx context.Context, ownerUserID, sessionKey string) (workflowEditorRecord, string, error) {
	if drafts, ok := s.repository.(DraftRepository); ok {
		draft, err := drafts.GetDraftByEditorSession(ctx, ownerUserID, sessionKey)
		if err != nil {
			return workflowEditorRecord{}, "", err
		}
		if draft == nil {
			return workflowEditorRecord{}, "", ErrNotFound
		}
		if err = s.renewDraftLease(ctx, drafts, draft); err != nil {
			return workflowEditorRecord{}, "", err
		}
		return editorRecordFromDraft(*draft), draft.EditorID, nil
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	s.cleanupExpiredPreviews(s.now().UTC())
	key := s.editorBySession[sessionKey]
	record, ok := s.editors[key]
	if !ok || record.ownerUserID != ownerUserID || record.sessionKey != sessionKey {
		return workflowEditorRecord{}, "", ErrNotFound
	}
	return cloneEditorRecord(record), editorIDFromCacheKey(key), nil
}

func (s *Service) renewDraftLease(
	ctx context.Context,
	drafts DraftRepository,
	draft *protocol.WorkGraphWorkflowDraft,
) error {
	if draft == nil {
		return nil
	}
	now := s.now().UTC()
	// Avoid a write on every read while still making the 30-day lifetime an
	// idle lease rather than a deadline that destroys a resumable editor.
	if draft.ExpiresAt.After(now.Add(workflowPreviewTTL / 2)) {
		return nil
	}
	expiresAt := now.Add(workflowPreviewTTL)
	if err := drafts.RenewDraftLease(ctx, draft.OwnerUserID, draft.PreviewID, expiresAt, now); err != nil {
		return err
	}
	draft.ExpiresAt = expiresAt
	draft.UpdatedAt = now
	draft.Preview.ExpiresAt = expiresAt
	for index := range draft.Versions {
		if draft.Versions[index].Revision == draft.SelectedRevision {
			draft.Versions[index].Preview.ExpiresAt = expiresAt
		}
	}
	return nil
}

func previewRecordFromDraft(draft protocol.WorkGraphWorkflowDraft) workflowPreviewRecord {
	return workflowPreviewRecord{
		ownerUserID: draft.OwnerUserID, preview: cloneWorkflowPreview(draft.Preview),
		sourceAgentID: draft.SourceAgentID, sourceConversationID: draft.SourceConversationID,
		outputLanguage: draft.OutputLanguage, saveScheduled: draft.SaveScheduled,
		savedWorkflowID: draft.SavedWorkflowID, savedRevision: draft.SavedRevision,
	}
}

func editorRecordFromDraft(draft protocol.WorkGraphWorkflowDraft) workflowEditorRecord {
	return workflowEditorRecord{
		ownerUserID: draft.OwnerUserID, sourceSessionKey: draft.SourceSessionKey,
		previewID: draft.PreviewID, language: draft.OutputLanguage,
		revision: draft.HeadRevision, selectedRevision: draft.SelectedRevision,
		agentID: draft.EditorAgentID, sessionKey: draft.EditorSessionKey,
		displayAfterUnixMilli: draft.EditorDisplayAfter, preview: cloneWorkflowPreview(draft.Preview),
		versions: cloneWorkflowPreviewVersions(draft.Versions), expiresAt: draft.ExpiresAt,
	}
}

// ListDrafts 返回 exact Session 中已提取草图的紧凑目录，支持一个对话多张 WorkGraph。
func (s *Service) ListDrafts(
	ctx context.Context,
	ownerUserID string,
	sourceSessionKey string,
) ([]protocol.WorkGraphWorkflowDraftSummary, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	sourceSessionKey = strings.TrimSpace(sourceSessionKey)
	if ownerUserID == "" || sourceSessionKey == "" {
		return nil, ErrInvalidInput
	}
	if drafts, ok := s.repository.(DraftRepository); ok {
		items, err := drafts.ListDrafts(ctx, ownerUserID, sourceSessionKey)
		if err != nil {
			return nil, err
		}
		result := make([]protocol.WorkGraphWorkflowDraftSummary, 0, len(items))
		for index := range items {
			if err = s.renewDraftLease(ctx, drafts, &items[index]); err != nil {
				return nil, err
			}
			result = append(result, draftSummary(items[index]))
		}
		return result, nil
	}
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	result := make([]protocol.WorkGraphWorkflowDraftSummary, 0)
	for _, record := range s.previews {
		if record.ownerUserID != ownerUserID || record.preview.SourceSessionKey != sourceSessionKey {
			continue
		}
		result = append(result, protocol.WorkGraphWorkflowDraftSummary{
			PreviewID: record.preview.PreviewID, SourceExecutionID: record.preview.SourceExecutionID,
			SlashName: record.preview.SlashName, Title: record.preview.Title,
			HeadRevision: 1, SelectedRevision: 1, VersionCount: 1,
			NodeCount: len(record.preview.Nodes), SaveScheduled: record.saveScheduled,
			SavedWorkflowID: record.savedWorkflowID, SavedRevision: record.savedRevision,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PreviewID < result[j].PreviewID })
	return result, nil
}

func draftSummary(draft protocol.WorkGraphWorkflowDraft) protocol.WorkGraphWorkflowDraftSummary {
	return protocol.WorkGraphWorkflowDraftSummary{
		PreviewID: draft.PreviewID, SourceExecutionID: draft.SourceExecutionID,
		SlashName: draft.Preview.SlashName, Title: draft.Preview.Title,
		HeadRevision: draft.HeadRevision, SelectedRevision: draft.SelectedRevision,
		VersionCount: len(draft.Versions), NodeCount: len(draft.Preview.Nodes),
		SaveScheduled: draft.SaveScheduled, SavedWorkflowID: draft.SavedWorkflowID,
		SavedRevision: draft.SavedRevision,
		UpdatedAt:     draft.UpdatedAt,
	}
}

func cloneWorkflowDraft(draft *protocol.WorkGraphWorkflowDraft) *protocol.WorkGraphWorkflowDraft {
	if draft == nil {
		return nil
	}
	result := *draft
	result.Preview = cloneWorkflowPreview(draft.Preview)
	result.Versions = cloneWorkflowPreviewVersions(draft.Versions)
	return &result
}

func cloneWorkflowPreviewVersions(
	versions []protocol.WorkGraphWorkflowPreviewVersion,
) []protocol.WorkGraphWorkflowPreviewVersion {
	result := make([]protocol.WorkGraphWorkflowPreviewVersion, len(versions))
	for index := range versions {
		result[index] = versions[index]
		result[index].Preview = cloneWorkflowPreview(versions[index].Preview)
	}
	return result
}
