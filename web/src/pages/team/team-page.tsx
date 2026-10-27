// INPUT: Relay Team Room 快照、当前 Control 用户与消息发送动作。
// OUTPUT: 复用 Room Header、右栏容器、FOLLOW/READING 阅读轨道、共享消息卡片和 Composer，保留独立读取重试。
// POS: Relay 真人消息与完整 Agent 回复到 Nexus Room UI 的窄适配层；不推断运行态或流式输出。

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react";
import { CircleAlert } from "lucide-react";
import { Navigate, useSearchParams } from "react-router-dom";
import { captureAuthOwnerScopeGeneration, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { UiButton } from "@/shared/ui/button/button";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import { listControlAgentDirectoryApi, type ControlAgentDirectoryEntry } from "@/lib/api/account/control-api";
import {
  ConversationPanelLayout,
  ConversationPanelViewport,
  ConversationPanelViewportArea,
} from "@/features/conversation/shared/conversation-panel-layout";
import {
  CONVERSATION_COMPOSER_LANE_CLASS_NAME,
  CONVERSATION_CONTENT_LANE_CLASS_NAME,
} from "@/features/conversation/shared/conversation-panel-styles";
import { useTeamRoom } from "@/features/team/use-team-room";
import { useTeamMembers } from "@/features/team/use-team-members";
import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { TeamRoomMembersDialog } from "@/features/team/team-room-members-dialog";
import { TeamExecutionObserver, TeamExecutionThread, type TeamExecutionControls } from "@/features/team/team-execution-thread";
import { ComposerInteractionSurface } from "@/features/conversation/shared/composer/components/interaction/composer-interaction-surface";
import { getRoomAgentRoundEntry, isAgentRoundActive } from "@/features/conversation/room/group/round/round-agent-model";
import type { RoomAgentExecutionState } from "@/types/agent/agent-conversation";
import { TeamExecutionSurface } from "@/features/team/team-execution-surface";
import { TeamMessageFeed } from "@/features/team/team-message-feed";
import { buildRoomHeaderTabs, type RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import { RoomSidePanelContainer } from "@/features/conversation/room/surface/layout/room-side-panel-container";
import { RoomConversationHeaderEdge } from "@/features/conversation/room/surface/layout/room-surface-header";
import { buildRoomAgentSessionKey } from "@/lib/conversation/session-key";
import { useAgentStore } from "@/store/agent";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import { useTeamRefresh } from "@/features/team/use-team-refresh";
import { getTeamNode, prepareTeamRoom, type TeamNodeJob, type TeamRoomBinding } from "@/lib/api/conversation/team-node-api";
import { RoomAgentStopButton } from "@/features/conversation/room/group/thread/round-card/group-agent-execution-shell";
import { getTeamCommands } from "@/lib/api/conversation/team-api";
import { uploadTeamFile } from "@/lib/api/conversation/team-files-api";
import { inspectComposerAttachment, ComposerAttachmentRejectedError } from "@/features/conversation/shared/composer/attachments/composer-attachments";
import type { MessageAttachment } from "@/types/conversation/message/attachment";
import type { CommandCatalogData } from "@/types/generated/protocol";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY, clampHomeSidePanelWidthPercent, HOME_SIDE_PANEL_DEFAULT_WIDTH_PERCENT } from "@/lib/layout/home-layout";
import { useMouseDrag } from "@/shared/lib/react/use-mouse-drag";
import { hasTeamAccess, useAuth } from "@/shared/auth/auth-context";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { UiAgentAvatar, UiRoomAvatar } from "@/shared/ui/display/avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { GroupMemberAvatarStack } from "@/features/conversation/room/group/header/group-member-avatar-stack";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

export function TeamPage() {
  const [searchParams] = useSearchParams();
  const { status } = useAuth();
  const generation = useSyncExternalStore(subscribeAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration);
  const roomId = searchParams.get("room_id");
  return <TeamPageContent key={JSON.stringify([generation, status?.organization_id, status?.control_user_id ?? status?.user_id, roomId])} roomId={roomId} />;
}

function TeamPageContent({ roomId }: { roomId: string | null }) {
  const { status } = useAuth();
  const canUseRelay = hasTeamAccess(status);
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
            <RoomConversationHeaderEdge>
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
                  <GroupMemberAvatarStack members={headerMembers} onClick={() => setMembersOpen(true)} />
                ) : null}
              />
            </RoomConversationHeaderEdge>
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
        {room.room && activeTab !== "chat" ? (
          <RoomSidePanelContainer
            compact={isCompact}
            kind="auxiliary"
            onSidePanelWidthChange={updatePanelWidth}
            onStartSidePanelResize={startDragging}
            sidePanelWidthPercent={panelWidth}
          >
            <TeamExecutionSurface roomId={room.room.room.id} tab={activeTab} agents={executionAgents} binding={executionBinding} selectedAgentId={executionAgentId} compact={isCompact}
              activeWorkspacePath={workspaceFile?.agentId === executionAgentId ? workspaceFile.path : null}
              onOpenWorkspaceFile={(path) => { setWorkspaceFile({agentId: executionAgentId, path}); setActiveTab("workspace"); }}
              onSelectAgent={(id) => { setSelectedLocalAgent(id); setWorkspaceFile(null); }} onClose={() => setActiveTab("chat")} />
          </RoomSidePanelContainer>
        ) : null}
        {thread ? (
          <RoomSidePanelContainer
            compact={isCompact}
            kind="thread"
            onSidePanelWidthChange={updatePanelWidth}
            onStartSidePanelResize={startDragging}
            sidePanelWidthPercent={panelWidth}
          >
            <TeamExecutionThread showInteraction={isCompact} key={thread.id} job={thread} name={agentDirectory.find((agent) => agent.agent_id === thread.agent_id)?.name ?? thread.agent_id}
              hasFinalOutput={Boolean(thread.delivery_id) && room.messages.some((message) => message.delivery_id === thread.delivery_id && message.author_type === "agent" && message.output_kind === "final")}
              avatar={agentDirectory.find((agent) => agent.agent_id === thread.agent_id)?.avatar || undefined}
              compact={isCompact} onClose={() => setThread(null)} onOpenWorkspaceFile={(path, workspaceAgentId) => {
                if (workspaceAgentId !== undefined && workspaceAgentId !== thread.local_agent_id) return;
                setSelectedLocalAgent(thread.local_agent_id!); setWorkspaceFile({agentId: thread.local_agent_id!, path}); setSelectedThreadID(null); setActiveTab("workspace");
              }} />
          </RoomSidePanelContainer>
        ) : null}
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

const TEAM_ERROR_KEYS = {
  load: "team.error_load",
  send: "team.error_send",
  rate_limited: "team.error_rate_limited",
  sync: "team.error_sync",
} as const;
