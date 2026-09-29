// INPUT: Relay Team Room 快照、当前 Control 用户与消息发送动作。
// OUTPUT: 复用 Room Header、FOLLOW/READING 阅读轨道、本人消息和 Composer，保留独立读取重试。
// POS: Relay 真人消息与完整 Agent 回复到 Nexus Room UI 的窄适配层；不推断运行态或流式输出。

import { Fragment, useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { CircleAlert } from "lucide-react";
import { Navigate, useSearchParams } from "react-router-dom";
import { captureAuthOwnerScopeGeneration, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { UiButton } from "@/shared/ui/button/button";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import { listControlAgentDirectoryApi, type ControlMemberDirectoryEntry, type ControlAgentDirectoryEntry } from "@/lib/api/account/control-api";

import { MessageUserSection } from "@/features/conversation/shared/message/item/view/user/message-user-section";
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import { MessageActivityStatus, ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME } from "@/features/conversation/shared/message/item/view/message-activity-status";
import type { AgentMention } from "@/types/conversation/message/entity";
import type { AgentMentionDirectory } from "@/features/conversation/shared/message/agent-mention-chip";
import {
  ConversationPanelLayout,
  ConversationPanelViewport,
  ConversationPanelViewportArea,
} from "@/features/conversation/shared/conversation-panel-layout";
import {
  CONVERSATION_COMPOSER_LANE_CLASS_NAME,
  CONVERSATION_CONTENT_LANE_CLASS_NAME,
} from "@/features/conversation/shared/conversation-panel-styles";
import { formatMessageTime } from "@/features/conversation/shared/message/message-time";
import { useTeamInvitations } from "@/features/team/use-team-invitations";
import { TeamInvitationList } from "@/features/team/team-invitation-list";
import { useTeamRoom } from "@/features/team/use-team-room";
import { useTeamMembers } from "@/features/team/use-team-members";
import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { TeamRoomMembersDialog } from "@/features/team/team-room-members-dialog";
import { TeamExecutionObserver, TeamExecutionThread, type TeamExecutionControls } from "@/features/team/team-execution-thread";
import { cancelTeamDelivery } from "@/lib/api/conversation/team-api";
import { ComposerInteractionSurface } from "@/features/conversation/shared/composer/components/interaction/composer-interaction-surface";
import { getRoomAgentRoundEntry, isAgentRoundActive } from "@/features/conversation/room/group/round/round-agent-model";
import type { RoomAgentExecutionState } from "@/types/agent/agent-conversation";
import { TeamExecutionSurface } from "@/features/team/team-execution-surface";
import { buildRoomHeaderTabs, type RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import { buildRoomAgentSessionKey } from "@/lib/conversation/session-key";
import { useAgentStore } from "@/store/agent";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import { useTeamRefresh } from "@/features/team/use-team-refresh";
import { getTeamNode, prepareTeamRoom, type TeamNodeJob, type TeamRoomBinding } from "@/lib/api/conversation/team-node-api";
import { ThreadActionButton } from "@/features/conversation/room/group/thread/round-card/thread-action-button";
import { RoomAgentExecutionActions, RoomAgentStopButton } from "@/features/conversation/room/group/thread/round-card/group-agent-execution-shell";
import type { TeamMessage } from "@/lib/api/conversation/team-api";
import { getTeamCommands } from "@/lib/api/conversation/team-api";
import { uploadTeamFile, saveTeamFile } from "@/lib/api/conversation/team-files-api";
import { inspectComposerAttachment, ComposerAttachmentRejectedError } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import { MessageUserAttachments } from "@/features/conversation/shared/message/item/view/user/message-user-attachments";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type { CommandCatalogData } from "@/types/generated/protocol";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY, clampHomeSidePanelWidthPercent, HOME_SIDE_PANEL_DEFAULT_WIDTH_PERCENT } from "@/lib/layout/home-layout";
import { useMouseDrag } from "@/shared/lib/react/use-mouse-drag";
import { useRoomSidePanelResize } from "@/features/conversation/room/surface/layout/use-room-side-panel-resize";
import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";
import { hasOrganizationAccess, useAuth } from "@/shared/auth/auth-context";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { GroupMemberAvatarStack } from "@/features/conversation/room/group/header/group-member-avatar-stack";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import "@/features/conversation/room/surface/room-conversation-header-edge.css";

export function TeamPage() {
  const [searchParams] = useSearchParams();
  const { status } = useAuth();
  const generation = useSyncExternalStore(subscribeAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration);
  const roomId = searchParams.get("room_id");
  return <TeamPageContent key={JSON.stringify([generation, status?.organization_id, status?.control_user_id ?? status?.user_id, roomId])} roomId={roomId} />;
}

function TeamPageContent({ roomId }: { roomId: string | null }) {
  const { status } = useAuth();
  const canUseRelay = hasOrganizationAccess(status);
  const [commandCatalog, setCommandCatalog] = useState<CommandCatalogData>({commands: [], status: "unavailable"});
  const transfers = useRef(new AbortController());
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [commandFailed, setCommandFailed] = useState(false);
  const [commandRetry, setCommandRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    transfers.current = controller;
    return () => controller.abort();
  }, []);
  useEffect(() => {
    if (!canUseRelay) return;
    const controller = new AbortController();
    void getTeamCommands(controller.signal).then((catalog) => {
      if (!controller.signal.aborted) {setCommandCatalog(catalog); setCommandFailed(false);}
    }).catch(() => {if (!controller.signal.aborted) setCommandFailed(true);});
    return () => controller.abort();
  }, [canUseRelay, commandRetry]);
  const [searchParams] = useSearchParams();
  const { t } = useI18n();
  const room = useTeamRoom(roomId);
  const [membersOpen, setMembersOpen] = useState(false);
  const [storedJobs, setJobs] = useState<TeamNodeJob[]>([]);
  const [executionStates, setExecutionStates] = useState<Record<string, RoomAgentExecutionState[]>>({});
  const [executionControls, setExecutionControls] = useState<Record<string, TeamExecutionControls>>({});
  const onControls = useCallback((id: string, controls: TeamExecutionControls | null) => {
    setExecutionControls((current) => {
      const next = {...current};
      if (controls) next[id] = controls;
      else delete next[id];
      return next;
    });
  }, []);
  const jobs = storedJobs.map((job) => {
    const states = executionStates[job.conversation_id ?? ""]?.filter((state) => state.round_id === job.round_id) ?? [];
    const entry = getRoomAgentRoundEntry([], job.local_agent_id ?? "", [], undefined, states);
    if (!entry || (job.state !== "running" && job.state !== "ready")) return job;
    const terminal = {done: "draining", cancelled: "cancelled", error: "failed"} as const;
    return {...job, state: terminal[entry.status as keyof typeof terminal] ?? job.state};
  });
  const [jobsFailed, setJobsFailed] = useState(false);
  const [bindings, setBindings] = useState<TeamRoomBinding[]>([]);
  const [bindingsFailed, setBindingsFailed] = useState(false);
  const [activeTab, setActiveTab] = useState<RoomSurfaceTabKey>("chat");
  const [selectedLocalAgent, setSelectedLocalAgent] = useState("");
  const [workspaceFile, setWorkspaceFile] = useState<{agentId: string; path: string | null} | null>(null);
  const runtimeKind = useDefaultAgentRuntimeKind();
  const [selectedThreadID, setSelectedThreadID] = useState<string | null>(searchParams.get("thread"));
  const setThread = (job: TeamNodeJob | null) => { setSelectedThreadID(job?.id ?? null); setActiveTab("chat"); };
  useEffect(() => { setSelectedThreadID(searchParams.get("thread")); }, [searchParams]);
  const thread = room.room ? jobs.find((job) => job.id === selectedThreadID) : undefined;
  const splitRef = useRef<HTMLDivElement>(null);
  const [panelWidth, setPanelWidth] = useState(HOME_SIDE_PANEL_DEFAULT_WIDTH_PERCENT);
  const updatePanelWidth = useCallback((value: number) => setPanelWidth(clampHomeSidePanelWidthPercent(value)), []);
  const resize = useRoomSidePanelResize(thread ? "thread" : "auxiliary", panelWidth, updatePanelWidth);
  const { startDragging } = useMouseDrag(useCallback((event: MouseEvent) => {
    const bounds = splitRef.current?.getBoundingClientRect();
    if (bounds && bounds.width > 0) updatePanelWidth((bounds.right - event.clientX) / bounds.width * 100);
  }, [updatePanelWidth]));
  const refreshJobs = useTeamRefresh(canUseRelay && room.room && !room.room.room.direct_user_id && roomId ? JSON.stringify([roomId, room.messages.map((message) => message.id), selectedThreadID]) : null, async (signal) => {
    try {
      if (!roomId) return;
      const messageIds = [...new Set(room.messages.flatMap((message) => [message.id, ...(message.delivery_id ? [message.delivery_id] : [])]))];
      const result: TeamNodeJob[] = [];
      for (let offset = 0; offset < Math.max(messageIds.length, 1); offset += 100) {
        const node = await getTeamNode(signal, {roomId, messageIds: messageIds.slice(offset, offset + 100), jobId: selectedThreadID});
        result.push(...node.jobs ?? []);
      }
      if (!signal.aborted) {
        setJobs([...new Map(result.map((job) => [job.id, job])).values()].filter((job) => job.source_room_id === roomId && job.room_id && job.conversation_id && job.local_agent_id && job.round_id));
        setJobsFailed(false);
      }
    } catch { if (!signal.aborted) setJobsFailed(true); }
  }, false);
  const onExecutionChange = useCallback((conversationId: string, states: RoomAgentExecutionState[]) => {
    setExecutionStates((current) => ({...current, [conversationId]: states}));
    refreshJobs();
  }, [refreshJobs]);
	const [agentDirectory, setAgentDirectory] = useState<ControlAgentDirectoryEntry[]>([]);
	const memberDirectory = useTeamMembers(canUseRelay);
	const localAgents = useHomeDirectory().agents;
  const ownedAgents = useAgentStore((state) => state.agents);
  const loadOwnedAgents = useAgentStore((state) => state.load_agents_from_server);
  const refreshBindings = useTeamRefresh(canUseRelay && room.room ? JSON.stringify([roomId, room.room.room.membership_version]) : null, async (signal) => {
    if (!roomId) return;
    try {
      const result = await prepareTeamRoom(roomId, signal);
      if (signal.aborted) return;
      setBindings(result); setBindingsFailed(false);
      if (result.some((binding) => !ownedAgents.some((agent) => agent.agent_id === binding.local_agent_id))) await loadOwnedAgents();
    } catch { if (!signal.aborted) { setBindings([]); setBindingsFailed(true); } }
  });
	useEffect(() => {
		if (!canUseRelay) return;
        let cancelled = false;
		void listControlAgentDirectoryApi().then((agents) => { if (!cancelled) setAgentDirectory(agents); }).catch(() => { if (!cancelled) setAgentDirectory([]); });
        return () => { cancelled = true; };
	}, [canUseRelay, room.room?.room.membership_version]);
  const scroll = useFollowScroll({
    messageCount: room.messages.length,
    historyPrependToken: room.historyPrependToken,
    sessionKey: room.room?.conversation.id ?? null,
  });
  const latestMessageSeq = room.messages.at(-1)?.message_seq ?? 0;
  const markRead = room.markRead;
  const isFollowingLatest = scroll.isFollowingLatest;
  useEffect(() => {
    const acknowledgeVisible = () => {
      if (document.visibilityState === "visible" && document.hasFocus() && isFollowingLatest()) {
        void markRead(latestMessageSeq);
      }
    };
    acknowledgeVisible();
    window.addEventListener("focus", acknowledgeVisible);
    document.addEventListener("visibilitychange", acknowledgeVisible);
    return () => {
      window.removeEventListener("focus", acknowledgeVisible);
      document.removeEventListener("visibilitychange", acknowledgeVisible);
    };
  }, [isFollowingLatest, latestMessageSeq, markRead, room.room?.last_read_message_seq, scroll.showScrollToBottom]);
  const isCompact = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
  const directUserId = room.room?.room.direct_user_id;
  const peer = memberDirectory.find((member) => member.user_id === directUserId);
  const title = directUserId ? peer?.display_name || peer?.username || directUserId : room.room?.room.name ?? t("team.shared_room");
  const headerMembers = (room.room?.members ?? []).filter((member) => member.state === "active").map((member) => {
    const agent = member.member_type === "agent" ? agentDirectory.find((entry) => entry.agent_id === member.member_id) : undefined;
    const person = member.member_type === "user" ? memberDirectory.find((entry) => entry.user_id === member.member_id) : undefined;
    const isSelf = member.member_type === "user" && member.member_id === (status?.control_user_id ?? status?.user_id);
    return {
      agent_id: `${member.member_type}:${member.member_id}`,
      name: agent?.name || person?.display_name || person?.username || (isSelf ? status?.display_name || status?.username || t("team.you") : member.member_id),
      avatar: agent?.avatar || person?.avatar || (isSelf ? status?.avatar : undefined),
    };
  });
  const errorMessage = room.error ? t(TEAM_ERROR_KEYS[room.error]) : null;
	const activeAgentIDs = new Set(room.room?.members
		.filter((member) => member.member_type === "agent" && member.state === "active" && !member.agent_paused)
		.map((member) => member.member_id) ?? []);
  const sendMessage = async (content: string, _policy: unknown, attachments: MessageAttachment[] = [], targets: string[] = []) => {
    if (!room.room || room.isSending || room.hasUnconfirmedSend) throw new Error("在线消息当前不可发送");
    const files = attachments.map((file) => {
      if (file.scope !== "relayRoom" || file.room_id !== room.room!.room.id || !file.relay_file || file.size === undefined) throw new Error("附件不属于当前在线群");
      return {id: file.relay_file.id, name: file.file_name, size: file.size, sha256: file.relay_file.sha256};
    });
    if (files.length > 8 || files.reduce((sum, file) => sum + file.size, 0) > 32 * 1024 * 1024) throw new Error("每条消息最多 8 个附件、合计 32 MiB");
    if (content.trim().startsWith("/") && !targets.length) {
      setSubmitError(t("team.command_requires_agent"));
      throw new Error("Slash 命令需要显式 @ Agent");
    }
    setSubmitError(null);
    const ok = files.length ? await room.send(content, targets, files) : await (targets.length ? room.send(content, targets) : room.send(content));
    if (!ok) throw new Error("在线消息未确认");
  };
  const prepareAttachments = async (files: File[]): Promise<MessageAttachment[]> => {
    const targetRoomId = room.room?.room.id;
    if (!targetRoomId || files.length > 8 || files.reduce((sum, file) => sum + file.size, 0) > 32 * 1024 * 1024) throw new Error("附件超过消息上限");
    const inputs = files.map((file) => {
      const inspection = inspectComposerAttachment(file);
      if (!inspection.accepted) throw new ComposerAttachmentRejectedError(inspection);
      return { file, kind: inspection.kind };
    });
    const result: MessageAttachment[] = [];
    for (const { file, kind } of inputs) {
      const uploaded = await uploadTeamFile(targetRoomId, file, transfers.current.signal);
      if (!uploaded.sha256) throw new Error("共享附件缺少摘要");
      result.push({file_name: uploaded.name, workspace_path: "", room_id: targetRoomId, scope: "relayRoom", relay_file: {id: uploaded.id, sha256: uploaded.sha256}, kind, size: uploaded.size, mime_type: file.type});
    }
    return result;
  };
  const composerScope = JSON.stringify([status?.organization_id, status?.control_user_id, roomId]);
  // 会话绑定独立于任务历史；暂停投递不剥夺本人查看和配置成员的能力。
  const memberAgentIDs = new Set(room.room?.members.filter((member) => member.member_type === "agent" && member.state === "active").map((member) => member.member_id));
  const executionBindings = room.room ? bindings.filter((binding) => memberAgentIDs.has(binding.agent_id)) : [];
  const executionAgents = ownedAgents.filter((agent) => executionBindings.some((binding) => binding.local_agent_id === agent.agent_id));
  const executionAgentId = executionAgents.some((agent) => agent.agent_id === selectedLocalAgent) ? selectedLocalAgent : executionAgents[0]?.agent_id ?? "";
  const executionBinding = executionBindings.find((binding) => binding.local_agent_id === executionAgentId);
  // 只有当前群的本人绑定可响应；请求始终回到其原生会话，不能广播审批或跨群停止。
  const interactions = Object.entries(executionControls).flatMap(([id, controls]) =>
    (controls.pending_permissions ?? []).filter((permission) => executionBindings.some((binding) =>
      binding.conversation_id === id && binding.local_agent_id === permission.agent_id))
      .filter((permission) => {
        const state = controls.room_agent_execution_states?.find((entry) => entry.round_id === permission.round_id
          && entry.agent_id === permission.agent_id && entry.agent_round_id === permission.agent_round_id);
        return !state || isAgentRoundActive(state.status);
      })
      .map((permission) => ({permission, controls})));
  const stopAction = (job: TeamNodeJob) => {
    if (!executionBindings.some((binding) => binding.conversation_id === job.conversation_id && binding.local_agent_id === job.local_agent_id)) return undefined;
    const controls = executionControls[job.conversation_id ?? ""];
    const entry = getRoomAgentRoundEntry([], job.local_agent_id ?? "", [], undefined,
      controls?.room_agent_execution_states?.filter((state) => state.round_id === job.round_id) ?? []);
    const agentRoundId = entry?.agent_round_id;
    if (!agentRoundId || !entry || !isAgentRoundActive(entry.status) || !controls) return undefined;
    return <RoomAgentStopButton isStopping={controls.stopping_agent_round_ids?.includes(agentRoundId)}
      onClick={() => controls.stop_generation(agentRoundId)} />;
  };

  if (!canUseRelay) {
    return <Navigate replace to={APP_ROUTE_PATHS.home} />;
  }

  return (
    <>
      {[...new Map(executionBindings.map((binding) => [binding.conversation_id, binding])).values()].map((binding) => (
        <TeamExecutionObserver key={binding.conversation_id} binding={binding} onChange={onExecutionChange} onControls={onControls} />
      ))}
      <WorkspacePageFrame contentPaddingClassName="p-0">
        <WorkspaceSurfaceScaffold
          bodyClassName="relative"
          header={(
            <div className="nexus-room-conversation-header-edge" data-room-conversation-header-edge="true">
              <WorkspaceSurfaceHeader
                activeTab={activeTab}
                tabs={room.room && !directUserId ? buildRoomHeaderTabs(t) : []}
                compactTabsLabel={t("room.panels")}
                onChangeTab={(tab) => { setActiveTab(activeTab === tab ? "chat" : tab); setSelectedThreadID(null); }}
                leading={(
                  directUserId ? <UiAgentAvatar avatar={peer?.avatar} name={title} size="md" /> : <UiRoomAvatar
                    avatar={directUserId ? peer?.avatar : room.room?.room.avatar}
                    members={[]}
                    roomId={room.room?.room.id}
                    size="md"
                    title={title}
                  />
                )}
                leadingVariant="identity"
                title={room.room ? undefined : title}
                tabsLeading={room.room ? (
                  <WorkspaceConversationTabs
                    activeConversationId={room.room.conversation.id}
                    tabs={[{ id: room.room.conversation.id, title, canClose: false }]}
                    onSelectConversation={() => {}}
                    onCloseConversation={() => {}}
                  />
                ) : null}
                navigationTrailing={room.room && !directUserId ? (
                  <>
                    <GroupMemberAvatarStack members={headerMembers} onClick={() => setMembersOpen(true)} />
                  </>
                ) : null}
              />
            </div>
          )}
        >
        <div ref={splitRef} className="flex h-full min-h-0 min-w-0 flex-1">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <ConversationPanelLayout>
          <ConversationPanelViewportArea>
            <ConversationPanelViewport
              floatingDockOccupied={scroll.showScrollToBottom}
              isMobileLayout={isCompact}
              viewport={{ ...scroll, isHistoryLoading: room.isHistoryLoading, ariaLabel: t("team.shared_room") }}
            >
              <div ref={scroll.feedRef} className="min-h-full">
              {room.hasEarlier && (
                <div className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex justify-center py-2`}>
                  <UiButton variant="ghost" disabled={room.isHistoryLoading} onClick={async () => {
                    if (!await room.loadEarlier(scroll.prepareHistoryPrependRestore)) scroll.cancelHistoryPrependRestore();
                  }}>
                    {t(room.historyError ? "team.history_retry" : "team.load_earlier")}
                  </UiButton>
                </div>
              )}
              <TeamMessageFeed
                roomId={room.room?.room.id ?? ""}
                stopAction={stopAction}
                deliveries={room.room?.deliveries ?? []}
                jobs={jobs} selectedThreadID={thread?.id} onOpenThread={(job) => setThread(thread?.id === job.id ? null : job)}
                agentDirectory={agentDirectory}
                memberDirectory={memberDirectory} directUserId={directUserId}
                currentUserId={status?.control_user_id ?? status?.user_id ?? null}
                currentUserAvatar={status?.avatar}
                isCompact={isCompact}
                isLoading={room.isLoading}
                loadFailed={room.error === "load"}
                messages={room.messages}
              />
              <div ref={scroll.bottomAnchorRef} />
              </div>
            </ConversationPanelViewport>
            <div
              className="pointer-events-none absolute inset-x-0 bottom-2 grid justify-items-center"
              data-conversation-activity-dock
            >
              <ScrollToLatestButton visible={scroll.showScrollToBottom} onClick={() => scroll.scrollToBottom()} />
            </div>
          </ConversationPanelViewportArea>

          <div className="relative z-10 shrink-0" data-conversation-bottom-area>
            {bindingsFailed || jobsFailed || errorMessage || submitError || commandFailed ? (
              <div className={`${CONVERSATION_COMPOSER_LANE_CLASS_NAME} grid gap-2 px-6 pb-2`}>
                {submitError ? <UiInlineNotice tone="danger" message={submitError} /> : null}
                {commandFailed ? <UiInlineNotice tone="warning" message={t("team.commands_failed")} action={{label: t("state.retry"), onClick: () => setCommandRetry((value) => value + 1)}} /> : null}
                {bindingsFailed ? (
                  <UiInlineNotice role="alert" tone="danger" icon={<CircleAlert />}
                    message={t("team.binding_error")}
                    action={{ label: t("state.retry"), onClick: refreshBindings }} />
                ) : null}
                {jobsFailed ? (
                  <UiInlineNotice role="alert" tone="danger" icon={<CircleAlert />}
                    message={t("team.node_jobs_error")}
                    action={{ label: t("team.node_refresh"), onClick: refreshJobs }} />
                ) : null}
                {errorMessage ? (
                  <UiInlineNotice role="alert" tone={room.error === "sync" ? "warning" : "danger"} icon={<CircleAlert />}
                    message={errorMessage}
                    action={room.error === "load" ? {
                      label: t("state.retry"), pending: room.isLoading,
                      onClick: () => { void room.retryLoad(); },
                    } : undefined} />
                ) : null}
              </div>
            ) : null}
            <fieldset disabled={!room.room || room.isSending} className="min-w-0 border-0 p-0 m-0">
            <ComposerPanel
              compact={isCompact} commandCatalog={commandCatalog}
              contextUsage={null} showActionMenu defaultPlaceholder={t(directUserId ? "team.direct_placeholder" : "team.message_placeholder")}
              draftScopeKey={composerScope} historyScopeKey={composerScope}
              sessionSettings={executionAgents.length ? {
                initialTargetId: executionAgentId, runtimeKind,
                targets: executionAgents.map((agent) => ({
                  agentId: agent.agent_id, name: agent.name, avatar: agent.avatar,
                  defaultModel: agent.options.model, defaultProvider: agent.options.provider,
                  defaultConnectorIds: agent.options.connector_ids, defaultPermissionMode: "default",
                  sessionKey: buildRoomAgentSessionKey(executionBindings.find((binding) => binding.local_agent_id === agent.agent_id)!.conversation_id, agent.agent_id),
                })),
              } : undefined}
              isLoading={!room.room || room.isSending} runtimePhase={null} runtimeKind={runtimeKind}
              defaultDeliveryPolicy="queue" queueWhenSessionBusy={false}
              roomMembers={agentDirectory.filter((agent) => activeAgentIDs.has(agent.agent_id))}
              onSendMessage={sendMessage} onEnqueueMessage={sendMessage}
              inputQueueItems={[]} onDeleteQueuedMessage={() => {}} onGuideQueuedMessage={() => {}} onReorderQueueMessages={() => {}}
              onPrepareAttachments={prepareAttachments} goalScopeLabel="" tourAnchor=""
              interactionIdentity={interactions[0]?.permission.request_id ?? null}
              interactionSurface={interactions.length ? <ComposerInteractionSurface
                permissions={interactions.map(({permission}) => permission)}
                agentNameMap={Object.fromEntries(executionAgents.map((agent) => [agent.agent_id, agent.name]))}
                agentAvatarMap={Object.fromEntries(executionAgents.map((agent) => [agent.agent_id, agent.avatar ?? null]))}
                onResponse={(payload) => interactions.find(({permission}) => permission.request_id === payload.request_id)?.controls.send_permission_response(payload) ?? false}
              /> : room.hasUnconfirmedSend ? <div className="p-4">
                <p className="whitespace-pre-wrap break-words">{room.pendingText}</p>
                {room.pendingAttachmentNames?.length ? <p>{room.pendingAttachmentNames.join(" · ")}</p> : null}
                <UiButton disabled={room.isSending || !room.room} onClick={() => { void room.send(room.pendingText ?? ""); }} size="sm" variant="surface">{t("state.retry")}</UiButton>
              </div> : undefined}
            />
            </fieldset>
          </div>
        </ConversationPanelLayout>
        </div>
        {!isCompact && (thread || (activeTab !== "chat" && room.room)) ? <PanelResizeHandle ariaLabel={t(thread ? "room.resize_thread_panel" : "room.resize_auxiliary_panel")} control={resize.control} controls={resize.panelId} onResizeStart={startDragging} variant="gutter" /> : null}
        {room.room && activeTab !== "chat" ? <aside id={resize.panelId} ref={resize.panelRef} style={isCompact ? undefined : {width: `${panelWidth}%`, ...resize.widthStyle}} className={isCompact ? "contents" : "nexus-room-surface-side-panel relative h-full min-h-0 shrink-0 overflow-hidden"}>
          <TeamExecutionSurface roomId={room.room.room.id} tab={activeTab} agents={executionAgents} binding={executionBinding} selectedAgentId={executionAgentId} compact={isCompact}
            activeWorkspacePath={workspaceFile?.agentId === executionAgentId ? workspaceFile.path : null}
            onOpenWorkspaceFile={(path) => { setWorkspaceFile({agentId: executionAgentId, path}); setActiveTab("workspace"); }}
            onSelectAgent={(id) => { setSelectedLocalAgent(id); setWorkspaceFile(null); }} onClose={() => setActiveTab("chat")} />
        </aside> : null}
        {thread ? <aside id={resize.panelId} ref={resize.panelRef} style={isCompact ? undefined : {width: `${panelWidth}%`, ...resize.widthStyle}} className={isCompact ? "contents" : "nexus-room-surface-side-panel relative h-full min-h-0 shrink-0 overflow-hidden"}>
          <TeamExecutionThread showInteraction={isCompact} key={thread.id} job={thread} name={agentDirectory.find((agent) => agent.agent_id === thread.agent_id)?.name ?? thread.agent_id}
            hasFinalOutput={Boolean(thread.delivery_id) && room.messages.some((message) => message.delivery_id === thread.delivery_id && message.author_type === "agent" && message.output_kind === "final")}
            avatar={agentDirectory.find((agent) => agent.agent_id === thread.agent_id)?.avatar || undefined}
            compact={isCompact} onClose={() => setThread(null)} onOpenWorkspaceFile={(path, workspaceAgentId) => {
              if (workspaceAgentId !== undefined && workspaceAgentId !== thread.local_agent_id) return;
              setSelectedLocalAgent(thread.local_agent_id!); setWorkspaceFile({agentId: thread.local_agent_id!, path}); setSelectedThreadID(null); setActiveTab("workspace");
            }} />
        </aside> : null}
        </div>
        </WorkspaceSurfaceScaffold>
      </WorkspacePageFrame>
		<TeamRoomMembersDialog
			agents={localAgents}
		currentUserId={status?.control_user_id ?? status?.user_id ?? ""}
        directory={memberDirectory}
        onClose={() => setMembersOpen(false)}
        onChanged={room.updateDetails}
        onDeparted={() => { void room.retryLoad(); }}
        open={membersOpen}
        roomId={room.room?.room.id ?? null}
      />
    </>
  );
}

function DeliveryCancelButton({roomId, deliveryId}: {roomId: string; deliveryId: string}) {
  const {t} = useI18n();
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  return <><UiButton variant="ghost" disabled={busy} onClick={async () => {
    setBusy(true); setFailed(false);
    try { await cancelTeamDelivery(roomId, deliveryId); }
    catch { setFailed(true); }
    finally { setBusy(false); window.dispatchEvent(new Event("focus")); }
  }}>{t("team.cancel_waiting")}</UiButton>{failed ? <span role="alert">{t("team.cancel_failed")}</span> : null}</>;
}

function TeamMessageFeed({
  roomId,
  stopAction,
  deliveries,
  jobs, selectedThreadID, onOpenThread,
  agentDirectory,
  memberDirectory, directUserId,
  currentUserId,
  currentUserAvatar,
  isCompact,
  isLoading,
  loadFailed,
  messages,
}: {
  roomId: string;
  stopAction: (job: TeamNodeJob) => import("react").ReactNode;
  deliveries: import("@/lib/api/conversation/team-api").TeamDeliveryStatus[];
  jobs: TeamNodeJob[]; selectedThreadID?: string; onOpenThread: (job: TeamNodeJob) => void;
  agentDirectory: ControlAgentDirectoryEntry[];
  memberDirectory: ControlMemberDirectoryEntry[]; directUserId?: string;
  currentUserId: string | null;
  currentUserAvatar?: string | null;
  isCompact: boolean;
  isLoading: boolean;
  loadFailed: boolean;
  messages: TeamMessage[];
}) {
  const { t } = useI18n();
  const onAccepted = useCallback(() => { window.dispatchEvent(new Event("focus")); }, []);
  const invitations = useTeamInvitations(onAccepted, Boolean(directUserId));
  const pending = invitations.invitations.filter((item) => item.invited_by_user_id === directUserId);
  if (loadFailed && !isLoading && messages.length === 0) return null;
  if (messages.length === 0 && pending.length === 0) {
    if (!isLoading) return null;
    return (
      <div role="status" className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex h-full min-h-64 items-center justify-center ${getUiTypographyClassName({ role: "supporting", tone: "muted" })}`}>
        {t("team.loading")}
      </div>
    );
  }
  const agentsByID = new Map(agentDirectory.map((agent) => [agent.agent_id, agent]));
  const messagesByID = new Map(messages.map((message) => [message.id, message]));
  const sourcesByDelivery = new Map(deliveries.map((delivery) => [delivery.id, messagesByID.get(delivery.message_id)]));
  for (const [id, source] of jobs.filter((job) => job.delivery_id && job.source_message_id).map((job) => [
    job.delivery_id, messagesByID.get(job.source_message_id!),
  ] as const)) { if (id) sourcesByDelivery.set(id, source); }
  const replyTarget = (source?: TeamMessage) => {
    if (!source) return undefined;
    const agent = source.author_type === "agent" ? agentsByID.get(source.author_agent_id ?? "") : undefined;
    const person = source.author_type === "user" ? memberDirectory.find((member) => member.user_id === source.author_user_id) : undefined;
    return {
      name: agent?.name || person?.display_name || person?.username || source.author_display_name || source.author_username || "?",
      avatar: agent?.avatar || person?.avatar || (source.author_type === "user" && source.author_user_id === currentUserId ? currentUserAvatar : undefined),
      message: source.content.blocks.map((block) => block.text).join("\n"),
    };
  };
  const mentionDirectory: AgentMentionDirectory = {
    names: Object.fromEntries(agentDirectory.map((agent) => [agent.agent_id, agent.name])),
    avatars: Object.fromEntries(agentDirectory.map((agent) => [agent.agent_id, agent.avatar ?? null])),
  };
  return (
    <ol aria-busy={isLoading || undefined} className={`nexus-chat-feed ${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex flex-col`}>
      {messages.map((message) => message.content.blocks.some((block) => block.type === "room_invitation") ? (
        <li key={message.id} className="px-3 py-2 text-sm text-(--text-muted)">
          {message.content.blocks.filter((block) => block.type === "room_invitation").map((block, index) => {
            const pendingInvitation = pending.find((item) => item.room.id === block.room_id && new Date(item.created_at).getTime() === new Date(block.invited_at ?? "").getTime());
            return pendingInvitation ? <TeamInvitationList key={index} {...invitations} invitations={[pendingInvitation]} recoveryRooms={[]}
              onRefresh={invitations.refresh} onResolve={invitations.resolve} onRecover={invitations.recover} /> : (
            <div key={index} className="rounded-xl border divider-subtle p-4">
              <span>{t("team.pending_invitations")} · {block.text}</span>
              <time className="ml-3" dateTime={message.created_at}>{formatMessageTime(new Date(message.created_at).getTime())}</time>
              <p className="mt-2">{t(block.invitee_user_id !== currentUserId ? "team.invitation_sent" : invitations.loading ? "team.loading" : invitations.failed ? "team.invitation_failed" : "team.invitation_closed")}</p>
            </div>
          ); })}
        </li>
      ) : (
        <Fragment key={message.id}><TeamMessageItem
          roomId={roomId}
          agent={message.author_type === "agent" ? agentsByID.get(message.author_agent_id ?? "") : undefined}
          person={memberDirectory.find((member) => member.user_id === message.author_user_id)}
          currentUserId={currentUserId}
          isCompact={isCompact}
          key={message.id}
          message={message}
          replyTarget={replyTarget(message.author_type === "agent" && message.delivery_id ? sourcesByDelivery.get(message.delivery_id) : undefined)}
          mentionDirectory={mentionDirectory}
          threadAction={jobs.filter((job) => message.author_type === "agent" && message.delivery_id && job.delivery_id === message.delivery_id).map((job) => <Fragment key={job.id}>{stopAction(job)}<ThreadActionButton active={selectedThreadID === job.id} agentName={agentsByID.get(job.agent_id)?.name ?? job.agent_id} onClick={() => onOpenThread(job)} /></Fragment>)}
        />
        {jobs.filter((job) => job.source_message_id === message.id && !messages.some((reply) => reply.author_type === "agent" && reply.delivery_id === job.delivery_id)).map((job) => <li key={job.id}>
          <MessageItem animateEntry={false} compact={isCompact} assistantContentMode="room_result"
            assistantReplyTarget={replyTarget(message)}
            currentAgentName={agentsByID.get(job.agent_id)?.name ?? job.agent_id} currentAgentAvatar={agentsByID.get(job.agent_id)?.avatar}
            roundId={job.round_id!} messages={[]} isLastRound
            isLoading={job.state === "running" || job.state === "ready"}
            activityState={job.state === "ready" ? "sending" : job.state === "running" ? "thinking" : undefined}
					assistantEmptyState={<span className={getUiTypographyClassName({role: "supporting", tone: "muted"})}>{t(job.failure_code === "artifact_delivery_failed" ? "team.artifact_delivery_failed" : job.failure_code === "execution_failed" ? "team.delivery_failed" : `team.node_job_${job.state}`)}</span>}
            canRespondToPermissions={false}
            assistantHeaderAction={<RoomAgentExecutionActions>{stopAction(job)}<ThreadActionButton active={selectedThreadID === job.id} agentName={agentsByID.get(job.agent_id)?.name ?? job.agent_id} onClick={() => onOpenThread(job)} /></RoomAgentExecutionActions>} />
        </li>)}
        {deliveries.filter((delivery) => delivery.message_id === message.id
          && !jobs.some((job) => job.delivery_id === delivery.id)
          && delivery.state !== "completed"
          && !messages.some((reply) => reply.delivery_id === delivery.id && reply.output_kind === "final"))
          .map((delivery) => <li key={delivery.id}>
            <MessageItem animateEntry={false} compact={isCompact} assistantContentMode="room_result"
              assistantReplyTarget={replyTarget(message)}
              currentAgentName={agentsByID.get(delivery.agent_id)?.name ?? delivery.agent_id}
              currentAgentAvatar={agentsByID.get(delivery.agent_id)?.avatar}
              roundId={delivery.id} messages={[]} isLastRound isLoading={false} canRespondToPermissions={false}
              assistantHeaderAction={delivery.state === "pending" && message.author_user_id === currentUserId ? <DeliveryCancelButton roomId={roomId} deliveryId={delivery.id} /> : undefined}
              assistantEmptyState={<div role="status">
                {delivery.state === "pending" || delivery.state === "leased" ? (
                  <MessageActivityStatus className={ROOM_RESULT_ACTIVITY_ALIGNMENT_CLASS_NAME} stableSlot state={delivery.state === "pending" ? "sending" : delivery.execution_state === "waiting_input" ? "waiting_input" : "replying"}
                    label={t(delivery.state === "leased" && delivery.execution_state ? `team.delivery_${delivery.execution_state}` : `team.delivery_${delivery.state}`)} />
                ) : (
                  <span className={getUiTypographyClassName({role: "supporting", tone: "muted"})}>
                    {t(DELIVERY_FAILURE_KEYS[delivery.failure_code ?? ""] ?? (delivery.state === "completed" ? "team.node_job_completed" : `team.delivery_${delivery.state}`))}
                  </span>
                )}
              </div>} />
          </li>)}
        </Fragment>
      ))}
      {directUserId ? <li><TeamInvitationList {...invitations} invitations={pending.filter((item) => !messages.some((message) => message.content.blocks.some((block) => block.room_id === item.room.id && new Date(block.invited_at ?? "").getTime() === new Date(item.created_at).getTime())))} recoveryRooms={[]}
        onRefresh={invitations.refresh} onResolve={invitations.resolve} onRecover={invitations.recover} /></li> : null}
    </ol>
  );
}

function TeamMessageItem({
  roomId,
  replyTarget,
  threadAction,
  mentionDirectory,
  person,
  agent,
  currentUserId,
  isCompact,
  message,
}: {
  roomId: string;
  replyTarget?: {name: string; avatar?: string | null; message?: string};
  threadAction?: import("react").ReactNode[];
  mentionDirectory: AgentMentionDirectory;
  agent?: ControlAgentDirectoryEntry;
  person?: ControlMemberDirectoryEntry;
  currentUserId: string | null;
  isCompact: boolean;
  message: TeamMessage;
}) {
  const {t} = useI18n();
  const [downloadFailed, setDownloadFailed] = useState(false);
  const transfer = useRef<AbortController | null>(null);
  useEffect(() => () => transfer.current?.abort(), []);
  const attachments: MessageAttachment[] = (message.content.attachments ?? []).map((file) => ({
    file_name: file.name, workspace_path: "", scope: "relayRoom", kind: "file", size: file.size,
    room_id: roomId, relay_file: {id: file.id, sha256: file.sha256},
  }));
  const download = (attachment: MessageAttachment) => {
    const file = message.content.attachments?.find((item) => item.id === attachment.relay_file?.id);
    if (!file) return;
    transfer.current?.abort();
    const controller = new AbortController(); transfer.current = controller;
    setDownloadFailed(false);
    void saveTeamFile(roomId, file, controller.signal).catch(() => {if (!controller.signal.aborted) setDownloadFailed(true);});
  };
  const content = message.content.blocks.map((block) => block.text).join("\n\n");
  const agentMentions = projectTeamMentions(content, message.mentions ?? [], mentionDirectory);
  // 在线回复只适配消息事实，身份头、正文、复制与 Thread 排布沿用 Room 展示面。
  if (message.author_type === "agent") {
    const files = attachments.length ? <div className="flex"><MessageUserAttachments attachments={attachments} onOpenAttachment={download} /></div> : null;
    return <li><MessageItem
      assistantEmptyState={files}
      assistantReplyTarget={replyTarget}
      animateEntry={false}
      compact={isCompact}
      assistantContentMode="room_result"
      currentAgentName={agent?.name || message.author_display_name || message.author_username || "?"}
      currentAgentAvatar={agent?.avatar}
      assistantHeaderAction={threadAction?.length ? <RoomAgentExecutionActions>{threadAction}</RoomAgentExecutionActions> : undefined}
      agentMentionDirectory={mentionDirectory}
      roundId={message.id}
      isLoading={false}
      canRespondToPermissions={false}
      messages={[{
        message_id: message.id, session_key: `team:${message.conversation_id}`,
        conversation_id: message.conversation_id, agent_id: message.author_agent_id ?? "",
        round_id: message.id, role: "assistant", timestamp: new Date(message.created_at).getTime(),
        content: [{type: "text", text: content}], is_complete: true,
        agent_mentions: agentMentions,
        model: message.content.execution?.model,
        result_summary: message.content.execution?.result_summary
          ? {...message.content.execution.result_summary, subtype: "success", is_error: false}
          : undefined,
      }]}
    />{content.trim() ? files : null}{downloadFailed ? <p role="alert">{t("team.files_error")}</p> : null}</li>;
  }
  const author = person?.display_name || person?.username || message.author_display_name || message.author_username || "?";
    return (
      <li>
        <MessageUserSection
          alignment={currentUserId && message.author_user_id === currentUserId ? "right" : "left"}
          onOpenAttachment={download}
          compact={isCompact}
          author={message.author_user_id === currentUserId ? undefined : {name: author, avatar: person?.avatar}}
          agentMentionDirectory={mentionDirectory}
          message={{
            attachments,
            agent_id: "",
            client_message_id: message.client_message_id,
            content,
            agent_mentions: agentMentions,
            conversation_id: message.conversation_id,
            message_id: message.id,
            role: "user",
            room_id: null,
            round_id: message.id,
            session_key: `team:${message.conversation_id}`,
            timestamp: new Date(message.created_at).getTime(),
          }}
        />
        {downloadFailed ? <p role="alert">{t("team.files_error")}</p> : null}
      </li>
    );
}

// Relay 的目标身份只用于展示映射；正文出现同名文本不会新增唤醒目标。
function projectTeamMentions(content: string, mentions: NonNullable<TeamMessage["mentions"]>, directory: AgentMentionDirectory): AgentMention[] {
  return mentions.flatMap(({member_id}) => {
    const name = directory.names?.[member_id];
    if (!name) return [];
    const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return Array.from(content.matchAll(new RegExp(`(?:^|\\s)(@${escaped})(?=$|\\s|[，。！？、,.!?;:：；])`, "giu")), (match) => {
      const label = match[1];
      const start = match.index + match[0].length - label.length;
      const startRune = Array.from(content.slice(0, start)).length;
      return {agent_id: member_id, label, content_block_index: 0, start_rune: startRune, end_rune: startRune + Array.from(label).length};
    });
  });
}

const TEAM_ERROR_KEYS = {
  load: "team.error_load",
  send: "team.error_send",
  sync: "team.error_sync",
} as const;

const DELIVERY_FAILURE_KEYS: Record<string, "team.handoff_limit_exceeded" | "team.artifact_delivery_failed" | "team.queue_expired" | "team.request_cancelled" | "team.delivery_expired" | "team.delivery_failed"> = {
  handoff_limit_exceeded: "team.handoff_limit_exceeded",
  artifact_delivery_failed: "team.artifact_delivery_failed",
  queue_expired: "team.queue_expired",
  request_cancelled: "team.request_cancelled",
	lease_expired: "team.delivery_expired",
	execution_failed: "team.delivery_failed",
};
