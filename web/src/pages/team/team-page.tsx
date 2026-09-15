// INPUT: Relay Team Room 快照、当前 Control 用户与消息发送动作。
// OUTPUT: 复用 Room Header、FOLLOW/READING 阅读轨道、本人消息和 Composer，保留独立读取重试。
// POS: Relay 真人消息与完整 Agent 回复到 Nexus Room UI 的窄适配层；不推断运行态或流式输出。

import { FormEvent, useEffect, useState, useSyncExternalStore } from "react";
import { MonitorCheck } from "lucide-react";
import { Navigate, useSearchParams } from "react-router-dom";
import { captureAuthOwnerScopeGeneration, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { getInitials } from "@/lib/avatar";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { UiButton } from "@/shared/ui/button/button";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { listControlAgentDirectoryApi, type ControlAgentDirectoryEntry } from "@/lib/api/account/control-api";

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
import { ComposerSubmitButton } from "@/features/conversation/shared/composer/components/composer-submit-button";
import { ComposerPoweredByNexus } from "@/features/conversation/shared/composer/components/footer/composer-footer";
import {
  COMPOSER_COMPACT_LANE_CLASS_NAME,
  COMPOSER_FOOTER_CLASS_NAME,
  COMPOSER_SHELL_CLASS_NAME,
  COMPOSER_TEXTAREA_CLASS_NAME,
  COMPOSER_TEXTAREA_MAX_HEIGHT_PX,
} from "@/features/conversation/shared/composer/composer-styles";
import { formatMessageTime } from "@/features/conversation/shared/message/message-time";
import { useTeamRoom } from "@/features/team/use-team-room";
import { useTeamMembers } from "@/features/team/use-team-members";
import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { TeamRoomMembersDialog } from "@/features/team/team-room-members-dialog";
import { TeamNodeDialog } from "@/features/team/team-node-dialog";
import type { TeamMessage } from "@/lib/api/conversation/team-api";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { hasOrganizationAccess, useAuth } from "@/shared/auth/auth-context";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { cn } from "@/shared/ui/class-name";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
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
  const { t } = useI18n();
  const { status } = useAuth();
  const canUseRelay = hasOrganizationAccess(status);
  const room = useTeamRoom(roomId);
  const [membersOpen, setMembersOpen] = useState(false);
  const [nodeOpen, setNodeOpen] = useState(false);
	const [agentDirectory, setAgentDirectory] = useState<ControlAgentDirectoryEntry[]>([]);
	const [mentionedAgentIDs, setMentionedAgentIDs] = useState<string[]>([]);
	const memberDirectory = useTeamMembers(canUseRelay);
	const localAgents = useHomeDirectory().agents;
	useEffect(() => {
		if (!canUseRelay) return;
        let cancelled = false;
		void listControlAgentDirectoryApi().then((agents) => { if (!cancelled) setAgentDirectory(agents); }).catch(() => { if (!cancelled) setAgentDirectory([]); });
        return () => { cancelled = true; };
	}, [canUseRelay, room.room?.room.membership_version]);
  const [draft, setDraft] = useState("");
  const scroll = useFollowScroll({
    messageCount: room.messages.length,
    sessionKey: room.room?.conversation.id ?? null,
  });
  const isCompact = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
  const title = room.room?.room.name ?? t("team.shared_room");
  const headerMembers = (room.room?.members ?? []).filter((member) => member.state === "active").map((member) => {
    const agent = member.member_type === "agent" ? agentDirectory.find((entry) => entry.agent_id === member.member_id) : undefined;
    const person = member.member_type === "user" ? memberDirectory.find((entry) => entry.user_id === member.member_id) : undefined;
    const isSelf = member.member_type === "user" && member.member_id === (status?.control_user_id ?? status?.user_id);
    return {
      agent_id: `${member.member_type}:${member.member_id}`,
      name: agent?.name || person?.display_name || person?.username || (isSelf ? t("team.you") : member.member_id),
      avatar: agent?.avatar || person?.avatar,
    };
  });
  const errorMessage = room.error ? t(TEAM_ERROR_KEYS[room.error]) : null;
	const activeAgentIDs = new Set(room.room?.members
		.filter((member) => member.member_type === "agent" && member.state === "active" && !member.agent_paused)
		.map((member) => member.member_id) ?? []);
	const agentNames = new Map(agentDirectory.map((agent) => [agent.agent_id, agent.name]));
	const mentionCandidates = agentDirectory.filter((agent) => activeAgentIDs.has(agent.agent_id) && !mentionedAgentIDs.includes(agent.agent_id));

  const sendDraft = async () => {
    if (!(room.pendingText ?? draft).trim() || room.isSending || !room.room) return;
	const targets = mentionedAgentIDs;
	const prefix = targets.map((agentID) => `@${agentNames.get(agentID) ?? agentID}`).join(" ");
	const message = prefix ? `${prefix} ${draft.trim()}` : draft;
    if (await (targets.length > 0 ? room.send(message, targets) : room.send(message))) {
      setDraft("");
	  setMentionedAgentIDs([]);
    }
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    void sendDraft();
  };

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
                leading={(
                  <UiRoomAvatar
                    avatar={room.room?.room.avatar}
                    members={headerMembers.map((member) => ({ id: member.agent_id, name: member.name, avatar: member.avatar }))}
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
                navigationTrailing={room.room ? (
                  <>
                    <UiButton aria-label={t("team.node_title")} className="workspace-surface-header-control-segment h-9 gap-1.5 px-2.5" onClick={() => setNodeOpen(true)} size="md" variant="ghost">
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
        <ConversationPanelLayout>
          <ConversationPanelViewportArea>
            <ConversationPanelViewport
              floatingDockOccupied={false}
              isMobileLayout={isCompact}
              viewport={{ ...scroll, isHistoryLoading: false, ariaLabel: t("team.shared_room") }}
            >
              <div ref={scroll.feedRef} className="min-h-full">
              <TeamMessageFeed
                agentDirectory={agentDirectory}
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

          <form className="relative z-10 shrink-0" data-conversation-bottom-area onSubmit={submit}>
            {errorMessage ? (
              <p className={`${CONVERSATION_COMPOSER_LANE_CLASS_NAME} px-6 ${getUiTypographyClassName({ role: "supporting", tone: "danger" })}`} role="alert">
                {errorMessage}
              </p>
            ) : null}
            {room.error === "load" ? (
              <div className={`${CONVERSATION_COMPOSER_LANE_CLASS_NAME} px-6`}>
                <UiButton size="sm" variant="text" disabled={room.isLoading} aria-busy={room.isLoading}
                  onClick={() => { void room.retryLoad(); }}>{t("state.retry")}</UiButton>
              </div>
            ) : null}
            <div data-conversation-composer-anchor>
              <section className={cn(
                "bg-transparent",
                isCompact
                  ? `${COMPOSER_COMPACT_LANE_CLASS_NAME} px-4 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-2`
                  : `${CONVERSATION_COMPOSER_LANE_CLASS_NAME} px-3 pb-2 pt-2 sm:px-5 xl:px-6`,
              )}>
                <div className="nexus-chat-composer-edge relative isolate" data-composer-edge="true">
                  <div className={COMPOSER_SHELL_CLASS_NAME} data-composer-surface="input">
					{activeAgentIDs.size > 0 || mentionedAgentIDs.length > 0 ? (
					  <div className="flex flex-wrap items-center gap-1 px-3.5 pt-2">
						{mentionedAgentIDs.map((agentID) => (
						  <UiButton disabled={room.hasUnconfirmedSend} key={agentID} onClick={() => setMentionedAgentIDs((current) => current.filter((value) => value !== agentID))} size="xs" variant="surface">
							@{agentNames.get(agentID) ?? agentID} ×
						  </UiButton>
						))}
						<UiSelectMenu
						  ariaLabel={t("team.mention_agent")}
						  disabled={room.isSending || room.hasUnconfirmedSend || mentionCandidates.length === 0}
						  onChange={(agentID) => setMentionedAgentIDs((current) => [...current, agentID])}
						  options={mentionCandidates.map((agent) => ({ label: `@${agent.name}`, value: agent.agent_id }))}
						  placeholder={t("team.mention_agent")}
						  size="sm"
						  value=""
						/>
					  </div>
					) : null}
                    <div className="flex items-end gap-2 px-3.5 pb-0.5 pt-1.5">
                      <textarea
                        aria-label={t("team.message")}
                        className={COMPOSER_TEXTAREA_CLASS_NAME}
                        disabled={!room.room || room.isSending || room.hasUnconfirmedSend}
                        onChange={(event) => setDraft(event.target.value)}
                        onKeyDown={(event) => {
                          if (event.defaultPrevented) return;
                          if (event.key === "Enter" && !event.shiftKey && !isImeKeyboardEvent(event.nativeEvent)) {
                            event.preventDefault();
                            void sendDraft();
                          }
                        }}
                        placeholder={t("team.message_placeholder")}
                        rows={1}
                        style={{ maxHeight: COMPOSER_TEXTAREA_MAX_HEIGHT_PX }}
                        value={room.pendingText ?? draft}
                      />
                    </div>
                    <div className={COMPOSER_FOOTER_CLASS_NAME}>
                      <span aria-hidden="true" className="nexus-chat-composer-footer-leading" />
                      <ComposerPoweredByNexus visible />
                      <div className="nexus-chat-composer-footer-trailing flex justify-self-end">
                        <ComposerSubmitButton
                          isDisabled={!(room.pendingText ?? draft).trim() || room.isSending || !room.room}
                          isGoalCreating={false}
                          isGoalMode={false}
                          isPreparingAttachments={room.isSending}
                          onSend={sendDraft}
                          sendLabel={t("team.send")}
                          shouldStop={false}
                          stopLabel={t("composer.stop_generation")}
                        />
                      </div>
                    </div>
                  </div>
                </div>
              </section>
            </div>
          </form>
        </ConversationPanelLayout>
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
  agentDirectory,
  currentUserId,
  isCompact,
  isLoading,
  loadFailed,
  messages,
}: {
  agentDirectory: ControlAgentDirectoryEntry[];
  currentUserId: string | null;
  isCompact: boolean;
  isLoading: boolean;
  loadFailed: boolean;
  messages: TeamMessage[];
}) {
  const { t } = useI18n();
  if (loadFailed && !isLoading && messages.length === 0) return null;
  if (messages.length === 0) {
    return (
      <div role={isLoading ? "status" : undefined} className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex h-full min-h-64 items-center justify-center ${getUiTypographyClassName({ role: "supporting", tone: "muted" })}`}>
        {t(isLoading ? "team.loading" : "team.empty")}
      </div>
    );
  }
  const agentsByID = new Map(agentDirectory.map((agent) => [agent.agent_id, agent]));
  return (
    <ol aria-busy={isLoading || undefined} className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex flex-col gap-5`}>
      {messages.map((message) => (
        <TeamMessageItem
          agent={message.author_type === "agent" ? agentsByID.get(message.author_agent_id ?? "") : undefined}
          currentUserId={currentUserId}
          isCompact={isCompact}
          key={message.id}
          message={message}
        />
      ))}
    </ol>
  );
}

function TeamMessageItem({
  agent,
  currentUserId,
  isCompact,
  message,
}: {
  agent?: ControlAgentDirectoryEntry;
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
      </li>
    );
  }
  const author = agent?.name || message.author_display_name || message.author_username || "?";
  return (
    <li className="nexus-chat-message-section px-0 sm:px-3">
      <div className="flex min-w-0 gap-3">
        <MessageAvatar avatarUrl={agent?.avatar} title={author}>
          <span aria-hidden="true" className={getUiTypographyClassName({ role: "supporting", weight: "semibold" })}>{getInitials(author, "?", 1)}</span>
        </MessageAvatar>
        <div className="min-w-0 flex-1">
          <div className="mb-1 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className={cn("min-w-0 break-words", getUiTypographyClassName({ role: "supporting", weight: "semibold", tone: "strong" }))}>{author}</span>
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
