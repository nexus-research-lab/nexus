// INPUT: Room ledger、成员 private overlay/transcript refs 与分页索引共享内核。
// OUTPUT: 覆盖引用/resolver selection 且经 ledger/dependency 两阶段预算校验的 Room source 快照与 generation。
// POS: RoomHistoryStore 的 seekable history page 适配层。
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

func (s *RoomHistoryStore) historyPageAccess(
	ownerUserID string,
	conversationID string,
) historyPageIndexAccess {
	scope := historyPageScope(
		"room",
		fmt.Sprintf("v%d", historyPageIndexVersion),
		strings.TrimSpace(ownerUserID),
		strings.TrimSpace(conversationID),
	)
	return historyPageIndexAccess{
		Scope:     scope,
		ReadModel: s.readModel,
		OpenRoot: func(create bool) (*confinedfs.Root, error) {
			return s.openRoomHistoryPageIndexRoot(ownerUserID, conversationID, create)
		},
		ValidateSources: func(ctx context.Context, expected []historyPageSourceSnapshot) (bool, error) {
			return s.validateRoomHistoryPageSources(ctx, ownerUserID, conversationID, expected)
		},
		Build: func(ctx context.Context) (historyPageIndexBuild, error) {
			return s.buildRoomHistoryPageIndexWithSourceLimit(
				ctx,
				ownerUserID,
				conversationID,
				historyPageSourceMaxBytes,
			)
		},
		BuildCanonical: func(ctx context.Context) (historyPageIndexBuild, error) {
			return s.buildRoomHistoryPageIndex(ctx, ownerUserID, conversationID)
		},
	}
}

// ReadMessageDetailContext 读取当前 Room generation 中的大型消息 detail。
func (s *RoomHistoryStore) ReadMessageDetailContext(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	ref string,
) (HistoryMessageDetail, error) {
	access := s.historyPageAccess(ownerUserID, conversationID)
	return access.ReadModel.loadDetail(ctx, access, ref)
}

func (s *RoomHistoryStore) openRoomHistoryPageIndexRoot(
	ownerUserID string,
	conversationID string,
	_ bool,
) (*confinedfs.Root, error) {
	target := filepath.Join(
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
	)
	// Room conversation 被删除后，后台 rebuild 同样不得重新创建目录。
	parent, _, err := s.files.openRoomFileParent(ownerUserID, target, false)
	return parent, err
}

func (s *RoomHistoryStore) snapshotRoomHistoryLedger(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
) (historyPageSourceSnapshot, error) {
	base := historyPageSourceSnapshot{Kind: historyPageSourceRoomLedger}
	parent, name, err := s.files.openRoomFileParent(
		ownerUserID,
		s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
		false,
	)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return historyPageSourceSnapshot{}, err
	}
	defer parent.Close()
	return snapshotFileAtRoot(ctx, parent, name, base)
}

func (s *RoomHistoryStore) validateRoomHistoryPageSources(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	expected []historyPageSourceSnapshot,
) (bool, error) {
	if len(expected) == 0 || expected[0].Kind != historyPageSourceRoomLedger {
		return false, nil
	}
	ledger, err := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
	if err != nil || ledger != expected[0] {
		return false, err
	}
	ownerHistory := s.agentHistory.ForOwner(ownerUserID)
	for _, source := range expected[1:] {
		if err = ctx.Err(); err != nil {
			return false, err
		}
		var current historyPageSourceSnapshot
		switch source.Kind {
		case historyPageSourceRoomPrivateOverlay:
			current, err = ownerHistory.snapshotAgentHistoryOverlay(
				ctx,
				historyPageSourceRoomPrivateOverlay,
				source.WorkspacePath,
				source.SessionKey,
			)
		case historyPageSourceTranscript:
			matches, matchErr := ownerHistory.selectedAgentHistoryTranscriptSourceMatches(
				ctx,
				source.WorkspacePath,
				source.SessionID,
				source,
			)
			if matchErr != nil || !matches {
				return false, matchErr
			}
			continue
		default:
			return false, nil
		}
		if err != nil || current != source {
			return false, err
		}
	}
	return true, nil
}

type roomHistoryPageDependency struct {
	WorkspacePath     string
	PrivateSessionKey string
	SessionID         string
}

func (s *RoomHistoryStore) roomHistoryPageDependencies(
	ownerUserID string,
	rows []map[string]any,
) []roomHistoryPageDependency {
	unique := make(map[string]roomHistoryPageDependency)
	for _, row := range rows {
		if stringFromAny(row[overlayKindField]) != overlayKindTranscriptRef {
			continue
		}
		workspacePath := stringFromAny(row["workspace_path"])
		privateSessionKey := stringFromAny(row["private_session_key"])
		sessionID := stringFromAny(row["session_id"])
		if workspacePath == "" || privateSessionKey == "" || sessionID == "" ||
			stringFromAny(row["agent_id"]) == "" || stringFromAny(row["message_id"]) == "" ||
			!s.paths.workspacePathIsConfinedForOwner(ownerUserID, workspacePath) {
			continue
		}
		dependency := roomHistoryPageDependency{
			WorkspacePath:     filepath.Clean(workspacePath),
			PrivateSessionKey: privateSessionKey,
			SessionID:         sessionID,
		}
		key := strings.Join([]string{
			dependency.WorkspacePath,
			dependency.PrivateSessionKey,
			dependency.SessionID,
		}, "\x00")
		unique[key] = dependency
	}
	result := make([]roomHistoryPageDependency, 0, len(unique))
	for _, dependency := range unique {
		result = append(result, dependency)
	}
	sort.Slice(result, func(left int, right int) bool {
		leftKey := strings.Join([]string{
			result[left].WorkspacePath,
			result[left].PrivateSessionKey,
			result[left].SessionID,
		}, "\x00")
		rightKey := strings.Join([]string{
			result[right].WorkspacePath,
			result[right].PrivateSessionKey,
			result[right].SessionID,
		}, "\x00")
		return leftKey < rightKey
	})
	return result
}

func (s *RoomHistoryStore) collectRoomHistoryPageDependencySources(
	ctx context.Context,
	ownerUserID string,
	rows []map[string]any,
) ([]historyPageSourceSnapshot, error) {
	ownerHistory := s.agentHistory.ForOwner(ownerUserID)
	sources := make([]historyPageSourceSnapshot, 0)
	seen := make(map[string]struct{})
	for _, dependency := range s.roomHistoryPageDependencies(ownerUserID, rows) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		overlay, err := ownerHistory.snapshotAgentHistoryOverlay(
			ctx,
			historyPageSourceRoomPrivateOverlay,
			dependency.WorkspacePath,
			dependency.PrivateSessionKey,
		)
		if err != nil {
			return nil, err
		}
		if key := historyPageSourceKey(overlay); key != "" {
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				sources = append(sources, overlay)
			}
		}
		transcript, err := ownerHistory.snapshotAgentHistoryTranscript(
			ctx,
			dependency.WorkspacePath,
			dependency.SessionID,
		)
		if err != nil {
			return nil, err
		}
		if key := historyPageSourceKey(transcript); key != "" {
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				sources = append(sources, transcript)
			}
		}
	}
	sortHistoryPageSources(sources)
	return sources, nil
}

func (s *RoomHistoryStore) buildRoomHistoryPageIndex(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
) (historyPageIndexBuild, error) {
	return s.buildRoomHistoryPageIndexWithSourceLimit(ctx, ownerUserID, conversationID, 0)
}

func (s *RoomHistoryStore) buildRoomHistoryPageIndexWithSourceLimit(
	ctx context.Context,
	ownerUserID string,
	conversationID string,
	sourceLimit int64,
) (historyPageIndexBuild, error) {
	var latest historyPageIndexBuild
	for attempt := 0; attempt < historyPageBuildAttempts; attempt++ {
		ledgerBefore, err := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		if historyPageSourcesExceedLimit([]historyPageSourceSnapshot{ledgerBefore}, sourceLimit) {
			ledgerAfter, afterErr := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
			if afterErr != nil {
				return historyPageIndexBuild{}, afterErr
			}
			latest = historyPageIndexBuild{
				Sources:          []historyPageSourceSnapshot{ledgerAfter},
				Disabled:         true,
				DisabledCoverage: historyPageDisabledCoverageRoomLedger,
			}
			if ledgerBefore == ledgerAfter {
				latest.Cacheable = true
				return latest, nil
			}
			continue
		}
		rawRows, err := readRoomJSONLWithContext(
			ctx,
			s.files,
			ownerUserID,
			s.paths.RoomConversationOverlayPath(ownerUserID, conversationID),
		)
		if errors.Is(err, os.ErrNotExist) {
			rawRows = []map[string]any{}
		} else if err != nil {
			return historyPageIndexBuild{}, err
		}
		dependenciesBefore, err := s.collectRoomHistoryPageDependencySources(ctx, ownerUserID, rawRows)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		before := append([]historyPageSourceSnapshot{ledgerBefore}, dependenciesBefore...)
		if historyPageSourcesExceedLimit(before, sourceLimit) {
			ledgerAfter, afterErr := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
			if afterErr != nil {
				return historyPageIndexBuild{}, afterErr
			}
			dependenciesAfter, afterErr := s.collectRoomHistoryPageDependencySources(
				ctx,
				ownerUserID,
				rawRows,
			)
			if afterErr != nil {
				return historyPageIndexBuild{}, afterErr
			}
			after := append([]historyPageSourceSnapshot{ledgerAfter}, dependenciesAfter...)
			latest = historyPageIndexBuild{Sources: after, Disabled: true}
			if snapshotsEqual(before, after) {
				latest.Cacheable = true
				return latest, nil
			}
			continue
		}
		for _, source := range dependenciesBefore {
			if source.Kind == historyPageSourceTranscript && source.Exists {
				s.agentHistory.ForOwner(ownerUserID).invalidateTranscriptCache(source.ResolvedPath)
			}
		}
		resolved, err := s.resolveRoomHistoryRowsContext(ctx, ownerUserID, conversationID, rawRows)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		groups, err := buildHistoryPageIndexedGroups(ctx, resolved, true)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		roundIndex, err := s.readCanonicalRoundIndexContext(
			ctx,
			ownerUserID,
			conversationID,
			nil,
		)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		ledgerAfter, err := s.snapshotRoomHistoryLedger(ctx, ownerUserID, conversationID)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		dependenciesAfter, err := s.collectRoomHistoryPageDependencySources(ctx, ownerUserID, rawRows)
		if err != nil {
			return historyPageIndexBuild{}, err
		}
		after := append([]historyPageSourceSnapshot{ledgerAfter}, dependenciesAfter...)
		latest = historyPageIndexBuild{
			Groups:     groups,
			RoundIndex: roundIndex.Items,
			Sources:    after,
		}
		if snapshotsEqual(before, after) {
			latest.Cacheable = true
			return latest, nil
		}
	}
	return latest, nil
}
