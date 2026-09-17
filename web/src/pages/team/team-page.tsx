// INPUT: Relay Team Room 快照、当前 Control 用户与消息发送动作。
// OUTPUT: 复用 Room Header、FOLLOW/READING 阅读轨道、本人消息和 Composer，保留独立读取重试。
// POS: Relay 真人消息与完整 Agent 回复到 Nexus Room UI 的窄适配层；不推断运行态或流式输出。

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { CircleAlert, MonitorCheck } from "lucide-react";
import { Navigate, useSearchParams } from "react-router-dom";
import { captureAuthOwnerScopeGeneration, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { getInitials } from "@/lib/avatar";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { UiButton } from "@/shared/ui/button/button";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import { listControlAgentDirectoryApi, type ControlMemberDirectoryEntry, type ControlAgentDirectoryEntry } from "@/lib/api/account/control-api";

import { MessageUserSection } from "@/features/conversation/shared/message/item/view/user/message-user-section";
import { ContentRenderer } from "@/features/conversation/shared/message/item/view/content/content-renderer";
import { MessageAvatar } from "@/features/conversation/shared/message/ui/message-avatar";
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
import { TeamExecutionThread } from "@/features/team/team-execution-thread";
import { TeamExecutionSurface } from "@/features/team/team-execution-surface";
import { buildRoomHeaderTabs, type RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import { buildRoomAgentSessionKey } from "@/lib/conversation/session-key";
import { useAgentStore } from "@/store/agent";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import { useTeamRefresh } from "@/features/team/use-team-refresh";
import { getTeamNode, prepareTeamRoom, type TeamNodeJob, type TeamRoomBinding } from "@/lib/api/conversation/team-node-api";
import { ThreadActionButton } from "@/features/conversation/room/group/thread/round-card/thread-action-button";
import { TeamNodeDialog } from "@/features/team/team-node-dialog";
import type { TeamMessage } from "@/lib/api/conversation/team-api";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { hasOrganizationAccess, useAuth } from "@/shared/auth/auth-context";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { cn } from "@/shared/ui/class-name";
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
  const [searchParams] = useSearchParams();
  const { t } = useI18n();
  const { status } = useAuth();
  const canUseRelay = hasOrganizationAccess(status);
  const room = useTeamRoom(roomId);
  const [membersOpen, setMembersOpen] = useState(false);
  const [nodeOpen, setNodeOpen] = useState(false);
  const [jobs, setJobs] = useState<TeamNodeJob[]>([]);
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
  });
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
    sessionKey: room.room?.conversation.id ?? null,
  });
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
  const sendMessage = async (content: string, _policy: unknown, _attachments?: unknown, targets: string[] = []) => {
    if (!room.room || room.isSending || room.hasUnconfirmedSend) throw new Error("在线消息当前不可发送");
    const ok = await (targets.length ? room.send(content, targets) : room.send(content));
    if (!ok) throw new Error("在线消息未确认");
  };
  const composerScope = JSON.stringify([status?.organization_id, status?.control_user_id, roomId]);
  // 会话绑定独立于任务历史；暂停投递不剥夺本人查看和配置成员的能力。
  const memberAgentIDs = new Set(room.room?.members.filter((member) => member.member_type === "agent" && member.state === "active").map((member) => member.member_id));
  const executionBindings = room.room ? bindings.filter((binding) => memberAgentIDs.has(binding.agent_id)) : [];
  const executionAgents = ownedAgents.filter((agent) => executionBindings.some((binding) => binding.local_agent_id === agent.agent_id));
  const executionAgentId = executionAgents.some((agent) => agent.agent_id === selectedLocalAgent) ? selectedLocalAgent : executionAgents[0]?.agent_id ?? "";
  const executionBinding = executionBindings.find((binding) => binding.local_agent_id === executionAgentId);

  if (!canUseRelay) {
    return <Navigate replace to={APP_ROUTE_PATHS.home} />;
  }

  return (
    <>
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
                    <UiButton aria-label={t("team.node_title")} className="workspace-surface-header-control-segment h-9 min-h-0 gap-1.5 px-2.5" onClick={() => setNodeOpen(true)} size="md" variant="ghost">
                      <MonitorCheck aria-hidden="true" className="h-3.5 w-3.5" />
                      <span className="max-sm:hidden">{t("team.node_title")}</span>
                    </UiButton>
                    <GroupMemberAvatarStack members={headerMembers} onClick={() => setMembersOpen(true)} />
                  </>
                ) : null}
              />
            </div>
          )}
        >
        <div className="flex h-full min-h-0 min-w-0 flex-1">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
        <ConversationPanelLayout>
          <ConversationPanelViewportArea>
            <ConversationPanelViewport
              floatingDockOccupied={false}
              isMobileLayout={isCompact}
              viewport={{ ...scroll, isHistoryLoading: false, ariaLabel: t("team.shared_room") }}
            >
              <div ref={scroll.feedRef} className="min-h-full">
              <TeamMessageFeed
                jobs={jobs} selectedThreadID={thread?.id} onOpenThread={(job) => setThread(thread?.id === job.id ? null : job)}
                agentDirectory={agentDirectory}
                memberDirectory={memberDirectory} directUserId={directUserId}
                currentUserId={status?.control_user_id ?? status?.user_id ?? null}
                isCompact={isCompact}
                isLoading={room.isLoading}
                loadFailed={room.error === "load"}
                messages={room.messages}
              />
              <div ref={scroll.bottomAnchorRef} />
              </div>
            </ConversationPanelViewport>
            <div className="pointer-events-none absolute inset-x-0 bottom-2 grid justify-items-center">
              <ScrollToLatestButton visible={scroll.showScrollToBottom} onClick={() => scroll.scrollToBottom()} />
            </div>
          </ConversationPanelViewportArea>

          <div className="relative z-10 shrink-0" data-conversation-bottom-area>
            {bindingsFailed || jobsFailed || errorMessage ? (
              <div className={`${CONVERSATION_COMPOSER_LANE_CLASS_NAME} grid gap-2 px-6 pb-2`}>
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
              compact={isCompact} commandCatalog={{commands: [], status: "unavailable"}}
              contextUsage={null} showActionMenu={false} defaultPlaceholder={t(directUserId ? "team.direct_placeholder" : "team.message_placeholder")}
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
              onPrepareAttachments={async () => []} goalScopeLabel="" tourAnchor=""
              interactionSurface={room.hasUnconfirmedSend ? <div className="p-4">
                <p className="whitespace-pre-wrap break-words">{room.pendingText}</p>
                <UiButton disabled={room.isSending || !room.room} onClick={() => { void room.send(room.pendingText ?? ""); }} size="sm" variant="surface">{t("state.retry")}</UiButton>
              </div> : undefined}
            />
            </fieldset>
          </div>
        </ConversationPanelLayout>
        </div>
        {room.room && activeTab !== "chat" ? <aside className={isCompact ? "contents" : "h-full min-h-0 w-[42%] min-w-80 border-l divider-subtle"}>
          <TeamExecutionSurface roomId={room.room.room.id} tab={activeTab} agents={executionAgents} binding={executionBinding} selectedAgentId={executionAgentId} compact={isCompact}
            activeWorkspacePath={workspaceFile?.agentId === executionAgentId ? workspaceFile.path : null}
            onOpenWorkspaceFile={(path) => { setWorkspaceFile({agentId: executionAgentId, path}); setActiveTab("workspace"); }}
            onSelectAgent={(id) => { setSelectedLocalAgent(id); setWorkspaceFile(null); }} onClose={() => setActiveTab("chat")} />
        </aside> : null}
        {thread ? <aside className={isCompact ? "contents" : "h-full min-h-0 w-[42%] min-w-80 border-l divider-subtle"}>
          <TeamExecutionThread key={thread.id} job={thread} name={agentDirectory.find((agent) => agent.agent_id === thread.agent_id)?.name ?? thread.agent_id}
            avatar={agentDirectory.find((agent) => agent.agent_id === thread.agent_id)?.avatar || undefined}
            compact={isCompact} onClose={() => setThread(null)} onOpenWorkspaceFile={(path, workspaceAgentId) => {
              if (workspaceAgentId !== undefined && workspaceAgentId !== thread.local_agent_id) return;
              setSelectedLocalAgent(thread.local_agent_id!); setWorkspaceFile({agentId: thread.local_agent_id!, path}); setSelectedThreadID(null); setActiveTab("workspace");
            }} />
        </aside> : null}
        </div>
        </WorkspaceSurfaceScaffold>
      </WorkspacePageFrame>
      {nodeOpen ? <TeamNodeDialog onClose={() => setNodeOpen(false)} /> : null}
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

function TeamMessageFeed({
  jobs, selectedThreadID, onOpenThread,
  agentDirectory,
  memberDirectory, directUserId,
  currentUserId,
  isCompact,
  isLoading,
  loadFailed,
  messages,
}: {
  jobs: TeamNodeJob[]; selectedThreadID?: string; onOpenThread: (job: TeamNodeJob) => void;
  agentDirectory: ControlAgentDirectoryEntry[];
  memberDirectory: ControlMemberDirectoryEntry[]; directUserId?: string;
  currentUserId: string | null;
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
  return (
    <ol aria-busy={isLoading || undefined} className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex flex-col gap-5`}>
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
        <TeamMessageItem
          agent={message.author_type === "agent" ? agentsByID.get(message.author_agent_id ?? "") : undefined}
          person={memberDirectory.find((member) => member.user_id === message.author_user_id)}
          currentUserId={currentUserId}
          isCompact={isCompact}
          key={message.id}
          message={message}
          threadAction={jobs.filter((job) => job.source_message_id === message.id || (message.delivery_id && job.delivery_id === message.delivery_id)).map((job) => <ThreadActionButton key={job.id} active={selectedThreadID === job.id} agentName={agentsByID.get(job.agent_id)?.name ?? job.agent_id} onClick={() => onOpenThread(job)} />)}
        />
      ))}
      {directUserId ? <li><TeamInvitationList {...invitations} invitations={pending.filter((item) => !messages.some((message) => message.content.blocks.some((block) => block.room_id === item.room.id && new Date(block.invited_at ?? "").getTime() === new Date(item.created_at).getTime())))} recoveryRooms={[]}
        onRefresh={invitations.refresh} onResolve={invitations.resolve} onRecover={invitations.recover} /></li> : null}
    </ol>
  );
}

function TeamMessageItem({
  threadAction,
  person,
  agent,
  currentUserId,
  isCompact,
  message,
}: {
  threadAction?: import("react").ReactNode;
  agent?: ControlAgentDirectoryEntry;
  person?: ControlMemberDirectoryEntry;
  currentUserId: string | null;
  isCompact: boolean;
  message: TeamMessage;
}) {
  const content = message.content.blocks.map((block) => block.text).join("\n\n");
  if (message.author_type === "user" && message.author_user_id === currentUserId) {
    return (
      <li>
        <MessageUserSection
          compact={isCompact}
          message={{
            agent_id: "",
            client_message_id: message.client_message_id,
            content,
            conversation_id: message.conversation_id,
            message_id: message.id,
            role: "user",
            room_id: null,
            round_id: message.id,
            session_key: `team:${message.conversation_id}`,
            timestamp: new Date(message.created_at).getTime(),
          }}
        />
        <div className="flex flex-wrap justify-end gap-2">{threadAction}</div>
      </li>
    );
  }
  const author = agent?.name || person?.display_name || person?.username || message.author_display_name || message.author_username || "?";
  return (
    <li className="nexus-chat-message-section px-0 sm:px-3">
      <div className="flex min-w-0 gap-3">
        <MessageAvatar avatarUrl={agent?.avatar || person?.avatar} title={author}>
          <span aria-hidden="true" className={getUiTypographyClassName({ role: "supporting", weight: "semibold" })}>{getInitials(author, "?", 1)}</span>
        </MessageAvatar>
        <div className="min-w-0 flex-1">
          <div className="mb-1 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className={cn("min-w-0 break-words", getUiTypographyClassName({ role: "supporting", weight: "semibold", tone: "strong" }))}>{author}</span>
            {threadAction}
            {message.author_type === "agent" ? <span className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>Agent</span> : null}
            <time className={getUiTypographyClassName({ role: "metadata", tone: "muted" })} dateTime={message.created_at}>
              {formatMessageTime(new Date(message.created_at).getTime())}
            </time>
          </div>
          <ContentRenderer
            className="nexus-chat-message-body-rhythm text-md leading-7 text-(--text-strong)"
            content={content}
          />
        </div>
      </div>
    </li>
  );
}

const TEAM_ERROR_KEYS = {
  load: "team.error_load",
  send: "team.error_send",
  sync: "team.error_sync",
} as const;
