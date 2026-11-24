// INPUT: DM session 身份、overlay/runtime transcript 与分页索引共享内核。
// OUTPUT: resolver selection-aware 的 DM 快照、generation、可持久化的尾轮增量状态与删除安全索引根。
// POS: AgentHistoryStore 的 seekable history page 适配层。
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func (s *AgentHistoryStore) historyPageAccess(
	workspacePath string,
	sessionValue protocol.Session,
) historyPageIndexAccess {
	scopeParts := []string{
		"dm",
		historyPageRulesVersion(),
		filepath.Clean(strings.TrimSpace(workspacePath)),
		strings.TrimSpace(sessionValue.SessionKey),
		strings.TrimSpace(sessionValue.AgentID),
	}
	segmented, _ := sessionValue.Options[protocol.OptionRuntimeSegmentedTranscript].(bool)
	scopeParts = append(scopeParts, fmt.Sprintf("segmented=%t", segmented))
	scopeParts = append(scopeParts, historyTranscriptSessionIDs(sessionValue)...)
	scope := historyPageScope(scopeParts...)
	return historyPageIndexAccess{
		Scope:     scope,
		LogAttrs:  []any{"session_key", strings.TrimSpace(sessionValue.SessionKey)},
		ReadModel: s.readModel,
		Refresh: func(ctx context.Context) (historyPageIndexBuild, bool) {
			return s.refreshAgentHistoryTail(ctx, workspacePath, sessionValue, scope)
		},
		OpenRoot: func(create bool) (*confinedfs.Root, error) {
			return s.openAgentHistoryPageIndexRoot(
				workspacePath,
				sessionValue.SessionKey,
				create,
			)
		},
		ValidateSources: func(ctx context.Context, expected []historyPageSourceSnapshot) (bool, error) {
			return s.validateAgentHistoryPageSources(ctx, workspacePath, sessionValue, expected)
		},
		Build: func(ctx context.Context) (historyPageIndexBuild, error) {
			// DM 不设源字节上限：超长会话也要建读模型，之后每轮只从持久化的检查点续读新增内容；
			// 禁用读模型只会让每次翻页都在前台完整读一遍且结果不落盘。
			return s.buildAgentHistoryPageIndex(ctx, workspacePath, sessionValue, true)
		},
		ReadRoundIndex: func(ctx context.Context, active []string) (protocol.SessionRoundIndex, error) {
			return s.readCanonicalRoundIndexContext(ctx, workspacePath, sessionValue, active)
		},
		BuildCanonical: func(ctx context.Context) (historyPageIndexBuild, error) {
			return s.buildAgentHistoryPageIndex(ctx, workspacePath, sessionValue, false)
		},
	}
}

// ReadMessageDetailContext 读取当前 DM generation 中的大型消息 detail。
func (s *AgentHistoryStore) ReadMessageDetailContext(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	ref string,
) (HistoryMessageDetail, error) {
	access := s.historyPageAccess(workspacePath, sessionValue)
	return access.ReadModel.loadDetail(ctx, access, ref)
}

func (s *AgentHistoryStore) validateAgentHistoryPageSources(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	expected []historyPageSourceSnapshot,
) (bool, error) {
	sessionIDs := historyTranscriptSessionIDs(sessionValue)
	if len(expected) != len(sessionIDs)+1 || expected[0].Kind != historyPageSourceDMOverlay {
		return false, nil
	}
	overlay, err := s.snapshotAgentHistoryOverlay(
		ctx,
		historyPageSourceDMOverlay,
		workspacePath,
		sessionValue.SessionKey,
	)
	if err != nil || overlay != expected[0] {
		return false, err
	}
	for index, sessionID := range sessionIDs {
		source := expected[index+1]
		if source.Kind != historyPageSourceTranscript || source.SessionID != sessionID {
			return false, nil
		}
		matches, matchErr := s.selectedAgentHistoryTranscriptSourceMatches(
			ctx,
			workspacePath,
			sessionID,
			source,
		)
		if matchErr != nil || !matches {
			return false, matchErr
		}
	}
	return true, nil
}

func (s *AgentHistoryStore) selectedAgentHistoryTranscriptSourceMatches(
	ctx context.Context,
	workspacePath string,
	sessionID string,
	expected historyPageSourceSnapshot,
) (bool, error) {
	resolvedPath, err := s.resolveTranscriptPathContext(ctx, workspacePath, sessionID)
	if errors.Is(err, os.ErrNotExist) {
		missing := historyPageSourceSnapshot{
			Kind:          historyPageSourceTranscript,
			WorkspacePath: filepath.Clean(strings.TrimSpace(workspacePath)),
			SessionID:     strings.TrimSpace(sessionID),
		}
		return missing == expected, nil
	}
	if err != nil {
		return false, err
	}
	// Source validity includes resolver selection identity, not merely the old
	// fallback file identity. A newly appeared canonical transcript must fence
	// a still-existing lower-priority fallback generation.
	if strings.TrimSpace(expected.ResolvedPath) == "" ||
		!sameTranscriptPath(resolvedPath, expected.ResolvedPath) {
		return false, nil
	}
	current, err := s.snapshotAgentHistoryTranscriptAtResolvedPath(
		ctx,
		workspacePath,
		sessionID,
		resolvedPath,
	)
	return err == nil && current == expected, err
}

func (s *AgentHistoryStore) openAgentHistoryPageIndexRoot(
	workspacePath string,
	sessionKey string,
	_ bool,
) (*confinedfs.Root, error) {
	// 派生 rebuild 绝不能创建 canonical session 容器；删除与后台 build
	// 竞态时，缺失目录应使 persist 放弃，而不是复活孤儿会话。
	root, err := s.files.openWorkspaceRoot(workspacePath, false)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	relative := filepath.ToSlash(filepath.Join(
		".agents",
		"sessions",
		encodeSessionDirName(sessionKey),
	))
	sessionRoot, err := root.OpenRootNoSymlink(relative)
	if err != nil {
		return nil, err
	}
	stored, metaErr := readSessionMeta(sessionRoot, "meta.json")
	if metaErr != nil {
		sessionRoot.Close()
		return nil, metaErr
	}
	if strings.TrimSpace(stored.SessionKey) != strings.TrimSpace(sessionKey) {
		sessionRoot.Close()
		return nil, ErrSessionStorageIdentityMismatch
	}
	return sessionRoot, nil
}

func (s *AgentHistoryStore) collectAgentHistoryPageSources(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
) ([]historyPageSourceSnapshot, error) {
	overlay, err := s.snapshotAgentHistoryOverlay(
		ctx,
		historyPageSourceDMOverlay,
		workspacePath,
		sessionValue.SessionKey,
	)
	if err != nil {
		return nil, err
	}
	sources := []historyPageSourceSnapshot{overlay}
	for _, sessionID := range historyTranscriptSessionIDs(sessionValue) {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		source, snapshotErr := s.snapshotAgentHistoryTranscript(ctx, workspacePath, sessionID)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func (s *AgentHistoryStore) snapshotAgentHistoryOverlay(
	ctx context.Context,
	kind string,
	workspacePath string,
	sessionKey string,
) (historyPageSourceSnapshot, error) {
	base := historyPageSourceSnapshot{
		Kind:          kind,
		WorkspacePath: filepath.Clean(strings.TrimSpace(workspacePath)),
		SessionKey:    strings.TrimSpace(sessionKey),
	}
	root, err := s.files.openWorkspaceRoot(workspacePath, false)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	defer root.Close()
	relative := filepath.ToSlash(filepath.Join(
		".agents",
		"sessions",
		encodeSessionDirName(sessionKey),
		"overlay.jsonl",
	))
	return snapshotFileAtRoot(ctx, root, relative, base)
}

func (s *AgentHistoryStore) snapshotAgentHistoryTranscript(
	ctx context.Context,
	workspacePath string,
	sessionID string,
) (historyPageSourceSnapshot, error) {
	base := historyPageSourceSnapshot{
		Kind:          historyPageSourceTranscript,
		WorkspacePath: filepath.Clean(strings.TrimSpace(workspacePath)),
		SessionID:     strings.TrimSpace(sessionID),
	}
	transcriptPath, err := s.resolveTranscriptPathContext(ctx, workspacePath, sessionID)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	return s.snapshotAgentHistoryTranscriptAtResolvedPath(ctx, workspacePath, sessionID, transcriptPath)
}

func (s *AgentHistoryStore) snapshotAgentHistoryTranscriptAtResolvedPath(
	ctx context.Context,
	workspacePath string,
	sessionID string,
	transcriptPath string,
) (historyPageSourceSnapshot, error) {
	base := historyPageSourceSnapshot{
		Kind:          historyPageSourceTranscript,
		WorkspacePath: filepath.Clean(strings.TrimSpace(workspacePath)),
		SessionID:     strings.TrimSpace(sessionID),
		ResolvedPath:  filepath.Clean(strings.TrimSpace(transcriptPath)),
	}
	root, relative, info, err := s.openTranscriptPath(workspacePath, transcriptPath)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	defer root.Close()
	file, err := s.openTranscriptReadFile(root, relative)
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	defer file.Close()
	return snapshotOpenedFile(ctx, file, info, base)
}

// buildAgentHistoryPageIndex 完整重建读模型；withTail 时同时建立可持久化的尾轮增量状态。
func (s *AgentHistoryStore) buildAgentHistoryPageIndex(
	ctx context.Context,
	workspacePath string,
	sessionValue protocol.Session,
	withTail bool,
) (historyPageIndexBuild, error) {
	var latest historyPageIndexBuild
	for attempt := 0; attempt < historyPageBuildAttempts; attempt++ {
		before, err := s.collectAgentHistoryPageSources(ctx, workspacePath, sessionValue)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		parts, err := s.readHistoryRowPartsContext(ctx, workspacePath, sessionValue, withTail)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		groups, err := buildHistoryPageIndexedGroups(ctx, parts.merged(), false)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		roundIndex, err := s.readCanonicalRoundIndexContext(
			ctx,
			workspacePath,
			sessionValue,
			nil,
		)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		after, err := s.collectAgentHistoryPageSources(ctx, workspacePath, sessionValue)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		latest = historyPageIndexBuild{
			Groups:     groups,
			RoundIndex: roundIndex.Items,
			Sources:    after,
		}
		if snapshotsEqual(before, after) {
			latest.Cacheable = true
			// 尾轮状态随本次 generation 在同一事务内发布，不会与分组或来源快照脱节。
			latest.TailState, latest.TailKeys = newAgentHistoryTailState(ctx, parts, groups)
			return latest, nil
		}
	}
	// 持续写入时仍返回最后一次 canonical 投影，但不持久化不一致快照。
	return latest, nil
}
