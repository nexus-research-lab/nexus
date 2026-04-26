package gateway

import "strings"

// mountRoutes 按功能域挂载全部网关路由。
func (s *Server) mountRoutes() {
	s.mountCoreRoutes()
	s.mountAgentRoutes()
	s.mountRoomRoutes()
	s.mountCapabilityRoutes()
	s.mountPlaceholderRoutes()
}

// mountCoreRoutes 挂载网关基础能力路由。
func (s *Server) mountCoreRoutes() {
	s.router.Get("/agent/v1/health", s.core.HandleHealth)
	s.router.Get("/agent/v1/auth/status", s.auth.HandleAuthStatus)
	s.router.Post("/agent/v1/auth/login", s.auth.HandleAuthLogin)
	s.router.Post("/agent/v1/auth/logout", s.auth.HandleAuthLogout)
	s.router.Get("/agent/v1/runtime/options", s.core.HandleRuntimeOptions)
	s.router.Get("/agent/v1/settings/profile", s.auth.HandlePersonalProfile)
	s.router.Patch("/agent/v1/settings/profile", s.auth.HandleUpdatePersonalProfile)
	s.router.Post("/agent/v1/settings/profile/password", s.auth.HandleChangePassword)
	s.router.Get("/agent/v1/settings/preferences", s.core.HandleGetPreferences)
	s.router.Patch("/agent/v1/settings/preferences", s.core.HandleUpdatePreferences)
	s.router.Get("/agent/v1/settings/providers", s.core.HandleListProviderConfigs)
	s.router.Get("/agent/v1/settings/providers/options", s.core.HandleListProviderOptions)
	s.router.Post("/agent/v1/settings/providers", s.core.HandleCreateProviderConfig)
	s.router.Put("/agent/v1/settings/providers/{provider}", s.core.HandleUpdateProviderConfig)
	s.router.Delete("/agent/v1/settings/providers/{provider}", s.core.HandleDeleteProviderConfig)
	s.router.Get("/agent/v1/chat/ws", s.websocket.HandleWebSocket)
}

// mountAgentRoutes 挂载 Agent、Session 与工作区相关路由。
func (s *Server) mountAgentRoutes() {
	s.router.Get("/agent/v1/agents", s.agent.HandleListAgents)
	s.router.Get("/agent/v1/agents/runtime/statuses", s.agent.HandleAgentRuntimeStatuses)
	s.router.Post("/agent/v1/agents", s.agent.HandleCreateAgent)
	s.router.Get("/agent/v1/agents/validate/name", s.agent.HandleValidateAgentName)
	s.router.Get("/agent/v1/agents/{agent_id}", s.agent.HandleGetAgent)
	s.router.Patch("/agent/v1/agents/{agent_id}", s.agent.HandleUpdateAgent)
	s.router.Delete("/agent/v1/agents/{agent_id}", s.agent.HandleDeleteAgent)
	s.router.Get("/agent/v1/agents/{agent_id}/sessions", s.agent.HandleListAgentSessions)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/files", s.workspace.HandleWorkspaceFiles)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/file", s.workspace.HandleWorkspaceFile)
	s.router.Put("/agent/v1/agents/{agent_id}/workspace/file", s.workspace.HandleUpdateWorkspaceFile)
	s.router.Post("/agent/v1/agents/{agent_id}/workspace/upload", s.workspace.HandleUploadWorkspaceFile)
	s.router.Get("/agent/v1/agents/{agent_id}/workspace/download", s.workspace.HandleDownloadWorkspaceFile)
	s.router.Post("/agent/v1/agents/{agent_id}/workspace/entry", s.workspace.HandleCreateWorkspaceEntry)
	s.router.Patch("/agent/v1/agents/{agent_id}/workspace/entry", s.workspace.HandleRenameWorkspaceEntry)
	s.router.Delete("/agent/v1/agents/{agent_id}/workspace/entry", s.workspace.HandleDeleteWorkspaceEntry)
	s.router.Get("/agent/v1/agents/{agent_id}/skills", s.skill.HandleAgentSkills)
	s.router.Post("/agent/v1/agents/{agent_id}/skills", s.skill.HandleInstallAgentSkill)
	s.router.Delete("/agent/v1/agents/{agent_id}/skills/{skill_name}", s.skill.HandleUninstallAgentSkill)

	s.router.Get("/agent/v1/sessions", s.agent.HandleListSessions)
	s.router.Post("/agent/v1/sessions", s.agent.HandleCreateSession)
	s.router.Patch("/agent/v1/sessions/{session_key}", s.agent.HandleUpdateSession)
	s.router.Delete("/agent/v1/sessions/{session_key}", s.agent.HandleDeleteSession)
}

// mountRoomRoutes 挂载 Room 与 Launcher 相关路由。
func (s *Server) mountRoomRoutes() {
	s.router.Get("/agent/v1/rooms/dm/{agent_id}", s.room.HandleEnsureDirectRoom)
	s.router.Get("/agent/v1/rooms", s.room.HandleListRooms)
	s.router.Post("/agent/v1/rooms", s.room.HandleCreateRoom)
	s.router.Get("/agent/v1/rooms/{room_id}", s.room.HandleGetRoom)
	s.router.Patch("/agent/v1/rooms/{room_id}", s.room.HandleUpdateRoom)
	s.router.Delete("/agent/v1/rooms/{room_id}", s.room.HandleDeleteRoom)
	s.router.Get("/agent/v1/rooms/{room_id}/contexts", s.room.HandleGetRoomContexts)
	s.router.Post("/agent/v1/rooms/{room_id}/members", s.room.HandleAddRoomMember)
	s.router.Delete("/agent/v1/rooms/{room_id}/members/{agent_id}", s.room.HandleRemoveRoomMember)
	s.router.Post("/agent/v1/rooms/{room_id}/conversations", s.room.HandleCreateConversation)
	s.router.Get("/agent/v1/rooms/{room_id}/conversations/{conversation_id}/messages", s.room.HandleConversationMessages)
	s.router.Patch("/agent/v1/rooms/{room_id}/conversations/{conversation_id}", s.room.HandleUpdateConversation)
	s.router.Delete("/agent/v1/rooms/{room_id}/conversations/{conversation_id}", s.room.HandleDeleteConversation)

	s.router.Post("/agent/v1/launcher/query", s.launcher.HandleLauncherQuery)
	s.router.Get("/agent/v1/launcher/bootstrap", s.launcher.HandleLauncherBootstrap)
	s.router.Get("/agent/v1/launcher/suggestions", s.launcher.HandleLauncherSuggestions)
}

// mountCapabilityRoutes 挂载技能、连接器、通道与自动化能力路由。
func (s *Server) mountCapabilityRoutes() {
	s.router.Get("/agent/v1/capability/summary", s.capability.HandleCapabilitySummary)

	s.router.Get("/agent/v1/skills", s.skill.HandleListSkills)
	s.router.Get("/agent/v1/skills/{skill_name}", s.skill.HandleGetSkillDetail)
	s.router.Post("/agent/v1/skills/import/local", s.skill.HandleImportLocalSkill)
	s.router.Post("/agent/v1/skills/import/git", s.skill.HandleImportGitSkill)
	s.router.Get("/agent/v1/skills/search/external", s.skill.HandleSearchExternalSkills)
	s.router.Get("/agent/v1/skills/external/preview", s.skill.HandlePreviewExternalSkill)
	s.router.Post("/agent/v1/skills/import/skills-sh", s.skill.HandleImportSkillsShSkill)
	s.router.Post("/agent/v1/skills/update-imported", s.skill.HandleUpdateImportedSkills)
	s.router.Post("/agent/v1/skills/{skill_name}/update", s.skill.HandleUpdateSingleSkill)
	s.router.Delete("/agent/v1/skills/{skill_name}", s.skill.HandleDeleteSkill)

	s.router.Get("/agent/v1/connectors", s.connector.HandleListConnectors)
	s.router.Get("/agent/v1/connectors/categories", s.connector.HandleConnectorCategories)
	s.router.Get("/agent/v1/connectors/count", s.connector.HandleConnectorCount)
	s.router.Get("/agent/v1/connectors/{connector_id}", s.connector.HandleConnectorDetail)
	s.router.Get("/agent/v1/connectors/{connector_id}/oauth-client", s.connector.HandleGetConnectorOAuthClient)
	s.router.Put("/agent/v1/connectors/{connector_id}/oauth-client", s.connector.HandleUpsertConnectorOAuthClient)
	s.router.Delete("/agent/v1/connectors/{connector_id}/oauth-client", s.connector.HandleDeleteConnectorOAuthClient)
	s.router.Get("/agent/v1/connectors/{connector_id}/auth-url", s.connector.HandleConnectorAuthURL)
	s.router.Post("/agent/v1/connectors/oauth/callback", s.connector.HandleConnectorOAuthCallback)
	s.router.Post("/agent/v1/connectors/{connector_id}/connect", s.connector.HandleConnectConnector)
	s.router.Post("/agent/v1/connectors/{connector_id}/disconnect", s.connector.HandleDisconnectConnector)

	s.router.Post("/agent/v1/channels/messages", s.channel.HandleChannelIngress)
	s.router.Post("/agent/v1/channels/internal/messages", s.channel.HandleInternalChannelIngress)
	s.router.Post("/agent/v1/channels/discord/messages", s.channel.HandleDiscordChannelIngress)
	s.router.Post("/agent/v1/channels/telegram/messages", s.channel.HandleTelegramChannelIngress)

	s.router.Get("/agent/v1/capability/scheduled/tasks", s.automation.HandleListScheduledTasks)
	s.router.Post("/agent/v1/capability/scheduled/tasks", s.automation.HandleCreateScheduledTask)
	s.router.Patch("/agent/v1/capability/scheduled/tasks/{job_id}", s.automation.HandleUpdateScheduledTask)
	s.router.Delete("/agent/v1/capability/scheduled/tasks/{job_id}", s.automation.HandleDeleteScheduledTask)
	s.router.Post("/agent/v1/capability/scheduled/tasks/{job_id}/run", s.automation.HandleRunScheduledTask)
	s.router.Patch("/agent/v1/capability/scheduled/tasks/{job_id}/status", s.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get("/agent/v1/capability/scheduled/tasks/{job_id}/runs", s.automation.HandleListScheduledTaskRuns)

	s.router.Get("/agent/v1/scheduled/tasks", s.automation.HandleListScheduledTasks)
	s.router.Post("/agent/v1/scheduled/tasks", s.automation.HandleCreateScheduledTask)
	s.router.Patch("/agent/v1/scheduled/tasks/{job_id}", s.automation.HandleUpdateScheduledTask)
	s.router.Delete("/agent/v1/scheduled/tasks/{job_id}", s.automation.HandleDeleteScheduledTask)
	s.router.Post("/agent/v1/scheduled/tasks/{job_id}/run", s.automation.HandleRunScheduledTask)
	s.router.Patch("/agent/v1/scheduled/tasks/{job_id}/status", s.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get("/agent/v1/scheduled/tasks/{job_id}/runs", s.automation.HandleListScheduledTaskRuns)

	s.router.Get("/agent/v1/automation/heartbeat/{agent_id}", s.automation.HandleGetHeartbeat)
	s.router.Put("/agent/v1/automation/heartbeat/{agent_id}", s.automation.HandleUpdateHeartbeat)
	s.router.Post("/agent/v1/automation/heartbeat/{agent_id}/wake", s.automation.HandleWakeHeartbeat)
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
