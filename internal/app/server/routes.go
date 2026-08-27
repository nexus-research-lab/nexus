// INPUT: 已装配的 HTTP handlers 与统一 API prefix。
// OUTPUT: 核心、Session、Room、能力和 Web 路由表。
// POS: app 层唯一 HTTP route composition root。
package server

import "strings"

// mountRoutes 按功能域挂载全部 HTTP 路由。
func (s *Server) mountRoutes() {
	s.router.Post(
		s.prefixPath("/internal/runtime/configuration"),
		newRuntimeConfigurationHandler(s.services.Configuration),
	)
	s.router.Post(
		s.prefixPath("/internal/runtime/command"),
		newRuntimeCommandHandler(
			s.services.RuntimeCommand,
			s.services.Automation,
			s.services.GoalCommand,
			s.services.Orchestration,
			s.services.Permission,
			s.services.WorkGraphWorkflow,
		),
	)
	if s.handlers.browser != nil {
		s.router.Get(
			s.prefixPath("/internal/browser/status"),
			s.handlers.browser.HandleStatus,
		)
		s.router.Get(
			s.prefixPath("/internal/browser/ws"),
			s.handlers.browser.HandleWebSocket,
		)
	}
	s.mountCoreRoutes()
	s.mountProviderRoutes()
	s.mountAdminRoutes()
	s.mountProjectRoutes()
	s.mountAgentRoutes()
	s.mountRoomRoutes()
	s.mountCapabilityRoutes()
	s.mountGoalRoutes()
	s.mountExecutionRoutes()
	s.mountPlaceholderRoutes()
	s.mountWebAppRoutes()
}

// mountProjectRoutes 挂载共享项目 ACL 控制面。
func (s *Server) mountProjectRoutes() {
	s.router.Get(s.prefixPath("/projects"), s.handlers.project.HandleListProjects)
	s.router.Post(s.prefixPath("/projects"), s.handlers.project.HandleEnsureProject)
	s.router.Put(
		s.prefixPath("/projects/{project_id}/members/{owner_user_id}"),
		s.handlers.project.HandleGrantProjectMember,
	)
}

// mountAdminRoutes 挂载管理员运营接口。
func (s *Server) mountAdminRoutes() {
	s.router.Get(s.prefixPath("/admin/subscription/overview"), s.handlers.subscription.HandleOverview)
	s.router.Post(s.prefixPath("/admin/subscription/plans"), s.handlers.subscription.HandleUpsertPlan)
	s.router.Put(s.prefixPath("/admin/subscription/plans/{plan_key}"), s.handlers.subscription.HandleUpsertPlan)
	s.router.Put(s.prefixPath("/admin/subscription/users/{user_id}"), s.handlers.subscription.HandleUpdateUserSubscription)
	s.router.Get(s.prefixPath("/admin/subscription/providers"), s.handlers.provider.HandleListSubscriptionProviderConfigs)
	s.router.Post(s.prefixPath("/admin/subscription/providers"), s.handlers.provider.HandleCreateSubscriptionProviderConfig)
	s.router.Post(s.prefixPath("/admin/subscription/providers/{provider}/models/fetch"), s.handlers.provider.HandleFetchSubscriptionProviderModels)
	s.router.Put(s.prefixPath("/admin/subscription/providers/{provider}/models/{model_id}"), s.handlers.provider.HandleUpdateSubscriptionProviderModel)
	s.router.Post(s.prefixPath("/admin/subscription/providers/{provider}/test"), s.handlers.provider.HandleTestSubscriptionProviderConfig)
	s.router.Post(s.prefixPath("/admin/subscription/providers/{provider}/models/{model_id}/test"), s.handlers.provider.HandleTestSubscriptionProviderModel)
	s.router.Put(s.prefixPath("/admin/subscription/providers/{provider}"), s.handlers.provider.HandleUpdateSubscriptionProviderConfig)
	s.router.Delete(s.prefixPath("/admin/subscription/providers/{provider}"), s.handlers.provider.HandleDeleteSubscriptionProviderConfig)
}

// prefixPath 返回带 config.APIPrefix 前缀的完整路径。
func (s *Server) prefixPath(p string) string {
	return s.config.APIPrefix + p
}

// mountCoreRoutes 挂载 HTTP 基础能力路由。
func (s *Server) mountCoreRoutes() {
	s.router.Get(s.prefixPath("/health"), s.handlers.core.HandleHealth)
	s.router.Get(s.prefixPath("/system/version"), s.handlers.core.HandleSystemVersion)
	s.router.Get(s.prefixPath("/auth/status"), s.handlers.auth.HandleAuthStatus)
	s.router.Post(s.prefixPath("/auth/login"), s.handlers.auth.HandleAuthLogin)
	s.router.Post(s.prefixPath("/auth/logout"), s.handlers.auth.HandleAuthLogout)
	s.router.Get(s.prefixPath("/runtime/options"), s.handlers.core.HandleRuntimeOptions)
	s.router.Get(s.prefixPath("/settings/profile"), s.handlers.auth.HandlePersonalProfile)
	s.router.Patch(s.prefixPath("/settings/profile"), s.handlers.auth.HandleUpdatePersonalProfile)
	s.router.Post(s.prefixPath("/settings/profile/password"), s.handlers.auth.HandleChangePassword)
	s.router.Get(s.prefixPath("/settings/preferences"), s.handlers.core.HandleGetPreferences)
	s.router.Patch(s.prefixPath("/settings/preferences"), s.handlers.core.HandleUpdatePreferences)
	s.router.Get(s.prefixPath("/settings/echo"), s.handlers.echo.HandleGetEcho)
	s.router.Put(s.prefixPath("/settings/echo"), s.handlers.echo.HandleUpdateEcho)
	s.router.Get(s.prefixPath("/settings/runtime/nxs/status"), s.handlers.core.HandleNXSRuntimeStatus)
	s.router.Get(s.prefixPath("/chat/ws"), s.handlers.websocket.HandleWebSocket)
}

// mountProviderRoutes 挂载 Provider 配置与模型管理路由。
func (s *Server) mountProviderRoutes() {
	s.router.Get(s.prefixPath("/settings/provider-presets"), s.handlers.provider.HandleListProviderPresets)
	s.router.Get(s.prefixPath("/settings/providers"), s.handlers.provider.HandleListProviderConfigs)
	s.router.Get(s.prefixPath("/settings/providers/options"), s.handlers.provider.HandleListProviderOptions)
	s.router.Post(s.prefixPath("/settings/provider-imports/cc-switch/preview"), s.handlers.provider.HandlePreviewCCSwitch)
	s.router.Post(s.prefixPath("/settings/provider-imports/cc-switch/sync"), s.handlers.provider.HandleSyncCCSwitch)
	s.router.Post(s.prefixPath("/settings/providers"), s.handlers.provider.HandleCreateProviderConfig)
	s.router.Post(s.prefixPath("/settings/providers/{provider}/models/fetch"), s.handlers.provider.HandleFetchProviderModels)
	s.router.Put(s.prefixPath("/settings/providers/{provider}/models/{model_id}"), s.handlers.provider.HandleUpdateProviderModel)
	s.router.Post(s.prefixPath("/settings/providers/{provider}/models/{model_id}/default"), s.handlers.provider.HandleSetDefaultProviderModel)
	s.router.Post(s.prefixPath("/settings/providers/{provider}/test"), s.handlers.provider.HandleTestProviderConfig)
	s.router.Post(s.prefixPath("/settings/providers/{provider}/models/{model_id}/test"), s.handlers.provider.HandleTestProviderModel)
	s.router.Put(s.prefixPath("/settings/providers/{provider}"), s.handlers.provider.HandleUpdateProviderConfig)
	s.router.Delete(s.prefixPath("/settings/providers/{provider}"), s.handlers.provider.HandleDeleteProviderConfig)
}

// mountAgentRoutes 挂载 Agent、Session 与工作区相关路由。
func (s *Server) mountAgentRoutes() {
	s.router.Get(s.prefixPath("/agents"), s.handlers.agent.HandleListAgents)
	s.router.Post(s.prefixPath("/agents"), s.handlers.agent.HandleCreateAgent)
	s.router.Get(s.prefixPath("/agents/profile-template"), s.handlers.agent.HandleGetAgentProfileTemplate)
	s.router.Get(s.prefixPath("/agents/validate/name"), s.handlers.agent.HandleValidateAgentName)
	s.router.Get(s.prefixPath("/agents/{agent_id}"), s.handlers.agent.HandleGetAgent)
	s.router.Patch(s.prefixPath("/agents/{agent_id}"), s.handlers.agent.HandleUpdateAgent)
	s.router.Delete(s.prefixPath("/agents/{agent_id}"), s.handlers.agent.HandleDeleteAgent)
	s.router.Get(s.prefixPath("/agents/{agent_id}/contacts"), s.handlers.agent.HandleListAgentContacts)
	s.router.Post(s.prefixPath("/agents/{agent_id}/contacts"), s.handlers.agent.HandleAddAgentContact)
	s.router.Delete(s.prefixPath("/agents/{agent_id}/contacts/{contact_agent_id}"), s.handlers.agent.HandleDeleteAgentContact)
	s.router.Post(s.prefixPath("/agents/{agent_id}/contacts/{contact_agent_id}/channel"), s.handlers.agent.HandleOpenAgentContactChannel)
	s.router.Post(s.prefixPath("/agents/{agent_id}/communications/messages"), s.handlers.agent.HandleSendAgentCommunicationMessage)
	s.router.Get(s.prefixPath("/agents/{agent_id}/sessions"), s.handlers.agent.HandleListAgentSessions)
	s.router.Get(s.prefixPath("/agents/{agent_id}/private-domain/threads"), s.handlers.room.HandleListAgentPrivateThreads)
	s.router.Get(s.prefixPath("/agents/{agent_id}/private-domain/threads/{thread_id}/events"), s.handlers.room.HandleListAgentPrivateEvents)
	s.router.Get(s.prefixPath("/agents/{agent_id}/workspace/files"), s.handlers.workspace.HandleWorkspaceFiles)
	s.router.Get(s.prefixPath("/agents/{agent_id}/workspace/memory"), s.handlers.workspace.HandleWorkspaceMemory)
	s.router.Delete(s.prefixPath("/agents/{agent_id}/workspace/memory"), s.handlers.workspace.HandleDeleteWorkspaceMemory)
	s.router.Get(s.prefixPath("/agents/{agent_id}/workspace/file"), s.handlers.workspace.HandleWorkspaceFile)
	s.router.Put(s.prefixPath("/agents/{agent_id}/workspace/file"), s.handlers.workspace.HandleUpdateWorkspaceFile)
	s.router.Post(s.prefixPath("/agents/{agent_id}/workspace/upload"), s.handlers.workspace.HandleUploadWorkspaceFile)
	s.router.Post(s.prefixPath("/agents/{agent_id}/workspace/reveal"), s.handlers.workspace.HandleRevealWorkspaceFile)
	s.router.Get(s.prefixPath("/agents/{agent_id}/workspace/download"), s.handlers.workspace.HandleDownloadWorkspaceFile)
	s.router.Post(s.prefixPath("/agents/{agent_id}/workspace/entry"), s.handlers.workspace.HandleCreateWorkspaceEntry)
	s.router.Patch(s.prefixPath("/agents/{agent_id}/workspace/entry"), s.handlers.workspace.HandleRenameWorkspaceEntry)
	s.router.Delete(s.prefixPath("/agents/{agent_id}/workspace/entry"), s.handlers.workspace.HandleDeleteWorkspaceEntry)
	s.router.Get(s.prefixPath("/agents/{agent_id}/skills"), s.handlers.skill.HandleAgentSkills)
	s.router.Post(s.prefixPath("/agents/{agent_id}/skills"), s.handlers.skill.HandleInstallAgentSkill)
	s.router.Patch(s.prefixPath("/agents/{agent_id}/skills/{skill_name}"), s.handlers.skill.HandleSetAgentSkillEnabled)
	s.router.Delete(s.prefixPath("/agents/{agent_id}/skills/{skill_name}"), s.handlers.skill.HandleUninstallAgentSkill)

	s.router.Get(s.prefixPath("/sessions"), s.handlers.agent.HandleListSessions)
	s.router.Post(s.prefixPath("/sessions"), s.handlers.agent.HandleCreateSession)
	s.router.Get(s.prefixPath("/sessions/messages"), s.handlers.agent.HandleSessionMessagesByQuery)
	s.router.Get(s.prefixPath("/sessions/message-detail"), s.handlers.agent.HandleSessionMessageDetailByQuery)
	s.router.Get(s.prefixPath("/sessions/rounds"), s.handlers.agent.HandleSessionRoundsByQuery)
	s.router.Get(s.prefixPath("/sessions/{session_key}/messages"), s.handlers.agent.HandleSessionMessages)
	s.router.Get(s.prefixPath("/sessions/{session_key}/runtime-settings"), s.handlers.agent.HandleSessionRuntimeSettings)
	s.router.Put(s.prefixPath("/sessions/{session_key}/runtime-settings"), s.handlers.agent.HandleUpdateSessionRuntimeSettings)
	s.router.Get(s.prefixPath("/sessions/{session_key}/local-directories"), s.handlers.agent.HandleSessionLocalDirectories)
	s.router.Put(s.prefixPath("/sessions/{session_key}/local-directories"), s.handlers.agent.HandleUpdateSessionLocalDirectories)
	s.router.Get(s.prefixPath("/sessions/{session_key}/tasks"), s.handlers.agent.HandleSessionSubagentTasks)
	s.router.Get(s.prefixPath("/sessions/{session_key}/tasks/{task_id}/messages"), s.handlers.agent.HandleSessionSubagentTaskMessages)
	s.router.Post(s.prefixPath("/sessions/{session_key}/tasks/{task_id}/messages"), s.handlers.agent.HandleSendSessionSubagentTaskMessage)
	s.router.Post(s.prefixPath("/sessions/{session_key}/tasks/{task_id}/stop"), s.handlers.agent.HandleStopSessionSubagentTask)
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
	s.router.Patch(s.prefixPath("/rooms/{room_id}/members/{agent_id}/participation"), s.handlers.room.HandleSetRoomMemberParticipation)
	s.router.Post(s.prefixPath("/rooms/{room_id}/conversations"), s.handlers.room.HandleCreateConversation)
	s.router.Post(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/fork"), s.handlers.room.HandleForkConversation)
	s.router.Get(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/messages"), s.handlers.room.HandleConversationMessages)
	s.router.Get(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/tasks"), s.handlers.room.HandleConversationSubagentTasks)
	s.router.Get(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/tasks/{task_id}/messages"), s.handlers.room.HandleConversationSubagentTaskMessages)
	s.router.Post(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/tasks/{task_id}/messages"), s.handlers.room.HandleSendConversationSubagentTaskMessage)
	s.router.Post(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/tasks/{task_id}/stop"), s.handlers.room.HandleStopConversationSubagentTask)
	s.router.Post(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/attachments/upload"), s.handlers.room.HandleUploadConversationAttachment)
	s.router.Post(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}/close"), s.handlers.room.HandleCloseConversationRuntime)
	s.router.Patch(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}"), s.handlers.room.HandleUpdateConversation)
	s.router.Delete(s.prefixPath("/rooms/{room_id}/conversations/{conversation_id}"), s.handlers.room.HandleDeleteConversation)

	s.router.Post(s.prefixPath("/launcher/query"), s.handlers.launcher.HandleLauncherQuery)
	s.router.Get(s.prefixPath("/launcher/bootstrap"), s.handlers.launcher.HandleLauncherBootstrap)
	s.router.Get(s.prefixPath("/launcher/suggestions"), s.handlers.launcher.HandleLauncherSuggestions)
}

// mountCapabilityRoutes 挂载技能、连接器、通道与自动化能力路由。
func (s *Server) mountCapabilityRoutes() {
	s.router.Get(s.prefixPath("/capability/summary"), s.handlers.capability.HandleCapabilitySummary)
	s.router.Get(s.prefixPath("/capability/loops"), s.handlers.loop.HandleListLoops)
	s.router.Get(s.prefixPath("/capability/loops/{slug}"), s.handlers.loop.HandleGetLoopDetail)

	s.router.Get(s.prefixPath("/skills"), s.handlers.skill.HandleListSkills)
	s.router.Get(s.prefixPath("/skills/{skill_name}/agents"), s.handlers.skill.HandleListSkillAgents)
	s.router.Get(s.prefixPath("/skills/{skill_name}"), s.handlers.skill.HandleGetSkillDetail)
	s.router.Post(s.prefixPath("/skills/import/local"), s.handlers.skill.HandleImportLocalSkill)
	s.router.Post(s.prefixPath("/skills/import/git"), s.handlers.skill.HandleImportGitSkill)
	s.router.Get(s.prefixPath("/skills/search/external"), s.handlers.skill.HandleSearchExternalSkills)
	s.router.Get(s.prefixPath("/skills/external/preview"), s.handlers.skill.HandlePreviewExternalSkill)
	s.router.Post(s.prefixPath("/skills/import/skills-sh"), s.handlers.skill.HandleImportSkillsShSkill)
	s.router.Post(s.prefixPath("/skills/import/source"), s.handlers.skill.HandleImportPrivateSkill)
	s.router.Get(s.prefixPath("/skills/sources"), s.handlers.skill.HandleListExternalSkillSources)
	s.router.Post(s.prefixPath("/skills/sources"), s.handlers.skill.HandleCreateExternalSkillSource)
	s.router.Patch(s.prefixPath("/skills/sources/{source_id}"), s.handlers.skill.HandleUpdateExternalSkillSource)
	s.router.Delete(s.prefixPath("/skills/sources/{source_id}"), s.handlers.skill.HandleDeleteExternalSkillSource)
	s.router.Post(s.prefixPath("/skills/check-updates"), s.handlers.skill.HandleCheckSkillUpdates)
	s.router.Post(s.prefixPath("/skills/update-imported"), s.handlers.skill.HandleUpdateImportedSkills)
	s.router.Post(s.prefixPath("/skills/{skill_name}/update"), s.handlers.skill.HandleUpdateSingleSkill)
	s.router.Delete(s.prefixPath("/skills/{skill_name}"), s.handlers.skill.HandleDeleteSkill)

	s.router.Get(s.prefixPath("/connectors"), s.handlers.connector.HandleListConnectors)
	s.router.Get(s.prefixPath("/connectors/categories"), s.handlers.connector.HandleConnectorCategories)
	s.router.Get(s.prefixPath("/connectors/count"), s.handlers.connector.HandleConnectorCount)
	s.router.Get(s.prefixPath("/custom-mcp-servers"), s.handlers.connector.HandleListCustomMCPServers)
	s.router.Post(s.prefixPath("/custom-mcp-servers"), s.handlers.connector.HandleCreateCustomMCPServer)
	s.router.Put(s.prefixPath("/custom-mcp-servers/{connector_id}"), s.handlers.connector.HandleUpdateCustomMCPServer)
	s.router.Delete(s.prefixPath("/custom-mcp-servers/{connector_id}"), s.handlers.connector.HandleDeleteCustomMCPServer)
	s.router.Get(s.prefixPath("/connectors/{connector_id}"), s.handlers.connector.HandleConnectorDetail)
	s.router.Put(s.prefixPath("/connectors/{connector_id}/oauth-client"), s.handlers.connector.HandleSaveConnectorOAuthClient)
	s.router.Delete(s.prefixPath("/connectors/{connector_id}/oauth-client"), s.handlers.connector.HandleDeleteConnectorOAuthClient)
	s.router.Get(s.prefixPath("/connectors/{connector_id}/auth-url"), s.handlers.connector.HandleConnectorAuthURL)
	s.router.Post(s.prefixPath("/connectors/oauth/callback"), s.handlers.connector.HandleConnectorOAuthCallback)
	s.router.Post(s.prefixPath("/connectors/{connector_id}/device/start"), s.handlers.connector.HandleConnectorDeviceAuthStart)
	s.router.Post(s.prefixPath("/connectors/{connector_id}/device/poll"), s.handlers.connector.HandleConnectorDeviceAuthPoll)
	s.router.Post(s.prefixPath("/connectors/{connector_id}/connect"), s.handlers.connector.HandleConnectConnector)
	s.router.Post(s.prefixPath("/connectors/{connector_id}/disconnect"), s.handlers.connector.HandleDisconnectConnector)
	mountConnectorAuthorizationRoutes(
		s.router,
		s.prefixPath,
		s.services.ConnectorAuthorization,
	)

	s.router.Post(s.prefixPath("/channels/messages"), s.handlers.channel.HandleChannelIngress)
	s.router.Post(s.prefixPath("/channels/internal/messages"), s.handlers.channel.HandleInternalChannelIngress)
	s.router.Post(s.prefixPath("/channels/discord/messages"), s.handlers.channel.HandleDiscordChannelIngress)
	s.router.Post(s.prefixPath("/channels/telegram/messages"), s.handlers.channel.HandleTelegramChannelIngress)
	s.router.Post(s.prefixPath("/channels/dingtalk/messages"), s.handlers.channel.HandleDingTalkChannelIngress)
	s.router.Post(s.prefixPath("/channels/feishu/messages"), s.handlers.channel.HandleFeishuChannelIngress)
	s.router.Post(s.prefixPath("/channels/weixin-personal/messages"), s.handlers.channel.HandleWeixinPersonalChannelIngress)

	s.router.Get(s.prefixPath("/capability/channels"), s.handlers.channel.HandleListChannels)
	s.router.Put(s.prefixPath("/capability/channels/{channel_type}/config"), s.handlers.channel.HandleUpsertChannelConfig)
	s.router.Delete(s.prefixPath("/capability/channels/{channel_type}/config"), s.handlers.channel.HandleDeleteChannelConfig)
	s.router.Delete(s.prefixPath("/capability/channels/{channel_type}/accounts/{account_id}"), s.handlers.channel.HandleDeleteChannelAccount)
	s.router.Post(s.prefixPath("/capability/channels/{channel_type}/login"), s.handlers.channel.HandleStartChannelLogin)
	s.router.Get(s.prefixPath("/capability/channels/{channel_type}/login/{login_id}"), s.handlers.channel.HandleGetChannelLogin)
	s.router.Post(s.prefixPath("/capability/channels/{channel_type}/login/{login_id}/verify-code"), s.handlers.channel.HandleSubmitChannelLoginVerifyCode)
	s.router.Get(s.prefixPath("/capability/pairings"), s.handlers.channel.HandleListPairings)
	s.router.Post(s.prefixPath("/capability/pairings"), s.handlers.channel.HandleCreatePairing)
	s.router.Patch(s.prefixPath("/capability/pairings/{pairing_id}"), s.handlers.channel.HandleUpdatePairing)
	s.router.Delete(s.prefixPath("/capability/pairings/{pairing_id}"), s.handlers.channel.HandleDeletePairing)

	s.router.Get(s.prefixPath("/capability/scheduled/reports/daily"), s.handlers.automation.HandleGetScheduledTaskDailyReport)
	s.router.Get(s.prefixPath("/capability/scheduled/permission-requests"), s.handlers.automation.HandleListPermissionRequests)
	s.router.Post(s.prefixPath("/capability/scheduled/permission-requests/{request_id}/decision"), s.handlers.automation.HandleResolvePermissionRequest)
	s.router.Get(s.prefixPath("/capability/scheduled/tasks"), s.handlers.automation.HandleListScheduledTasks)
	s.router.Post(s.prefixPath("/capability/scheduled/tasks"), s.handlers.automation.HandleCreateScheduledTask)
	s.router.Patch(s.prefixPath("/capability/scheduled/tasks/{job_id}"), s.handlers.automation.HandleUpdateScheduledTask)
	s.router.Delete(s.prefixPath("/capability/scheduled/tasks/{job_id}"), s.handlers.automation.HandleDeleteScheduledTask)
	s.router.Post(s.prefixPath("/capability/scheduled/tasks/{job_id}/run"), s.handlers.automation.HandleRunScheduledTask)
	s.router.Post(s.prefixPath("/capability/scheduled/tasks/{job_id}/recover"), s.handlers.automation.HandleRecoverScheduledTask)
	s.router.Get(s.prefixPath("/capability/scheduled/tasks/{job_id}/status"), s.handlers.automation.HandleGetScheduledTaskStatus)
	s.router.Patch(s.prefixPath("/capability/scheduled/tasks/{job_id}/status"), s.handlers.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get(s.prefixPath("/capability/scheduled/tasks/{job_id}/runs"), s.handlers.automation.HandleListScheduledTaskRuns)
	s.router.Get(s.prefixPath("/capability/scheduled/tasks/{job_id}/events"), s.handlers.automation.HandleListScheduledTaskEvents)
	s.router.Post(s.prefixPath("/capability/scheduled/tasks/{job_id}/runs/{run_id}/delivery/retry"), s.handlers.automation.HandleRetryScheduledTaskRunDelivery)
	s.router.Post(s.prefixPath("/capability/scheduled/tasks/{job_id}/runs/{run_id}/permission/resume"), s.handlers.automation.HandleResumePermissionRun)

	s.router.Get(s.prefixPath("/scheduled/reports/daily"), s.handlers.automation.HandleGetScheduledTaskDailyReport)
	s.router.Get(s.prefixPath("/scheduled/permission-requests"), s.handlers.automation.HandleListPermissionRequests)
	s.router.Post(s.prefixPath("/scheduled/permission-requests/{request_id}/decision"), s.handlers.automation.HandleResolvePermissionRequest)
	s.router.Get(s.prefixPath("/scheduled/tasks"), s.handlers.automation.HandleListScheduledTasks)
	s.router.Post(s.prefixPath("/scheduled/tasks"), s.handlers.automation.HandleCreateScheduledTask)
	s.router.Patch(s.prefixPath("/scheduled/tasks/{job_id}"), s.handlers.automation.HandleUpdateScheduledTask)
	s.router.Delete(s.prefixPath("/scheduled/tasks/{job_id}"), s.handlers.automation.HandleDeleteScheduledTask)
	s.router.Post(s.prefixPath("/scheduled/tasks/{job_id}/run"), s.handlers.automation.HandleRunScheduledTask)
	s.router.Post(s.prefixPath("/scheduled/tasks/{job_id}/recover"), s.handlers.automation.HandleRecoverScheduledTask)
	s.router.Get(s.prefixPath("/scheduled/tasks/{job_id}/status"), s.handlers.automation.HandleGetScheduledTaskStatus)
	s.router.Patch(s.prefixPath("/scheduled/tasks/{job_id}/status"), s.handlers.automation.HandleUpdateScheduledTaskStatus)
	s.router.Get(s.prefixPath("/scheduled/tasks/{job_id}/runs"), s.handlers.automation.HandleListScheduledTaskRuns)
	s.router.Get(s.prefixPath("/scheduled/tasks/{job_id}/events"), s.handlers.automation.HandleListScheduledTaskEvents)
	s.router.Post(s.prefixPath("/scheduled/tasks/{job_id}/runs/{run_id}/delivery/retry"), s.handlers.automation.HandleRetryScheduledTaskRunDelivery)
	s.router.Post(s.prefixPath("/scheduled/tasks/{job_id}/runs/{run_id}/permission/resume"), s.handlers.automation.HandleResumePermissionRun)

}

// mountGoalRoutes 挂载 Goal 相关路由。
func (s *Server) mountGoalRoutes() {
	s.router.Get(s.prefixPath("/goals/current"), s.handlers.goal.HandleGetCurrentGoal)
	s.router.Post(s.prefixPath("/goals"), s.handlers.goal.HandleCreateGoal)
	s.router.Get(s.prefixPath("/goals/{goal_id}/execution-binding"), s.handlers.goal.HandleGetGoalExecutionBinding)
	s.router.Get(s.prefixPath("/goals/{goal_id}/usage"), s.handlers.goal.HandleGetGoalUsage)
	s.router.Patch(s.prefixPath("/goals/{goal_id}"), s.handlers.goal.HandleUpdateGoal)
	s.router.Post(s.prefixPath("/goals/{goal_id}/pause"), s.handlers.goal.HandlePauseGoal)
	s.router.Post(s.prefixPath("/goals/{goal_id}/resume"), s.handlers.goal.HandleResumeGoal)
	s.router.Post(s.prefixPath("/goals/{goal_id}/clear"), s.handlers.goal.HandleClearGoal)
	s.router.Get(s.prefixPath("/goals/{goal_id}/events"), s.handlers.goal.HandleGoalEvents)
	s.router.Post(s.prefixPath("/app-server/thread/goal/set"), s.handlers.goal.HandleThreadGoalSet)
	s.router.Post(s.prefixPath("/app-server/thread/goal/get"), s.handlers.goal.HandleThreadGoalGet)
	s.router.Post(s.prefixPath("/app-server/thread/goal/clear"), s.handlers.goal.HandleThreadGoalClear)
}

// mountExecutionRoutes 挂载 WorkGraph 历史读取、草图预览与已保存草图目录。
func (s *Server) mountExecutionRoutes() {
	s.router.Get(
		s.prefixPath("/executions/latest"),
		s.handlers.execution.HandleGetLatestExecution,
	)
	s.router.Get(
		s.prefixPath("/executions/history"),
		s.handlers.execution.HandleListExecutionHistory,
	)
	s.router.Get(
		s.prefixPath("/executions/{execution_id}"),
		s.handlers.execution.HandleGetExecution,
	)
	s.router.Get(
		s.prefixPath("/workgraph/workflows"),
		s.handlers.execution.HandleListWorkGraphWorkflows,
	)
	s.router.Get(
		s.prefixPath("/workgraph/workflows/slash-name-availability"),
		s.handlers.execution.HandleCheckWorkGraphWorkflowSlashName,
	)
	s.router.Post(
		s.prefixPath("/workgraph/workflows/{workflow_id}/preview"),
		s.handlers.execution.HandlePreviewSavedWorkGraphWorkflow,
	)
	s.router.Post(
		s.prefixPath("/workgraph/previews"),
		s.handlers.execution.HandlePreviewWorkGraphWorkflow,
	)
	s.router.Post(
		s.prefixPath("/workgraph/previews/{preview_id}/save"),
		s.handlers.execution.HandleScheduleWorkGraphWorkflowSave,
	)
	s.router.Post(
		s.prefixPath("/workgraph/previews/{preview_id}/editor"),
		s.handlers.execution.HandleStartWorkGraphWorkflowEditor,
	)
	s.router.Get(
		s.prefixPath("/workgraph/editors/{editor_id}"),
		s.handlers.execution.HandleGetWorkGraphWorkflowEditor,
	)
	s.router.Post(
		s.prefixPath("/workgraph/editors/{editor_id}/apply"),
		s.handlers.execution.HandleApplyWorkGraphWorkflowEditor,
	)
	s.router.Post(
		s.prefixPath("/workgraph/editors/{editor_id}/versions/select"),
		s.handlers.execution.HandleSelectWorkGraphWorkflowEditorVersion,
	)
	s.router.Delete(
		s.prefixPath("/workgraph/editors/{editor_id}"),
		s.handlers.execution.HandleCloseWorkGraphWorkflowEditor,
	)
	s.router.Delete(
		s.prefixPath("/workgraph/workflows/{workflow_id}"),
		s.handlers.execution.HandleDeleteWorkGraphWorkflow,
	)
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
