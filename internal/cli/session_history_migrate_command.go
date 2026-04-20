// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：session_history_migrate_command.go
// @Date   ：2026/04/19 15:50:00
// @Author ：leemysw
// 2026/04/19 15:50:00   Create
// =====================================================

package cli

import (
	"context"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/spf13/cobra"
)

type historyMigrationScopeSummary struct {
	Scanned             int                                     `json:"scanned"`
	Migrated            int                                     `json:"migrated"`
	MovedLegacyDirs     int                                     `json:"moved_legacy_dirs"`
	RemovedInvalidRows  int                                     `json:"removed_invalid_rows"`
	RemovedOrphanPaths  int                                     `json:"removed_orphan_paths"`
	RemovedLegacyFiles  int                                     `json:"removed_legacy_files"`
	WrittenRoundMarkers int                                     `json:"written_round_markers"`
	WrittenOverlayRows  int                                     `json:"written_overlay_rows"`
	WrittenRefs         int                                     `json:"written_refs"`
	ChangedItems        []workspacestore.HistoryMigrationChange `json:"changed_items,omitempty"`
}

type historyMigrationSummary struct {
	AgentSessions     historyMigrationScopeSummary `json:"agent_sessions"`
	RoomConversations historyMigrationScopeSummary `json:"room_conversations"`
}

func newSessionHistoryMigrateCommand(
	workspaceRoot string,
	agentService *agent2.Service,
	roomService *roomsvc.Service,
	sessionService *sessionsvc.Service,
) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate-history",
		Short: "把旧版历史与 session 目录布局迁移到 transcript + overlay 机制",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			migrator := workspacestore.NewHistoryMigrator(workspaceRoot)
			summary := historyMigrationSummary{}
			validSessionKeysByWorkspace := map[string]map[string]struct{}{}
			validConversationIDs := map[string]struct{}{}

			sessions, err := sessionService.ListSessions(ctx)
			if err != nil {
				return err
			}
			for _, sessionValue := range sessions {
				summary.AgentSessions.Scanned++
				agentValue, err := agentService.GetAgent(ctx, sessionValue.AgentID)
				if err != nil {
					return err
				}
				change, err := migrator.MigrateAgentSession(agentValue.WorkspacePath, sessionValue)
				if err != nil {
					return err
				}
				accumulateHistoryMigration(&summary.AgentSessions, change)
				if _, exists := validSessionKeysByWorkspace[agentValue.WorkspacePath]; !exists {
					validSessionKeysByWorkspace[agentValue.WorkspacePath] = map[string]struct{}{}
				}
				validSessionKeysByWorkspace[agentValue.WorkspacePath][sessionValue.SessionKey] = struct{}{}
			}

			rooms, err := roomService.ListRooms(ctx, 10_000)
			if err != nil {
				return err
			}
			for _, roomValue := range rooms {
				contexts, err := roomService.GetRoomContexts(ctx, roomValue.Room.ID)
				if err != nil {
					return err
				}
				for _, contextValue := range contexts {
					summary.RoomConversations.Scanned++
					change, err := migrator.MigrateRoomConversation(contextValue)
					if err != nil {
						return err
					}
					accumulateHistoryMigration(&summary.RoomConversations, change)
					validConversationIDs[contextValue.Conversation.ID] = struct{}{}
				}
			}

			agents, err := agentService.ListAgents(ctx)
			if err != nil {
				return err
			}
			workspacePaths := make([]string, 0, len(agents))
			for _, agentValue := range agents {
				workspacePath := agentValue.WorkspacePath
				if workspacePath == "" {
					continue
				}
				workspacePaths = append(workspacePaths, workspacePath)
				if _, exists := validSessionKeysByWorkspace[workspacePath]; !exists {
					validSessionKeysByWorkspace[workspacePath] = map[string]struct{}{}
				}
			}

			sessionPruneReport, err := migrator.PruneOrphanSessionDirs(workspacePaths, validSessionKeysByWorkspace)
			if err != nil {
				return err
			}
			summary.AgentSessions.RemovedOrphanPaths += sessionPruneReport.RemovedPaths

			roomPruneReport, err := migrator.PruneOrphanRoomConversationDirs(validConversationIDs)
			if err != nil {
				return err
			}
			summary.RoomConversations.RemovedOrphanPaths += roomPruneReport.RemovedPaths

			return emitJSON(map[string]any{
				"domain": "session",
				"action": "migrate_history",
				"item":   summary,
			})
		},
	}
}

func accumulateHistoryMigration(
	scope *historyMigrationScopeSummary,
	change workspacestore.HistoryMigrationChange,
) {
	if !change.Migrated {
		return
	}
	scope.Migrated++
	if change.MovedLegacyDir {
		scope.MovedLegacyDirs++
	}
	scope.RemovedInvalidRows += change.RemovedInvalidRows
	if change.RemovedLegacyFile {
		scope.RemovedLegacyFiles++
	}
	scope.WrittenRoundMarkers += change.WrittenRoundMarkers
	scope.WrittenOverlayRows += change.WrittenOverlayRows
	scope.WrittenRefs += change.WrittenRefs
	scope.ChangedItems = append(scope.ChangedItems, change)
}
