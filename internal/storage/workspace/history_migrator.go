// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：history_migrator.go
// @Date   ：2026/04/19 15:40:00
// @Author ：leemysw
// 2026/04/19 15:40:00   Create
// =====================================================

package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	roommodel "github.com/nexus-research-lab/nexus/internal/model/room"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// HistoryMigrationChange 表示单个历史对象的迁移结果。
type HistoryMigrationChange struct {
	Migrated            bool   `json:"migrated"`
	MovedLegacyDir      bool   `json:"moved_legacy_dir"`
	LegacyRows          int    `json:"legacy_rows"`
	RemovedInvalidRows  int    `json:"removed_invalid_rows"`
	WrittenRoundMarkers int    `json:"written_round_markers"`
	WrittenOverlayRows  int    `json:"written_overlay_rows"`
	WrittenRefs         int    `json:"written_refs"`
	RemovedLegacyFile   bool   `json:"removed_legacy_file"`
	Target              string `json:"target"`
}

type roomTranscriptSource struct {
	WorkspacePath     string
	PrivateSessionKey string
	SessionID         string
	MessageIndexByID  map[string]sessionmodel.Message
}

// OrphanPruneReport 表示孤儿目录清理结果。
type OrphanPruneReport struct {
	RemovedPaths int `json:"removed_paths"`
}

// HistoryMigrator 负责把旧版 messages.jsonl 迁移到 transcript + overlay 机制。
type HistoryMigrator struct {
	paths      *Store
	files      *SessionFileStore
	agentStore *AgentHistoryStore
}

// NewHistoryMigrator 创建历史迁移门面。
func NewHistoryMigrator(root string) *HistoryMigrator {
	return &HistoryMigrator{
		paths:      New(root),
		files:      NewSessionFileStore(root),
		agentStore: NewAgentHistoryStore(root),
	}
}

// PruneOrphanSessionDirs 删除数据库与当前 session 视图都不存在的私有 session 目录。
func (m *HistoryMigrator) PruneOrphanSessionDirs(
	workspacePaths []string,
	validSessionKeysByWorkspace map[string]map[string]struct{},
) (OrphanPruneReport, error) {
	report := OrphanPruneReport{}
	for _, workspacePath := range workspacePaths {
		trimmedWorkspacePath := strings.TrimSpace(workspacePath)
		if trimmedWorkspacePath == "" {
			continue
		}
		keepNames := make(map[string]struct{})
		for sessionKey := range validSessionKeysByWorkspace[trimmedWorkspacePath] {
			keepNames[filepath.Base(m.paths.SessionDir(trimmedWorkspacePath, sessionKey))] = struct{}{}
		}
		removedPaths, err := m.pruneUnknownEntries(m.paths.SessionRoot(trimmedWorkspacePath), keepNames)
		if err != nil {
			return report, err
		}
		report.RemovedPaths += removedPaths
	}
	return report, nil
}

// PruneOrphanRoomConversationDirs 删除数据库中不存在的 room conversation 目录。
func (m *HistoryMigrator) PruneOrphanRoomConversationDirs(
	validConversationIDs map[string]struct{},
) (OrphanPruneReport, error) {
	keepNames := make(map[string]struct{}, len(validConversationIDs))
	for conversationID := range validConversationIDs {
		trimmedConversationID := strings.TrimSpace(conversationID)
		if trimmedConversationID == "" {
			continue
		}
		keepNames[filepath.Base(m.paths.RoomConversationDir(trimmedConversationID))] = struct{}{}
	}
	removedPaths, err := m.pruneUnknownEntries(m.paths.RoomConversationRoot(), keepNames)
	if err != nil {
		return OrphanPruneReport{}, err
	}
	return OrphanPruneReport{RemovedPaths: removedPaths}, nil
}

// MigrateAgentSession 把单个私有 session 的旧历史迁移到 overlay。
func (m *HistoryMigrator) MigrateAgentSession(
	workspacePath string,
	sessionValue sessionmodel.Session,
) (HistoryMigrationChange, error) {
	sessionValue = cloneSessionModel(sessionValue)
	isRoomBackedSession := isRoomBackedAgentSession(sessionValue)
	report := HistoryMigrationChange{
		Target: sessionValue.SessionKey,
	}
	if moved, removedInvalidRows, err := m.migrateSessionDirectoryLayout(workspacePath, sessionValue.SessionKey); err != nil {
		return report, err
	} else if moved {
		report.MovedLegacyDir = true
		report.Migrated = true
		report.RemovedInvalidRows += removedInvalidRows
	} else {
		report.RemovedInvalidRows += removedInvalidRows
	}
	legacyPath := m.paths.SessionMessagePath(workspacePath, sessionValue.SessionKey)
	legacyFileExists := true
	legacyRows, err := m.files.readMessagesFromPath(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		legacyFileExists = false
		sanitizedOverlayRows, removedInvalidRows, overlayChanged, err := m.sanitizeAgentOverlayAtPath(
			m.paths.SessionOverlayPath(workspacePath, sessionValue.SessionKey),
			sessionValue.SessionKey,
			sessionValue.AgentID,
		)
		if err != nil {
			return report, err
		}
		report.RemovedInvalidRows += removedInvalidRows
		if overlayChanged {
			report.Migrated = true
		}
		if len(sanitizedOverlayRows) == 0 {
			if isRoomBackedSession {
				if _, err := m.files.DeleteSession(workspacePath, sessionValue.SessionKey); err != nil {
					return report, err
				}
				return report, nil
			}
			if err := removeFileIfExists(m.paths.SessionOverlayPath(workspacePath, sessionValue.SessionKey)); err != nil {
				return report, err
			}
		}
		if !report.Migrated {
			return report, nil
		}
		if isRoomBackedSession {
			if overlayChanged {
				if err := m.files.replaceJSONL(
					m.paths.SessionOverlayPath(workspacePath, sessionValue.SessionKey),
					sanitizedOverlayRows,
				); err != nil {
					return report, err
				}
			}
			return report, nil
		}
		updatedSession := cloneSessionModel(sessionValue)
		if updatedSession.Options == nil {
			updatedSession.Options = map[string]any{}
		}
		updatedSession.Options[sessionmodel.OptionHistorySource] = sessionmodel.HistorySourceTranscript
		if overlayChanged {
			if err := m.files.replaceJSONL(
				m.paths.SessionOverlayPath(workspacePath, sessionValue.SessionKey),
				sanitizedOverlayRows,
			); err != nil {
				return report, err
			}
		}
		if _, err := m.files.UpsertSession(workspacePath, updatedSession); err != nil {
			return report, err
		}
		return report, nil
	}
	if err != nil {
		return report, err
	}

	report.LegacyRows = len(legacyRows)
	compactedRows := compactMessages(legacyRows)
	compactedRows, removedInvalidRows := sanitizeLegacyAgentMessages(
		compactedRows,
		sessionValue.SessionKey,
		sessionValue.AgentID,
	)
	report.RemovedInvalidRows += removedInvalidRows
	if strings.TrimSpace(stringPointerValue(sessionValue.SessionID)) == "" {
		if inferredSessionID := deriveLegacySessionID(compactedRows); inferredSessionID != "" {
			sessionValue.SessionID = stringPointer(inferredSessionID)
		}
	}
	overlayPath := m.paths.SessionOverlayPath(workspacePath, sessionValue.SessionKey)
	existingRows, removedInvalidRows, _, err := m.sanitizeAgentOverlayAtPath(
		overlayPath,
		sessionValue.SessionKey,
		sessionValue.AgentID,
	)
	if err != nil {
		return report, err
	}
	report.RemovedInvalidRows += removedInvalidRows

	existingMarkers := indexAgentRoundMarkers(existingRows)
	existingOverlay := indexOverlayRows(existingRows)
	transcriptIDs, err := m.loadTranscriptMessageIDs(workspacePath, sessionValue)
	if err != nil {
		return report, err
	}

	for _, row := range compactedRows {
		role := strings.TrimSpace(stringFromAny(row["role"]))
		if role == "user" {
			marker := buildLegacyRoundMarkerRow(row)
			if marker == nil {
				continue
			}
			roundID := strings.TrimSpace(stringFromAny(marker["round_id"]))
			if roundID == "" {
				continue
			}
			if !sameJSONLRow(existingMarkers[roundID], marker) {
				existingMarkers[roundID] = marker
				report.WrittenRoundMarkers++
			}
			continue
		}

		messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
		if messageID != "" {
			if _, exists := transcriptIDs[messageID]; exists {
				continue
			}
		}

		overlayRow := cloneMessageMap(row)
		key := overlayRowKey(overlayRow)
		if key == "" {
			continue
		}
		if _, exists := existingOverlay[key]; exists {
			continue
		}
		existingOverlay[key] = overlayRow
		report.WrittenOverlayRows++
	}

	finalRows := materializeOverlayRows(existingMarkers, existingOverlay)
	if isRoomBackedSession && len(finalRows) == 0 {
		if _, err := m.files.DeleteSession(workspacePath, sessionValue.SessionKey); err != nil {
			return report, err
		}
		report.Migrated = true
	} else if !sameJSONLRows(existingRows, finalRows) {
		if len(finalRows) == 0 {
			if err := removeFileIfExists(overlayPath); err != nil {
				return report, err
			}
		} else {
			if err := m.files.replaceJSONL(overlayPath, finalRows); err != nil {
				return report, err
			}
		}
		report.Migrated = true
	}

	if isRoomBackedSession {
		if err := os.Remove(legacyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return report, err
		}
		if legacyFileExists {
			report.RemovedLegacyFile = true
			report.Migrated = true
		}
		return report, nil
	}

	updatedSession := cloneSessionModel(sessionValue)
	if updatedSession.Options == nil {
		updatedSession.Options = map[string]any{}
	}
	updatedSession.Options[sessionmodel.OptionHistorySource] = sessionmodel.HistorySourceTranscript
	refreshedRows, err := m.agentStore.ReadMessages(workspacePath, updatedSession, nil)
	if err != nil {
		return report, err
	}
	updatedSession.MessageCount = len(refreshedRows)
	if lastTimestamp := latestMessageTimestamp(refreshedRows); lastTimestamp > 0 {
		lastActivity := time.UnixMilli(lastTimestamp).UTC()
		if updatedSession.LastActivity.Before(lastActivity) {
			updatedSession.LastActivity = lastActivity
		}
	}
	if _, err := m.files.UpsertSession(workspacePath, updatedSession); err != nil {
		return report, err
	}

	if err := os.Remove(legacyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return report, err
	}
	if legacyFileExists {
		report.RemovedLegacyFile = true
		report.Migrated = true
	}
	return report, nil
}

func deriveLegacySessionID(rows []sessionmodel.Message) string {
	for _, row := range rows {
		if sessionID := strings.TrimSpace(stringFromAny(row["session_id"])); sessionID != "" {
			return sessionID
		}
	}
	return ""
}

// MigrateRoomConversation 把 Room 共享历史迁移到 inline overlay + transcript_ref。
func (m *HistoryMigrator) MigrateRoomConversation(
	contextValue roommodel.ConversationContextAggregate,
) (HistoryMigrationChange, error) {
	report := HistoryMigrationChange{
		Target: contextValue.Conversation.ID,
	}
	if moved, removedInvalidRows, err := m.migrateRoomConversationDirectoryLayout(
		contextValue.Conversation.ID,
	); err != nil {
		return report, err
	} else if moved {
		report.MovedLegacyDir = true
		report.Migrated = true
		report.RemovedInvalidRows += removedInvalidRows
	} else {
		report.RemovedInvalidRows += removedInvalidRows
	}
	legacyPath := m.paths.LegacyRoomConversationMessagePath(contextValue.Conversation.ID)
	legacyFileExists := true
	legacyRows, err := m.files.readMessagesFromPath(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		legacyFileExists = false
		sanitizedOverlayRows, removedInvalidRows, overlayChanged, err := m.sanitizeRoomOverlayAtPath(
			m.paths.RoomConversationOverlayPath(contextValue.Conversation.ID),
			contextValue.Conversation.ID,
			contextValue.Room.ID,
		)
		if err != nil {
			return report, err
		}
		report.RemovedInvalidRows += removedInvalidRows
		if len(sanitizedOverlayRows) == 0 {
			if _, err := m.files.DeleteRoomConversation(contextValue.Conversation.ID); err != nil {
				return report, err
			}
			if overlayChanged || report.MovedLegacyDir {
				report.Migrated = true
			}
			return report, nil
		}
		if overlayChanged {
			if err := m.files.replaceJSONL(
				m.paths.RoomConversationOverlayPath(contextValue.Conversation.ID),
				sanitizedOverlayRows,
			); err != nil {
				return report, err
			}
			report.Migrated = true
		}
		return report, nil
	}
	if err != nil {
		return report, err
	}
	report.LegacyRows = len(legacyRows)

	compactedRows := compactMessages(legacyRows)
	overlayPath := m.paths.RoomConversationOverlayPath(contextValue.Conversation.ID)
	existingRows, removedInvalidRows, _, err := m.sanitizeRoomOverlayAtPath(
		overlayPath,
		contextValue.Conversation.ID,
		contextValue.Room.ID,
	)
	if err != nil {
		return report, err
	}
	report.RemovedInvalidRows += removedInvalidRows
	existingOverlay := indexOverlayRows(existingRows)

	transcriptSources, err := m.buildRoomTranscriptSources(contextValue)
	if err != nil {
		return report, err
	}
	for _, row := range compactedRows {
		migratedRow, isRef, ok := sanitizeMigratedLegacyRoomRow(
			row,
			contextValue.Conversation.ID,
			contextValue.Room.ID,
			transcriptSources,
		)
		if !ok {
			continue
		}
		key := overlayRowKey(migratedRow)
		if key == "" {
			report.RemovedInvalidRows++
			continue
		}
		if _, exists := existingOverlay[key]; exists {
			continue
		}
		existingOverlay[key] = migratedRow
		if isRef {
			report.WrittenRefs++
			continue
		}
		report.WrittenOverlayRows++
	}

	finalRows := materializeOverlayRows(nil, existingOverlay)
	if len(finalRows) == 0 {
		if _, err := m.files.DeleteRoomConversation(contextValue.Conversation.ID); err != nil {
			return report, err
		}
		report.Migrated = true
	} else if !sameJSONLRows(existingRows, finalRows) {
		if err := m.files.replaceJSONL(overlayPath, finalRows); err != nil {
			return report, err
		}
		report.Migrated = true
	}

	if err := os.Remove(legacyPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return report, err
	}
	if legacyFileExists {
		report.RemovedLegacyFile = true
		report.Migrated = true
	}
	return report, nil
}

func (m *HistoryMigrator) loadTranscriptMessageIDs(
	workspacePath string,
	sessionValue sessionmodel.Session,
) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	sessionID := strings.TrimSpace(stringPointerValue(sessionValue.SessionID))
	if sessionID == "" {
		return result, nil
	}
	_, roundMarkers, err := m.agentStore.readOverlayRowsAndMarkers(workspacePath, sessionValue.SessionKey)
	if err != nil {
		return nil, err
	}
	transcriptRows, err := m.agentStore.readTranscriptMessages(
		workspacePath,
		sessionValue.SessionKey,
		sessionValue.AgentID,
		sessionID,
		roundMarkers,
	)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, row := range transcriptRows {
		if !sessionmodel.IsTranscriptNativeMessage(row) {
			continue
		}
		messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
		if messageID == "" {
			continue
		}
		result[messageID] = struct{}{}
	}
	return result, nil
}

func (m *HistoryMigrator) buildRoomTranscriptSources(
	contextValue roommodel.ConversationContextAggregate,
) (map[string]roomTranscriptSource, error) {
	memberWorkspaceByAgent := make(map[string]string, len(contextValue.MemberAgents))
	for _, agentValue := range contextValue.MemberAgents {
		if strings.TrimSpace(agentValue.AgentID) == "" || strings.TrimSpace(agentValue.WorkspacePath) == "" {
			continue
		}
		memberWorkspaceByAgent[strings.TrimSpace(agentValue.AgentID)] = strings.TrimSpace(agentValue.WorkspacePath)
	}

	result := make(map[string]roomTranscriptSource)
	for _, sessionRecord := range contextValue.Sessions {
		agentID := strings.TrimSpace(sessionRecord.AgentID)
		sessionID := strings.TrimSpace(sessionRecord.SDKSessionID)
		workspacePath := strings.TrimSpace(memberWorkspaceByAgent[agentID])
		if agentID == "" || sessionID == "" || workspacePath == "" {
			continue
		}
		privateSessionKey := protocol.BuildRoomAgentSessionKey(
			contextValue.Conversation.ID,
			agentID,
			contextValue.Room.RoomType,
		)
		_, roundMarkers, err := m.agentStore.readOverlayRowsAndMarkers(workspacePath, privateSessionKey)
		if err != nil {
			return nil, err
		}
		transcriptRows, err := m.agentStore.readTranscriptMessages(
			workspacePath,
			privateSessionKey,
			agentID,
			sessionID,
			roundMarkers,
		)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		current := roomTranscriptSource{
			WorkspacePath:     workspacePath,
			PrivateSessionKey: privateSessionKey,
			SessionID:         sessionID,
			MessageIndexByID:  indexRoomTranscriptMessages(transcriptRows),
		}
		previous, exists := result[agentID]
		if !exists || sessionRecord.IsPrimary || previous.SessionID == "" {
			result[agentID] = current
		}
	}
	return result, nil
}

func migrateLegacyRoomRow(
	row sessionmodel.Message,
	sources map[string]roomTranscriptSource,
) (map[string]any, bool) {
	if !sessionmodel.IsTranscriptNativeMessage(row) {
		return cloneMessageMap(row), false
	}
	agentID := strings.TrimSpace(stringFromAny(row["agent_id"]))
	messageID := strings.TrimSpace(stringFromAny(row["message_id"]))
	source, exists := sources[agentID]
	if !exists || messageID == "" {
		return cloneMessageMap(row), false
	}
	if _, exists = source.MessageIndexByID[messageID]; !exists {
		return cloneMessageMap(row), false
	}
	refSource := cloneMessage(row)
	refSource["session_id"] = source.SessionID
	reference := buildRoomTranscriptReference(refSource, source.WorkspacePath, source.PrivateSessionKey)
	if reference == nil {
		return cloneMessageMap(row), false
	}
	return reference, true
}

func sanitizeMigratedLegacyRoomRow(
	row sessionmodel.Message,
	conversationID string,
	roomID string,
	sources map[string]roomTranscriptSource,
) (map[string]any, bool, bool) {
	migratedRow, isRef := migrateLegacyRoomRow(row, sources)
	if migratedRow == nil {
		return nil, false, false
	}
	if isRef {
		reference := canonicalizeRoomTranscriptReference(
			sessionmodel.Message(migratedRow),
			conversationID,
			roomID,
		)
		if reference == nil {
			return nil, false, false
		}
		return reference, true, true
	}
	inlineRow, ok := sanitizeRoomInlineMessage(
		sessionmodel.Message(migratedRow),
		conversationID,
		roomID,
	)
	if !ok {
		return nil, false, false
	}
	return cloneMessageMap(inlineRow), false, true
}

func buildLegacyRoundMarkerRow(row sessionmodel.Message) map[string]any {
	roundID := firstNonEmpty(stringFromAny(row["round_id"]), stringFromAny(row["message_id"]))
	if roundID == "" {
		return nil
	}
	content := strings.TrimSpace(stringFromAny(row["content"]))
	if content == "" {
		return nil
	}
	return map[string]any{
		overlayKindField: overlayKindRoundMarker,
		"round_id":       roundID,
		"content":        content,
		"timestamp":      messageTimestamp(row),
	}
}

func (m *HistoryMigrator) migrateSessionDirectoryLayout(
	workspacePath string,
	sessionKey string,
) (bool, int, error) {
	currentDir := m.paths.SessionDir(workspacePath, sessionKey)
	return m.migrateDirectoryLayout(currentDir, []string{
		m.paths.CompactSessionDir(workspacePath, sessionKey),
		m.paths.LegacySessionDir(workspacePath, sessionKey),
	})
}

func (m *HistoryMigrator) migrateRoomConversationDirectoryLayout(
	conversationID string,
) (bool, int, error) {
	currentDir := m.paths.RoomConversationDir(conversationID)
	return m.migrateDirectoryLayout(currentDir, []string{
		m.paths.CompactRoomConversationDir(conversationID),
		m.paths.LegacyRoomConversationDir(conversationID),
	})
}

func (m *HistoryMigrator) migrateDirectoryLayout(
	currentDir string,
	legacyDirs []string,
) (bool, int, error) {
	moved := false
	removedInvalidRows := 0
	for _, legacyDir := range legacyDirs {
		if strings.TrimSpace(legacyDir) == "" || legacyDir == currentDir {
			continue
		}
		if _, err := os.Stat(legacyDir); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return false, 0, err
		}

		if _, err := os.Stat(currentDir); errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(currentDir), 0o755); err != nil {
				return false, 0, err
			}
			if err := os.Rename(legacyDir, currentDir); err != nil {
				return false, 0, err
			}
			moved = true
			continue
		} else if err != nil {
			return false, 0, err
		}

		currentRemovedInvalidRows, err := m.mergeDirectoryContents(currentDir, legacyDir)
		if err != nil {
			return false, 0, err
		}
		moved = true
		removedInvalidRows += currentRemovedInvalidRows
	}
	return moved, removedInvalidRows, nil
}

func (m *HistoryMigrator) mergeDirectoryContents(currentDir string, legacyDir string) (int, error) {
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return 0, err
	}
	removedInvalidRows := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		sourcePath := filepath.Join(legacyDir, entry.Name())
		targetPath := filepath.Join(currentDir, entry.Name())
		if _, err := os.Stat(targetPath); errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return 0, err
			}
			if err := os.Rename(sourcePath, targetPath); err != nil {
				return 0, err
			}
			continue
		} else if err != nil {
			return 0, err
		}

		switch entry.Name() {
		case "messages.jsonl", "overlay.jsonl":
			currentRemovedInvalidRows, err := m.mergeJSONLFiles(targetPath, sourcePath)
			if err != nil {
				return 0, err
			}
			removedInvalidRows += currentRemovedInvalidRows
		default:
			if err := os.Remove(sourcePath); err != nil {
				return 0, err
			}
		}
	}
	return removedInvalidRows, os.RemoveAll(legacyDir)
}

func (m *HistoryMigrator) mergeJSONLFiles(targetPath string, sourcePath string) (int, error) {
	targetRows, err := m.files.readJSONL(targetPath)
	if err != nil {
		return 0, err
	}
	sourceRows, err := m.files.readJSONL(sourcePath)
	if err != nil {
		return 0, err
	}

	merged := make([]map[string]any, 0, len(sourceRows)+len(targetRows))
	seen := make(map[string]struct{}, len(sourceRows)+len(targetRows))
	appendRows := func(rows []map[string]any) {
		for _, row := range rows {
			payload, marshalErr := json.Marshal(row)
			if marshalErr != nil {
				continue
			}
			key := string(payload)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, row)
		}
	}
	appendRows(sourceRows)
	appendRows(targetRows)
	if err := m.files.replaceJSONL(targetPath, merged); err != nil {
		return 0, err
	}
	return 0, os.Remove(sourcePath)
}

func (m *HistoryMigrator) pruneUnknownEntries(rootPath string, keepNames map[string]struct{}) (int, error) {
	entries, err := os.ReadDir(rootPath)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	removedPaths := 0
	for _, entry := range entries {
		entryName := strings.TrimSpace(entry.Name())
		if entryName == "" {
			continue
		}
		if _, exists := keepNames[entryName]; exists {
			continue
		}
		if err := os.RemoveAll(filepath.Join(rootPath, entryName)); err != nil {
			return removedPaths, err
		}
		removedPaths++
	}
	return removedPaths, nil
}

func indexAgentRoundMarkers(rows []map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, row := range rows {
		if strings.TrimSpace(stringFromAny(row[overlayKindField])) != overlayKindRoundMarker {
			continue
		}
		roundID := strings.TrimSpace(stringFromAny(row["round_id"]))
		if roundID == "" {
			continue
		}
		result[roundID] = cloneMessageMap(row)
	}
	return result
}

func indexOverlayRows(rows []map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for _, row := range rows {
		if strings.TrimSpace(stringFromAny(row[overlayKindField])) == overlayKindRoundMarker {
			continue
		}
		key := overlayRowKey(row)
		if key == "" {
			continue
		}
		result[key] = cloneMessageMap(row)
	}
	return result
}

func overlayRowKey(row map[string]any) string {
	kind := strings.TrimSpace(stringFromAny(row[overlayKindField]))
	if kind == overlayKindTranscriptRef {
		return strings.Join([]string{
			"ref",
			strings.TrimSpace(stringFromAny(row["workspace_path"])),
			strings.TrimSpace(stringFromAny(row["private_session_key"])),
			strings.TrimSpace(stringFromAny(row["session_id"])),
			strings.TrimSpace(stringFromAny(row["message_id"])),
		}, "\x00")
	}
	if kind == overlayKindRoundMarker {
		return "marker\x00" + strings.TrimSpace(stringFromAny(row["round_id"]))
	}
	if messageID := strings.TrimSpace(stringFromAny(row["message_id"])); messageID != "" {
		return "msg\x00" + messageID
	}
	return strings.Join([]string{
		"inline",
		strings.TrimSpace(stringFromAny(row["role"])),
		strings.TrimSpace(stringFromAny(row["subtype"])),
		fmt.Sprintf("%d", messageTimestamp(sessionmodel.Message(row))),
		strings.TrimSpace(stringFromAny(row["content"])),
		strings.TrimSpace(stringFromAny(row["result"])),
	}, "\x00")
}

func materializeOverlayRows(
	markers map[string]map[string]any,
	overlayRows map[string]map[string]any,
) []map[string]any {
	result := make([]map[string]any, 0, len(markers)+len(overlayRows))
	for _, row := range markers {
		result = append(result, cloneMessageMap(row))
	}
	for _, row := range overlayRows {
		result = append(result, cloneMessageMap(row))
	}
	sort.SliceStable(result, func(i int, j int) bool {
		leftTimestamp := messageTimestamp(sessionmodel.Message(result[i]))
		rightTimestamp := messageTimestamp(sessionmodel.Message(result[j]))
		if leftTimestamp != rightTimestamp {
			return leftTimestamp < rightTimestamp
		}
		leftWeight := overlayRowWeight(result[i])
		rightWeight := overlayRowWeight(result[j])
		if leftWeight != rightWeight {
			return leftWeight < rightWeight
		}
		return overlayRowKey(result[i]) < overlayRowKey(result[j])
	})
	return result
}

func overlayRowWeight(row map[string]any) int {
	switch strings.TrimSpace(stringFromAny(row[overlayKindField])) {
	case overlayKindRoundMarker:
		return 0
	case overlayKindTranscriptRef:
		return 2
	default:
		return 1
	}
}

func (m *HistoryMigrator) sanitizeAgentOverlayAtPath(
	overlayPath string,
	sessionKey string,
	agentID string,
) ([]map[string]any, int, bool, error) {
	rows, err := m.files.readJSONL(overlayPath)
	if errors.Is(err, os.ErrNotExist) {
		return []map[string]any{}, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	sanitizedRows, removedInvalidRows := sanitizeAgentOverlayRows(rows, sessionKey, agentID)
	return sanitizedRows, removedInvalidRows, !sameJSONLRows(rows, sanitizedRows), nil
}

func (m *HistoryMigrator) sanitizeRoomOverlayAtPath(
	overlayPath string,
	conversationID string,
	roomID string,
) ([]map[string]any, int, bool, error) {
	rows, err := m.files.readJSONL(overlayPath)
	if errors.Is(err, os.ErrNotExist) {
		return []map[string]any{}, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	sanitizedRows, removedInvalidRows := sanitizeRoomOverlayRows(rows, conversationID, roomID)
	return sanitizedRows, removedInvalidRows, !sameJSONLRows(rows, sanitizedRows), nil
}

func sanitizeLegacyAgentMessages(
	rows []sessionmodel.Message,
	sessionKey string,
	agentID string,
) ([]sessionmodel.Message, int) {
	result := make([]sessionmodel.Message, 0, len(rows))
	removedInvalidRows := 0
	for _, row := range rows {
		sanitizedRow, ok := sanitizeAgentMessageRow(row, sessionKey, agentID)
		if !ok {
			removedInvalidRows++
			continue
		}
		result = append(result, sanitizedRow)
	}
	return result, removedInvalidRows
}

func sanitizeLegacyRoomMessages(
	rows []sessionmodel.Message,
	conversationID string,
	roomID string,
) ([]sessionmodel.Message, int) {
	result := make([]sessionmodel.Message, 0, len(rows))
	removedInvalidRows := 0
	for _, row := range rows {
		sanitizedRow, ok := sanitizeRoomInlineMessage(row, conversationID, roomID)
		if !ok {
			removedInvalidRows++
			continue
		}
		result = append(result, sanitizedRow)
	}
	return result, removedInvalidRows
}

func sanitizeAgentOverlayRows(
	rows []map[string]any,
	sessionKey string,
	agentID string,
) ([]map[string]any, int) {
	sanitizedRows := make([]map[string]any, 0, len(rows))
	removedInvalidRows := 0
	for _, row := range rows {
		sanitizedRow, ok := sanitizeAgentOverlayRow(row, sessionKey, agentID)
		if !ok {
			removedInvalidRows++
			continue
		}
		sanitizedRows = append(sanitizedRows, sanitizedRow)
	}
	return materializeOverlayRows(
		indexAgentRoundMarkers(sanitizedRows),
		indexOverlayRows(sanitizedRows),
	), removedInvalidRows
}

func sanitizeRoomOverlayRows(
	rows []map[string]any,
	conversationID string,
	roomID string,
) ([]map[string]any, int) {
	sanitizedRows := make([]map[string]any, 0, len(rows))
	removedInvalidRows := 0
	for _, row := range rows {
		sanitizedRow, ok := sanitizeRoomOverlayRow(row, conversationID, roomID)
		if !ok {
			removedInvalidRows++
			continue
		}
		sanitizedRows = append(sanitizedRows, sanitizedRow)
	}
	return materializeOverlayRows(nil, indexOverlayRows(sanitizedRows)), removedInvalidRows
}

func sanitizeAgentOverlayRow(
	row map[string]any,
	sessionKey string,
	agentID string,
) (map[string]any, bool) {
	messageValue := cloneMessage(sessionmodel.Message(row))
	if sessionKey != "" && strings.TrimSpace(stringFromAny(messageValue["session_key"])) == "" {
		messageValue["session_key"] = sessionKey
	}
	if agentID != "" && strings.TrimSpace(stringFromAny(messageValue["agent_id"])) == "" {
		messageValue["agent_id"] = agentID
	}

	overlayKind := strings.TrimSpace(stringFromAny(messageValue[overlayKindField]))
	if overlayKind == overlayKindRoundMarker || strings.TrimSpace(stringFromAny(messageValue["role"])) == "user" {
		rowMarker := buildLegacyRoundMarkerRow(messageValue)
		if rowMarker == nil {
			return nil, false
		}
		return rowMarker, true
	}
	if overlayKind != "" {
		return nil, false
	}

	sanitizedRow, ok := sanitizeAgentMessageRow(messageValue, sessionKey, agentID)
	if !ok {
		return nil, false
	}
	return cloneMessageMap(sanitizedRow), true
}

func sanitizeRoomOverlayRow(
	row map[string]any,
	conversationID string,
	roomID string,
) (map[string]any, bool) {
	messageValue := cloneMessage(sessionmodel.Message(row))
	overlayKind := strings.TrimSpace(stringFromAny(messageValue[overlayKindField]))
	switch overlayKind {
	case overlayKindTranscriptRef:
		reference := canonicalizeRoomTranscriptReference(messageValue, conversationID, roomID)
		if reference == nil {
			return nil, false
		}
		return reference, true
	case "", overlayKindRoundMarker:
		sanitizedRow, ok := sanitizeRoomInlineMessage(messageValue, conversationID, roomID)
		if !ok {
			return nil, false
		}
		return cloneMessageMap(sanitizedRow), true
	default:
		return nil, false
	}
}

func sanitizeAgentMessageRow(
	row sessionmodel.Message,
	sessionKey string,
	agentID string,
) (sessionmodel.Message, bool) {
	sanitized := cloneMessage(row)
	if sessionKey != "" && strings.TrimSpace(stringFromAny(sanitized["session_key"])) == "" {
		sanitized["session_key"] = sessionKey
	}
	if agentID != "" && strings.TrimSpace(stringFromAny(sanitized["agent_id"])) == "" {
		sanitized["agent_id"] = agentID
	}
	role := strings.TrimSpace(stringFromAny(sanitized["role"]))
	if role == "" {
		return nil, false
	}
	if role == "user" {
		if buildLegacyRoundMarkerRow(sanitized) == nil {
			return nil, false
		}
		return sanitized, true
	}
	if !isMeaningfulHistoryMessage(sanitized) {
		return nil, false
	}
	return sanitized, true
}

func sanitizeRoomInlineMessage(
	row sessionmodel.Message,
	conversationID string,
	roomID string,
) (sessionmodel.Message, bool) {
	sanitized := cloneMessage(row)
	if conversationID != "" && strings.TrimSpace(stringFromAny(sanitized["conversation_id"])) == "" {
		sanitized["conversation_id"] = conversationID
	}
	if roomID != "" && strings.TrimSpace(stringFromAny(sanitized["room_id"])) == "" {
		sanitized["room_id"] = roomID
	}
	if strings.TrimSpace(stringFromAny(sanitized["session_key"])) == "" && conversationID != "" {
		sanitized["session_key"] = protocol.BuildRoomSharedSessionKey(conversationID)
	}

	role := strings.TrimSpace(stringFromAny(sanitized["role"]))
	if role == "" {
		return nil, false
	}
	if messageID := strings.TrimSpace(stringFromAny(sanitized["message_id"])); messageID == "" {
		if roundID := strings.TrimSpace(stringFromAny(sanitized["round_id"])); roundID != "" {
			sanitized["message_id"] = roundID
		}
	}
	if role == "user" && strings.TrimSpace(stringFromAny(sanitized["round_id"])) == "" {
		if messageID := strings.TrimSpace(stringFromAny(sanitized["message_id"])); messageID != "" {
			sanitized["round_id"] = messageID
		}
	}
	if strings.TrimSpace(stringFromAny(sanitized["message_id"])) == "" &&
		strings.TrimSpace(stringFromAny(sanitized["round_id"])) == "" {
		return nil, false
	}
	if !isMeaningfulHistoryMessage(sanitized) {
		return nil, false
	}
	return sanitized, true
}

func canonicalizeRoomTranscriptReference(
	row sessionmodel.Message,
	conversationID string,
	roomID string,
) map[string]any {
	sanitized := cloneMessage(row)
	if conversationID != "" && strings.TrimSpace(stringFromAny(sanitized["conversation_id"])) == "" {
		sanitized["conversation_id"] = conversationID
	}
	if roomID != "" && strings.TrimSpace(stringFromAny(sanitized["room_id"])) == "" {
		sanitized["room_id"] = roomID
	}
	if strings.TrimSpace(stringFromAny(sanitized["session_key"])) == "" && conversationID != "" {
		sanitized["session_key"] = protocol.BuildRoomSharedSessionKey(conversationID)
	}
	workspacePath := strings.TrimSpace(stringFromAny(sanitized["workspace_path"]))
	privateSessionKey := strings.TrimSpace(stringFromAny(sanitized["private_session_key"]))
	reference := buildRoomTranscriptReference(sanitized, workspacePath, privateSessionKey)
	if reference == nil {
		return nil
	}
	return reference
}

func isRoomBackedAgentSession(value sessionmodel.Session) bool {
	return strings.TrimSpace(stringPointerValue(value.ConversationID)) != "" ||
		strings.TrimSpace(stringPointerValue(value.RoomID)) != "" ||
		strings.TrimSpace(stringPointerValue(value.RoomSessionID)) != ""
}

func isMeaningfulHistoryMessage(row sessionmodel.Message) bool {
	role := strings.TrimSpace(stringFromAny(row["role"]))
	if role == "" {
		return false
	}
	if strings.TrimSpace(stringFromAny(row["content"])) != "" {
		return true
	}
	if len(normalizeMessageContentBlocks(row["content"])) > 0 {
		return true
	}
	if strings.TrimSpace(stringFromAny(row["result"])) != "" {
		return true
	}
	if strings.TrimSpace(stringFromAny(row["subtype"])) != "" {
		return true
	}
	return false
}

func latestMessageTimestamp(rows []sessionmodel.Message) int64 {
	var latest int64
	for _, row := range rows {
		timestamp := messageTimestamp(row)
		if timestamp > latest {
			latest = timestamp
		}
	}
	return latest
}

func cloneSessionModel(value sessionmodel.Session) sessionmodel.Session {
	cloned := value
	if len(value.Options) == 0 {
		cloned.Options = map[string]any{}
		return cloned
	}
	cloned.Options = cloneMessageMap(value.Options)
	return cloned
}

func sameJSONLRows(left []map[string]any, right []map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !sameJSONLRow(left[index], right[index]) {
			return false
		}
	}
	return true
}

func sameJSONLRow(left map[string]any, right map[string]any) bool {
	leftPayload, leftErr := json.Marshal(left)
	rightPayload, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftPayload) == string(rightPayload)
}

func removeFileIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
