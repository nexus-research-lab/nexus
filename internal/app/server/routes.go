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

// prefixPath 返回带 config.APIPrefix 前缀的完整路径。
func (s *Server) prefixPath(p string) string {
	return s.config.APIPrefix + p
}

// mountCoreRoutes 挂载 HTTP 基础能力路由。
func (s *Server) mountCoreRoutes() {
	s.router.Get(s.prefixPath("/health"), s.handlers.core.HandleHealth)
	s.router.Get(s.prefixPath("/auth/status"), s.handlers.auth.HandleAuthStatus)
	s.router.Post(s.prefixPath("/auth/login"), s.handlers.auth.HandleAuthLogin)
	s.router.Post(s.prefixPath("/auth/logout"), s.handlers.auth.HandleAuthLogout)
	s.router.Get(s.prefixPath("/runtime/options"), s.handlers.core.HandleRuntimeOptions)
	s.router.Get(s.prefixPath("/settings/profile"), s.handlers.auth.HandlePersonalProfile)
	s.router.Patch(s.prefixPath("/settings/profile"), s.handlers.auth.HandleUpdatePersonalProfile)
	s.router.Post(s.prefixPath("/settings/profile/password"), s.handlers.auth.HandleChangePassword)
	s.router.Get(s.prefixPath("/settings/preferences"), s.handlers.core.HandleGetPreferences)
	s.router.Patch(s.prefixPath("/settings/preferences"), s.handlers.core.HandleUpdatePreferences)
	s.router.Get(s.prefixPath("/settings/providers"), s.handlers.core.HandleListProviderConfigs)
	s.router.Get(s.prefixPath("/settings/providers/options"), s.handlers.core.HandleListProviderOptions)
	s.router.Post(s.prefixPath("/settings/providers"), s.handlers.core.HandleCreateProviderConfig)
	s.router.Put(s.prefixPath("/settings/providers/{provider}"), s.handlers.core.HandleUpdateProviderConfig)
	s.router.Delete(s.prefixPath("/settings/providers/{provider}"), s.handlers.core.HandleDeleteProviderConfig)
	s.router.Get(s.prefixPath("/chat/ws"), s.handlers.websocket.HandleWebSocket)
}

// mountAgentRoutes 挂载 Agent、Session 与工作区相关路由。
func (s *Server) mountAgentRoutes() {
	s.router.Get(s.prefixPath("/agents"), s.handlers.agent.HandleListAgents)
	s.router.Get(s.prefixPath("/agents/runtime/statuses"), s.handlers.agent.HandleAgentRuntimeStatuses)
	s.router.Post(s.prefixPath("/agents"), s.handlers.agent.HandleCreateAgent)
	s.router.Get(s.prefixPath("/agents/validate/name"), s.handlers.agent.HandleValidateAgentName)
	s.router.Get(s.prefixPath("/agents/{agent_id}"), s.handlers.agent.HandleGetAgent)
	s.router.Patch(s.prefixPath("/agents/{agent_id}"), s.handlers.agent.HandleUpdateAgent)
	s.router.Delete(s.prefixPath("/agents/{agent_id}"), s.handlers.agent.HandleDeleteAgent)
	s.router.Get(s.prefixPath("/agents/{agent_id}/sessions"), s.handlers.agent.HandleListAgentSessions)
	s.router.Get(s.prefixPath("/agents/{agent_id}/workspace/files"), s.handlers.workspace.HandleWorkspaceFiles)
	s.router.Get(s.prefixPath("/agents/{agent_id}/workspace/file"), s.handlers.workspace.HandleWorkspaceFile)
	s.router.Put(s.prefixPath("/agents/{agent_id}/workspace/file"), s.handlers.workspace.HandleUpdateWorkspaceFile)
	s.router.Post(s.prefixPath("/agents/{agent_id}/workspace/upload"), s.handlers.workspace.HandleUploadWorkspaceFile)
	s.router.Get(s.prefixPath("/agents/{agent_id}/workspace/download"), s.handlers.workspace.HandleDownloadWorkspaceFile)
	s.router.Post(s.prefixPath("/agents/{agent_id}/workspace/entry"), s.handlers.workspace.HandleCreateWorkspaceEntry)
	s.router.Patch(s.prefixPath("/agents/{agent_id}/workspace/entry"), s.handlers.workspace.HandleRenameWorkspaceEntry)
	s.router.Delete(s.prefixPath("/agents/{agent_id}/workspace/entry"), s.handlers.workspace.HandleDeleteWorkspaceEntry)
	s.router.Get(s.prefixPath("/agents/{agent_id}/skills"), s.handlers.skill.HandleAgentSkills)
	s.router.Post(s.prefixPath("/agents/{agent_id}/skills"), s.handlers.skill.HandleInstallAgentSkill)
	s.router.Delete(s.prefixPath("/agents/{agent_id}/skills/{skill_name}"), s.handlers.skill.HandleUninstallAgentSkill)

	s.router.Get(s.prefixPath("/sessions"), s.handlers.agent.HandleListSessions)
	s.router.Post(s.prefixPath("/sessions"), s.handlers.agent.HandleCreateSession)
	s.router.Patch(s.prefixPath("/sessions/{session_key}"), s.handlers.agent.HandleUpdateSession)
	s.router.Delete(s.prefixPath("/sessions/{session_key}"), s.handlers.agent.HandleDeleteSession)
}

// mountRoomRoutes 挂载 Room 与 Launcher 相关路由。
func (s *Server) mountRoomRoutes() {
	s.router.Get(s.prefixPath("/rooms/dm/{agent_id}"), s.handlers.room.HandleEnsureDirectRoom)
	s.router.Get(s.prefixPath("/rooms"), s.handlers.room.HandleListRooms)
	s.router.Post(s.prefixPath("/rooms"), s.handlers.room.HandleCreateRoom)
	s.router.Get(s.prefixPath("/rooms/{room_id}"), s.handlers.room.HandleGetRoom)
	s.router.Patch(s.prefixPath("/rooms/{room_id}"), s.handlers.room.HandleUpdateRoom)
	s.router.Delete(s.prefixPath("/rooms/{room_id}"), s.handlers.room.HandleDeleteRoom)
	s.router.Get(s.prefixPath("/rooms/{room_id}/contexts"), s.handlers.room.HandleGetRoomContexts)
	s.router.Post(s.prefixPath("/rooms/{room_id}/members"), s.handlers.room.HandleAddRoomMember)
	s.router.Delete(s.prefixPath("/rooms/{room_id}/members/{agent_id}"), s.handlers.room.HandleRemoveRoomMember)
	s.router.Post(s.prefixPath("/rooms/{room_id}/conversations"), s.handlers.room.HandleCreateConversation)
	s.router.Get(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/messages"), s.handlers.room.HandleConversationMessages)
	s.router.Patch(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}"), s.handlers.room.HandleUpdateConversation)
	s.router.Delete(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}"), s.handlers.room.HandleDeleteConversation)

	s.router.Post(s.prefixPath("/launcher/query"), s.handlers.launcher.HandleLauncherQuery)
	s.router.Get(s.prefixPath("/launcher/bootstrap"), s.handlers.launcher.HandleLauncherBootstrap)
	s.router.Get(s.prefixPath("/launcher/suggestions"), s.handlers.launcher.HandleLauncherSuggestions)
}

// mountCapabilityRoutes 挂载技能、连接器、通道与自动化能力路由。
func (s *Server) mountCapabilityRoutes() {
	s.router.Get(s.prefixPath("/capability/summary"), s.handlers.capability.HandleCapabilitySummary)

	s.router.Get(s.prefixPath("/skills"), s.handlers.skill.HandleListSkills)
	s.router.Get(s.prefixPath("/skills/{skill_name}"), s.handlers.skill.HandleGetSkillDetail)
	s.router.Post(s.prefixPath("/skills/import/local"), s.handlers.skill.HandleImportLocalSkill)
	s.router.Post(s.prefixPath("/skills/import/git"), s.handlers.skill.HandleImportGitSkill)
	s.router.Get(s.prefixPath("/skills/search/external"), s.handlers.skill.HandleSearchExternalSkills)
	s.router.Get(s.prefixPath("/skills/external/preview"), s.handlers.skill.HandlePreviewExternalSkill)
	s.router.Post(s.prefixPath("/skills/import/skills-sh"), s.handlers.skill.HandleImportSkillsShSkill)
	s.router.Post(s.prefixPath("/skills/update-imported"), s.handlers.skill.HandleUpdateImportedSkills)
	s.router.Post(s.prefixPath("/skills/{skill_name}/update"), s.handlers.skill.HandleUpdateSingleSkill)
	s.router.Delete(s.prefixPath("/skills/{skill_name}"), s.handlers.skill.HandleDeleteSkill)

	s.router.Get(s.prefixPath("/connectors"), s.handlers.connector.HandleListConnectors)
	s.router.Get(s.prefixPath("/connectors/categories"), s.handlers.connector.HandleConnectorCategories)
	s.router.Get(s.prefixPath("/connectors/count"), s.handlers.connector.HandleConnectorCount)
	s.router.Get(s.prefixPath("/connectors/{connector_id}"), s.handlers.connector.HandleConnectorDetail)
	s.router.Get(s.prefixPath("/connectors/{connector_id}/oauth-client"), s.handlers.connector.HandleGetConnectorOAuthClient)
	s.router.Put(s.prefixPath("/connectors/{connector_id}/oauth-client"), s.handlers.connector.HandleUpsertConnectorOAuthClient)
	s.router.Delete(s.prefixPath("/connectors/{connector_id}/oauth-client"), s.handlers.connector.HandleDeleteConnectorOAuthClient)
	s.router.Get(s.prefixPath("/connectors/{connector_id}/auth-url"), s.handlers.connector.HandleConnectorAuthURL)
	s.router.Post(s.prefixPath("/connectors/oauth/callback"), s.handlers.connector.HandleConnectorOAuthCallback)
	s.router.Post(s.prefixPath("/connectors/{connector_id}/connect"), s.handlers.connector.HandleConnectConnector)
	s.router.Post(s.prefixPath("/connectors/{connector_id}/disconnect"), s.handlers.connector.HandleDisconnectConnector)

	s.router.Post(s.prefixPath("/channels/messages"), s.handlers.channel.HandleChannelIngress)
	s.router.Post(s.prefixPath("/channels/internal/messages"), s.handlers.channel.HandleInternalChannelIngress)
	s.router.Post(s.prefixPath("/channels/discord/messages"), s.handlers.channel.HandleDiscordChannelIngress)
	s.router.Post(s.prefixPath("/channels/telegram/messages"), s.handlers.channel.HandleTelegramChannelIngress)

	s.router.Get(s.prefixPath("/capability/scheduled/tasks"), s.handlers.automation.HandleListScheduledTasks)
	s.router.Post(s.prefixPath("/capability/scheduled/tasks"), s.handlers.automation.HandleCreateScheduledTask)
	s.router.Patch(s.prefixPath("/capability/scheduled/tasks/{job_id}"), s.handlers.automation.HandleUpdateScheduledTask)
	s.router.Delete(s.prefixPath("/capability/scheduled/tasks/{job_id}"), s.handlers.automation.HandleDeleteScheduledTask)
	s.router.Post(s.prefixPath("/capability/scheduled/tasks/{job_id}/run"), s.handlers.automation.HandleRunScheduledTask)
	s.router.Patch(s.prefixPath("/capability/scheduled/tasks/{job_id}/status"), s.handlers.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get(s.prefixPath("/capability/scheduled/tasks/{job_id}/runs"), s.handlers.automation.HandleListScheduledTaskRuns)

	s.router.Get(s.prefixPath("/scheduled/tasks"), s.handlers.automation.HandleListScheduledTasks)
	s.router.Post(s.prefixPath("/scheduled/tasks"), s.handlers.automation.HandleCreateScheduledTask)
	s.router.Patch(s.prefixPath("/scheduled/tasks/{job_id}"), s.handlers.automation.HandleUpdateScheduledTask)
	s.router.Delete(s.prefixPath("/scheduled/tasks/{job_id}"), s.handlers.automation.HandleDeleteScheduledTask)
	s.router.Post(s.prefixPath("/scheduled/tasks/{job_id}/run"), s.handlers.automation.HandleRunScheduledTask)
	s.router.Patch(s.prefixPath("/scheduled/tasks/{job_id}/status"), s.handlers.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get(s.prefixPath("/scheduled/tasks/{job_id}/runs"), s.handlers.automation.HandleListScheduledTaskRuns)

	s.router.Get(s.prefixPath("/automation/heartbeat/{agent_id}"), s.handlers.automation.HandleGetHeartbeat)
	s.router.Put(s.prefixPath("/automation/heartbeat/{agent_id}"), s.handlers.automation.HandleUpdateHeartbeat)
	s.router.Post(s.prefixPath("/automation/heartbeat/{agent_id}/wake"), s.handlers.automation.HandleWakeHeartbeat)
}

// mountPlaceholderRoutes 挂载保留占位路由。
func (s *Server) mountPlaceholderRoutes() {
	for _, group := range []string{} {
		s.mountPlaceholderGroup(group)
	}
}

func (s *Server) mountPlaceholderGroup(group string) {
	base := strings.TrimPrefix(group, "/")
	s.router.HandleFunc(s.prefixPath("/"+base), s.api.HandleNotImplemented(group))
	s.router.HandleFunc(s.prefixPath("/"+base+"/*"), s.api.HandleNotImplemented(group))
}
