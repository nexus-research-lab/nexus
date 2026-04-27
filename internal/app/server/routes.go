package server

import "strings"

// mountRoutes 按功能域挂载全部 HTTP 路由。
func (s *Server) mountRoutes() {
	s.mountCoreRoutes()
	s.mountAgentRoutes()
	s.mountRoomRoutes()
	s.mountCapabilityRoutes()
	s.mountPlaceholderRoutes()
}

// mountCoreRoutes 挂载 HTTP 基础能力路由。
func (s *Server) mountCoreRoutes() {
	s.router.Get("/agent/v1/health", s.handlers.core.HandleHealth)
	s.router.Get("/agent/v1/auth/status", s.handlers.auth.HandleAuthStatus)
	s.router.Post("/agent/v1/auth/login", s.handlers.auth.HandleAuthLogin)
	s.router.Post("/agent/v1/auth/logout", s.handlers.auth.HandleAuthLogout)
	s.router.Get("/agent/v1/runtime/options", s.handlers.core.HandleRuntimeOptions)
	s.router.Get("/agent/v1/settings/profile", s.handlers.auth.HandlePersonalProfile)
	s.router.Patch("/agent/v1/settings/profile", s.handlers.auth.HandleUpdatePersonalProfile)
	s.router.Post("/agent/v1/settings/profile/password", s.handlers.auth.HandleChangePassword)
	s.router.Get("/agent/v1/settings/preferences", s.handlers.core.HandleGetPreferences)
	s.router.Patch("/agent/v1/settings/preferences", s.handlers.core.HandleUpdatePreferences)
	s.router.Get("/agent/v1/settings/providers", s.handlers.core.HandleListProviderConfigs)
	s.router.Get("/agent/v1/settings/providers/options", s.handlers.core.HandleListProviderOptions)
	s.router.Post("/agent/v1/settings/providers", s.handlers.core.HandleCreateProviderConfig)
	s.router.Put("/agent/v1/settings/providers/{provider}", s.handlers.core.HandleUpdateProviderConfig)
	s.router.Delete("/agent/v1/settings/providers/{provider}", s.handlers.core.HandleDeleteProviderConfig)
	s.router.Get("/agent/v1/chat/ws", s.handlers.websocket.HandleWebSocket)
}

// mountAgentRoutes 挂载 Agent、Session 与工作区相关路由。
func (s *Server) mountAgentRoutes() {
	s.router.Get("/agent/v1/agents", s.handlers.agent.HandleListAgents)
	s.router.Get("/agent/v1/agents/runtime/statuses", s.handlers.agent.HandleAgentRuntimeStatuses)
	s.router.Post("/agent/v1/agents", s.handlers.agent.HandleCreateAgent)
	s.router.Get("/agent/v1/agents/validate/name", s.handlers.agent.HandleValidateAgentName)
	s.router.Get("/agent/v1/agents/{agent_id}", s.handlers.agent.HandleGetAgent)
	s.router.Patch("/agent/v1/agents/{agent_id}", s.handlers.agent.HandleUpdateAgent)
	s.router.Delete("/agent/v1/agents/{agent_id}", s.handlers.agent.HandleDeleteAgent)
	s.router.Get("/agent/v1/agents/{agent_id}/sessions", s.handlers.agent.HandleListAgentSessions)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/files", s.handlers.workspace.HandleWorkspaceFiles)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/file", s.handlers.workspace.HandleWorkspaceFile)
	s.router.Put("/agent/v1/agents/{agent_id}/workspace/file", s.handlers.workspace.HandleUpdateWorkspaceFile)
	s.router.Post("/agent/v1/agents/{agent_id}/workspace/upload", s.handlers.workspace.HandleUploadWorkspaceFile)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/download", s.handlers.workspace.HandleDownloadWorkspaceFile)
	s.router.Post("/agent/v1/agents/{agent_id}/workspace/entry", s.handlers.workspace.HandleCreateWorkspaceEntry)
	s.router.Patch("/agent/v1/agents/{agent_id}/workspace/entry", s.handlers.workspace.HandleRenameWorkspaceEntry)
	s.router.Delete("/agent/v1/agents/{agent_id}/workspace/entry", s.handlers.workspace.HandleDeleteWorkspaceEntry)
	s.router.Get("/agent/v1/agents/{agent_id}/skills", s.handlers.skill.HandleAgentSkills)
	s.router.Post("/agent/v1/agents/{agent_id}/skills", s.handlers.skill.HandleInstallAgentSkill)
	s.router.Delete("/agent/v1/agents/{agent_id}/skills/{skill_name}", s.handlers.skill.HandleUninstallAgentSkill)

	s.router.Get("/agent/v1/sessions", s.handlers.agent.HandleListSessions)
	s.router.Post("/agent/v1/sessions", s.handlers.agent.HandleCreateSession)
	s.router.Patch("/agent/v1/sessions/{session_key}", s.handlers.agent.HandleUpdateSession)
	s.router.Delete("/agent/v1/sessions/{session_key}", s.handlers.agent.HandleDeleteSession)
}

// mountRoomRoutes 挂载 Room 与 Launcher 相关路由。
func (s *Server) mountRoomRoutes() {
	s.router.Get("/agent/v1/rooms/dm/{agent_id}", s.handlers.room.HandleEnsureDirectRoom)
	s.router.Get("/agent/v1/rooms", s.handlers.room.HandleListRooms)
	s.router.Post("/agent/v1/rooms", s.handlers.room.HandleCreateRoom)
	s.router.Get("/agent/v1/rooms/{room_id}", s.handlers.room.HandleGetRoom)
	s.router.Patch("/agent/v1/rooms/{room_id}", s.handlers.room.HandleUpdateRoom)
	s.router.Delete("/agent/v1/rooms/{room_id}", s.handlers.room.HandleDeleteRoom)
	s.router.Get("/agent/v1/rooms/{room_id}/contexts", s.handlers.room.HandleGetRoomContexts)
	s.router.Post("/agent/v1/rooms/{room_id}/members", s.handlers.room.HandleAddRoomMember)
	s.router.Delete("/agent/v1/rooms/{room_id}/members/{agent_id}", s.handlers.room.HandleRemoveRoomMember)
	s.router.Post("/agent/v1/rooms/{room_id}/conversations", s.handlers.room.HandleCreateConversation)
	s.router.Get("/agent/v1/rooms/{room_id}/conversations/{conversation_id}/messages", s.handlers.room.HandleConversationMessages)
	s.router.Patch("/agent/v1/rooms/{room_id}/conversations/{conversation_id}", s.handlers.room.HandleUpdateConversation)
	s.router.Delete("/agent/v1/rooms/{room_id}/conversations/{conversation_id}", s.handlers.room.HandleDeleteConversation)

	s.router.Post("/agent/v1/launcher/query", s.handlers.launcher.HandleLauncherQuery)
	s.router.Get("/agent/v1/launcher/bootstrap", s.handlers.launcher.HandleLauncherBootstrap)
	s.router.Get("/agent/v1/launcher/suggestions", s.handlers.launcher.HandleLauncherSuggestions)
}

// mountCapabilityRoutes 挂载技能、连接器、通道与自动化能力路由。
func (s *Server) mountCapabilityRoutes() {
	s.router.Get("/agent/v1/capability/summary", s.handlers.capability.HandleCapabilitySummary)

	s.router.Get("/agent/v1/skills", s.handlers.skill.HandleListSkills)
	s.router.Get("/agent/v1/skills/{skill_name}", s.handlers.skill.HandleGetSkillDetail)
	s.router.Post("/agent/v1/skills/import/local", s.handlers.skill.HandleImportLocalSkill)
	s.router.Post("/agent/v1/skills/import/git", s.handlers.skill.HandleImportGitSkill)
	s.router.Get("/agent/v1/skills/search/external", s.handlers.skill.HandleSearchExternalSkills)
	s.router.Get("/agent/v1/skills/external/preview", s.handlers.skill.HandlePreviewExternalSkill)
	s.router.Post("/agent/v1/skills/import/skills-sh", s.handlers.skill.HandleImportSkillsShSkill)
	s.router.Post("/agent/v1/skills/update-imported", s.handlers.skill.HandleUpdateImportedSkills)
	s.router.Post("/agent/v1/skills/{skill_name}/update", s.handlers.skill.HandleUpdateSingleSkill)
	s.router.Delete("/agent/v1/skills/{skill_name}", s.handlers.skill.HandleDeleteSkill)

	s.router.Get("/agent/v1/connectors", s.handlers.connector.HandleListConnectors)
	s.router.Get("/agent/v1/connectors/categories", s.handlers.connector.HandleConnectorCategories)
	s.router.Get("/agent/v1/connectors/count", s.handlers.connector.HandleConnectorCount)
	s.router.Get("/agent/v1/connectors/{connector_id}", s.handlers.connector.HandleConnectorDetail)
	s.router.Get("/agent/v1/connectors/{connector_id}/oauth-client", s.handlers.connector.HandleGetConnectorOAuthClient)
	s.router.Put("/agent/v1/connectors/{connector_id}/oauth-client", s.handlers.connector.HandleUpsertConnectorOAuthClient)
	s.router.Delete("/agent/v1/connectors/{connector_id}/oauth-client", s.handlers.connector.HandleDeleteConnectorOAuthClient)
	s.router.Get("/agent/v1/connectors/{connector_id}/auth-url", s.handlers.connector.HandleConnectorAuthURL)
	s.router.Post("/agent/v1/connectors/oauth/callback", s.handlers.connector.HandleConnectorOAuthCallback)
	s.router.Post("/agent/v1/connectors/{connector_id}/connect", s.handlers.connector.HandleConnectConnector)
	s.router.Post("/agent/v1/connectors/{connector_id}/disconnect", s.handlers.connector.HandleDisconnectConnector)

	s.router.Post("/agent/v1/channels/messages", s.handlers.channel.HandleChannelIngress)
	s.router.Post("/agent/v1/channels/internal/messages", s.handlers.channel.HandleInternalChannelIngress)
	s.router.Post("/agent/v1/channels/discord/messages", s.handlers.channel.HandleDiscordChannelIngress)
	s.router.Post("/agent/v1/channels/telegram/messages", s.handlers.channel.HandleTelegramChannelIngress)

	s.router.Get("/agent/v1/capability/scheduled/tasks", s.handlers.automation.HandleListScheduledTasks)
	s.router.Post("/agent/v1/capability/scheduled/tasks", s.handlers.automation.HandleCreateScheduledTask)
	s.router.Patch("/agent/v1/capability/scheduled/tasks/{job_id}", s.handlers.automation.HandleUpdateScheduledTask)
	s.router.Delete("/agent/v1/capability/scheduled/tasks/{job_id}", s.handlers.automation.HandleDeleteScheduledTask)
	s.router.Post("/agent/v1/capability/scheduled/tasks/{job_id}/run", s.handlers.automation.HandleRunScheduledTask)
	s.router.Patch("/agent/v1/capability/scheduled/tasks/{job_id}/status", s.handlers.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get("/agent/v1/capability/scheduled/tasks/{job_id}/runs", s.handlers.automation.HandleListScheduledTaskRuns)

	s.router.Get("/agent/v1/scheduled/tasks", s.handlers.automation.HandleListScheduledTasks)
	s.router.Post("/agent/v1/scheduled/tasks", s.handlers.automation.HandleCreateScheduledTask)
	s.router.Patch("/agent/v1/scheduled/tasks/{job_id}", s.handlers.automation.HandleUpdateScheduledTask)
	s.router.Delete("/agent/v1/scheduled/tasks/{job_id}", s.handlers.automation.HandleDeleteScheduledTask)
	s.router.Post("/agent/v1/scheduled/tasks/{job_id}/run", s.handlers.automation.HandleRunScheduledTask)
	s.router.Patch("/agent/v1/scheduled/tasks/{job_id}/status", s.handlers.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get("/agent/v1/scheduled/tasks/{job_id}/runs", s.handlers.automation.HandleListScheduledTaskRuns)

	s.router.Get("/agent/v1/automation/heartbeat/{agent_id}", s.handlers.automation.HandleGetHeartbeat)
	s.router.Put("/agent/v1/automation/heartbeat/{agent_id}", s.handlers.automation.HandleUpdateHeartbeat)
	s.router.Post("/agent/v1/automation/heartbeat/{agent_id}/wake", s.handlers.automation.HandleWakeHeartbeat)
}

// mountPlaceholderRoutes 挂载保留占位路由。
func (s *Server) mountPlaceholderRoutes() {
	for _, group := range []string{} {
		s.mountPlaceholderGroup(group)
	}
}

func (s *Server) mountPlaceholderGroup(group string) {
	base := strings.TrimPrefix(group, "/")
	s.router.HandleFunc("/agent/v1/"+base, s.api.HandleNotImplemented(group))
	s.router.HandleFunc("/agent/v1/"+base+"/*", s.api.HandleNotImplemented(group))
}
