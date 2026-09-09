// INPUT: Relay Team Room 快照、当前 Control 用户与消息发送动作。
// OUTPUT: 复用 Room Header、FOLLOW/READING 阅读轨道、本人消息和 Composer，保留独立读取重试。
// POS: Relay 真人消息到 Nexus Room UI 的窄适配层；不拥有同步与投递规则。

import { FormEvent, useState } from "react";
import { Navigate, useSearchParams } from "react-router-dom";
import { getInitials } from "@/lib/avatar";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import { ScrollToLatestButton } from "@/features/conversation/shared/scroll-to-latest-button";
import { UiButton } from "@/shared/ui/button/button";

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
import type { TeamMessage } from "@/lib/api/conversation/team-api";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { isRemoteAccountAuthenticated, useAuth } from "@/shared/auth/auth-context";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { cn } from "@/shared/ui/class-name";
import { UiRoomAvatar } from "@/shared/ui/display/avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { WorkspacePageFrame } from "@/shared/ui/workspace/frame/workspace-page-frame";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import "@/features/conversation/room/surface/room-conversation-header-edge.css";

export function TeamPage() {
  const { t } = useI18n();
  const { status } = useAuth();
  const canUseRelay = isRemoteAccountAuthenticated(status);
  const [searchParams] = useSearchParams();
  const room = useTeamRoom(searchParams.get("room_id"));
  const [draft, setDraft] = useState("");
  const scroll = useFollowScroll({
    messageCount: room.messages.length,
    sessionKey: room.room?.conversation.id ?? null,
  });
  const isCompact = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
  const title = room.room?.room.name ?? t("team.shared_room");
  const errorMessage = room.error ? t(TEAM_ERROR_KEYS[room.error]) : null;

  const sendDraft = async () => {
    if (!draft.trim() || room.isSending || !room.room) return;
    if (await room.send(draft)) {
      setDraft("");
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
    <WorkspacePageFrame contentPaddingClassName="p-0">
      <WorkspaceSurfaceScaffold
        bodyClassName="relative"
        header={(
          <div className="nexus-room-conversation-header-edge" data-room-conversation-header-edge="true">
            <WorkspaceSurfaceHeader
              leading={(
                <UiRoomAvatar
                  members={[]}
                  roomId={room.room?.room.id}
                  size="md"
                  title={title}
                />
              )}
              leadingVariant="identity"
              title={title}
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
                currentUserId={status?.user_id ?? null}
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
              <p className={`${CONVERSATION_COMPOSER_LANE_CLASS_NAME} px-6 text-xs text-destructive`} role="alert">
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
                    <div className="flex items-end gap-2 px-3.5 pb-0.5 pt-1.5">
                      <textarea
                        aria-label={t("team.message")}
                        className={COMPOSER_TEXTAREA_CLASS_NAME}
                        disabled={!room.room || room.isSending}
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
                        value={draft}
                      />
                    </div>
                    <div className={COMPOSER_FOOTER_CLASS_NAME}>
                      <span aria-hidden="true" className="nexus-chat-composer-footer-leading" />
                      <ComposerPoweredByNexus visible />
                      <div className="nexus-chat-composer-footer-trailing flex justify-self-end">
                        <ComposerSubmitButton
                          isDisabled={!draft.trim() || room.isSending || !room.room}
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
  );
}

function TeamMessageFeed({
  currentUserId,
  isCompact,
  isLoading,
  loadFailed,
  messages,
}: {
  currentUserId: string | null;
  isCompact: boolean;
  isLoading: boolean;
  loadFailed: boolean;
  messages: TeamMessage[];
}) {
  const { t } = useI18n();
  if (loadFailed && !isLoading && messages.length === 0) return null;
  if (isLoading || messages.length === 0) {
    return (
      <div role={isLoading ? "status" : undefined} className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex h-full min-h-64 items-center justify-center text-sm text-(--text-soft)`}>
        {t(isLoading ? "team.loading" : "team.empty")}
      </div>
    );
  }
  return (
    <ol className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex flex-col gap-5`}>
      {messages.map((message) => (
        <TeamMessageItem
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
  currentUserId,
  isCompact,
  message,
}: {
  currentUserId: string | null;
  isCompact: boolean;
  message: TeamMessage;
}) {
  const content = message.content.blocks.map((block) => block.text).join("\n\n");
  if (message.author_user_id === currentUserId) {
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
  const author = message.author_display_name || message.author_username || "?";
  return (
    <li className="nexus-chat-message-section px-0 sm:px-3">
      <div className="flex min-w-0 gap-3">
        <MessageAvatar title={author}>
          <span aria-hidden="true" className="text-sm font-semibold">{getInitials(author, "?", 1)}</span>
        </MessageAvatar>
        <div className="min-w-0 flex-1">
          <div className="mb-1 flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className="min-w-0 break-words text-sm font-semibold text-(--text-strong)">{author}</span>
            <time className="text-xs text-(--text-soft)" dateTime={message.created_at}>
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
