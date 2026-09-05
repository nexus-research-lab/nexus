/**
 * INPUT: 当前 Agent、联络读模型、Session、私信事件、失败事实与页面命令。
 * OUTPUT: 编排独立目录、共享聊天面板、Header 与删除确认的 Agent 联络工作面。
 * POS: Contacts 详情“联络”根编排；不定义目录行、添加表单或资源状态样式。
 */
"use client";

import {
  ArrowLeft,
  MessageCircle,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { useMemo } from "react";

import { ComposerPanel } from "@/features/conversation/shared/composer/composer-panel";
import {
  ConversationPanelBottomArea,
  ConversationPanelLayout,
  ConversationPanelViewport,
  ConversationPanelViewportArea,
} from "@/features/conversation/shared/conversation-panel-layout";
import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "@/features/conversation/shared/conversation-panel-styles";
import { MessageItem } from "@/features/conversation/shared/message/item/message-item";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import { useConversationHistoryLoader } from "@/features/conversation/shared/timeline/use-history-loader";
import { RoomHistoryMenu } from "@/features/conversation/room/surface/history/room-history-menu";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { RoomConversationTabs } from "@/features/navigation/conversation-tabs/room-conversation-tabs";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { Agent, AgentContact } from "@/types/agent/agent";
import type { InputQueueItem } from "@/types/agent/agent-conversation";
import type {
  AgentCommunicationMutationFailure,
  AgentCommunicationReadFailure,
  AgentCommunicationReadFailureKind,
} from "@/types/agent/communication";
import type { AgentPrivateEvent } from "@/types/agent/private-domain";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { RoomContextAggregate } from "@/types/conversation/room";
import type { CommandCatalogData } from "@/types/generated/protocol";

import { AgentCommunicationDirectory } from "./agent-communication-directory";
import {
  getCommunicationAgentName,
  getCommunicationContactLabel,
} from "./agent-communication-model";
import {
  AgentCommunicationEmptyState,
  AgentCommunicationReadFailureState,
} from "./agent-communication-status";

const EMPTY_COMMAND_CATALOG: CommandCatalogData = {
  commands: [],
  status: "unavailable",
};
const EMPTY_INPUT_QUEUE: InputQueueItem[] = [];
const HIDDEN_SCROLL_CONTROL = {
  isGenerating: false,
  onClick: ignoreAction,
  visible: false,
} as const;
const UNAVAILABLE_ROUND_INDEX_RESOURCE = {
  access: null,
  error: null,
  isLoading: false,
  isStale: false,
  retry: ignoreAction,
} as const;

export interface AgentCommunicationViewState {
  contacts: AgentContact[];
  conversationId: string | null;
  conversationFailure: AgentCommunicationReadFailure | null;
  directEvents: AgentPrivateEvent[];
  directoryFailure: AgentCommunicationReadFailure | null;
  hasMoreHistory: boolean;
  historyPrependToken: number;
  isDirectoryLoading: boolean;
  isHistoryLoading: boolean;
  isMessagesLoading: boolean;
  isSending: boolean;
  mutationFailure: AgentCommunicationMutationFailure | null;
  pendingAgentId: string | null;
  roomContexts: RoomContextAggregate[];
  selectedContactId: string | null;
}

interface AgentCommunicationViewProps {
  agent: Agent;
  agents: Agent[];
  state: AgentCommunicationViewState;
  onAddContact: (contactAgentId: string, alias: string) => Promise<boolean>;
  onBackToDirectory: () => void;
  onClearMutationFailure: () => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onLoadOlderMessages: () => Promise<boolean>;
  onRefresh: (kind?: AgentCommunicationReadFailureKind) => void;
  onRemoveContact: (contactAgentId: string) => Promise<boolean>;
  onSelectContact: (contactAgentId: string) => void;
  onSelectConversation: (conversationId: string) => void;
  onSendMessage: (content: string) => Promise<void>;
}

export function AgentCommunicationView({
  agent,
  agents,
  onAddContact,
  onBackToDirectory,
  onClearMutationFailure,
  onCreateConversation,
  onLoadOlderMessages,
  onRefresh,
  onRemoveContact,
  onSelectContact,
  onSelectConversation,
  onSendMessage,
  state,
}: AgentCommunicationViewProps) {
  const { t } = useI18n();
  const [removeDialogOpen, setRemoveDialogOpen] = useResettableState(false, agent.agent_id);
  const agentsById = useMemo(
    () => new Map(agents.map((item) => [item.agent_id, item])),
    [agents],
  );
  const selectedContact = useMemo(
    () => state.contacts.find(
      (item) => item.contact_agent_id === state.selectedContactId,
    ) ?? null,
    [state.contacts, state.selectedContactId],
  );
  const conversations = useMemo(
    () => buildConversationViews(state.roomContexts),
    [state.roomContexts],
  );
  const mutationFeedback = state.mutationFailure
    ? buildCommunicationMutationFeedback(
      state.mutationFailure,
      t,
      () => onRefresh(mutationRefreshKind(state.mutationFailure!.kind)),
      onClearMutationFailure,
    )
    : null;

  return (
    <div className="grid min-h-0 min-w-0 flex-1 grid-cols-1 overflow-hidden bg-(--surface-canvas-background) md:grid-cols-[minmax(240px,288px)_minmax(0,1fr)] md:gap-2">
      <FeedbackBannerViewport item={mutationFeedback} />
      <AgentCommunicationDirectory
        agent={agent}
        agents={agents}
        contacts={state.contacts}
        directoryFailure={state.directoryFailure}
        isDirectoryLoading={state.isDirectoryLoading}
        onAddContact={onAddContact}
        onRefresh={() => onRefresh("directory")}
        onSelectContact={onSelectContact}
        pendingAgentId={state.pendingAgentId}
        selectedContactId={state.selectedContactId}
      />

      <main className={cn(
        "min-h-0 min-w-0 flex-col overflow-hidden bg-(--background) md:flex",
        state.selectedContactId ? "flex" : "hidden",
      )}>
        {selectedContact ? (
          <>
            <CommunicationHeader
              contact={selectedContact}
              conversations={conversations}
              currentConversationId={state.conversationId}
              onBack={onBackToDirectory}
              onCreateConversation={onCreateConversation}
              onRefresh={() => onRefresh(
                state.conversationFailure?.kind
                ?? (state.conversationId ? "messages" : "channel"),
              )}
              onRemove={() => setRemoveDialogOpen(true)}
              onSelectConversation={onSelectConversation}
            />
            <ContactConversation
              agent={agent}
              agentsById={agentsById}
              conversationId={state.conversationId}
              events={state.directEvents}
              hasMoreHistory={state.hasMoreHistory}
              historyPrependToken={state.historyPrependToken}
              isHistoryLoading={state.isHistoryLoading}
              isLoading={state.isMessagesLoading}
              isSending={state.isSending}
              onLoadOlderMessages={onLoadOlderMessages}
              onRetryRead={onRefresh}
              onSend={onSendMessage}
              readFailure={state.conversationFailure}
              targetId={selectedContact.contact_agent_id}
            />
          </>
        ) : (
          <AgentCommunicationEmptyState
            icon={MessageCircle}
            label={t("agent_options.contact.select_contact")}
          />
        )}
      </main>
      <ConfirmDialog
        confirmText={t("agent_options.contact.remove_friend")}
        isOpen={removeDialogOpen && selectedContact !== null}
        message={selectedContact
          ? t("agent_options.contact.remove_friend_confirm", {
            name: getCommunicationContactLabel(selectedContact),
          })
          : ""}
        onCancel={() => setRemoveDialogOpen(false)}
        onConfirm={() => {
          if (selectedContact) {
            void onRemoveContact(selectedContact.contact_agent_id).then((removed) => {
              if (removed) {
                setRemoveDialogOpen(false);
              }
            });
          }
        }}
        title={t("agent_options.contact.remove_friend")}
      />
    </div>
  );
}

function CommunicationHeader({
  contact,
  conversations,
  currentConversationId,
  onBack,
  onCreateConversation,
  onRefresh,
  onRemove,
  onSelectConversation,
}: {
  contact: AgentContact;
  conversations: RoomConversationView[];
  currentConversationId: string | null;
  onBack: () => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onRefresh: () => void;
  onRemove: () => void;
  onSelectConversation: (conversationId: string) => void;
}) {
  const { t } = useI18n();
  const label = getCommunicationContactLabel(contact);
  return (
    <div className="shrink-0">
      <WorkspaceSurfaceHeader
        leading={(
          <>
            <UiIconButton
              aria-label={t("common.back")}
              className="h-full w-full md:hidden"
              onClick={onBack}
              size="lg"
              title={t("common.back")}
              variant="ghost"
            >
              <ArrowLeft className="h-4 w-4" />
            </UiIconButton>
            <UiAgentAvatar
              avatar={contact.avatar}
              className="hidden h-full w-full border-0 shadow-none md:flex"
              name={label}
              size="sm"
            />
          </>
        )}
        leadingClassName="h-10 w-10 max-md:border-0 max-md:bg-transparent max-md:shadow-none"
        leadingVariant="identity"
        tabsLeading={(
          <RoomConversationTabs
            conversationId={currentConversationId}
            conversations={conversations}
            leadingControl={conversations.length > 0 ? (
              <RoomHistoryMenu
                canManageConversations={false}
                conversationId={currentConversationId}
                conversations={conversations}
                onCreateConversation={onCreateConversation}
                onDeleteConversation={ignoreConversationDelete}
                onSelectConversation={onSelectConversation}
                triggerVariant="session"
              />
            ) : undefined}
            onCreateConversation={onCreateConversation}
            onSelectConversation={onSelectConversation}
          />
        )}
        trailing={(
          <div className="flex items-center gap-0.5">
            <UiIconButton
              aria-label={t("agent_options.contact.refresh")}
              onClick={onRefresh}
              size="sm"
              title={t("agent_options.contact.refresh")}
              variant="ghost"
            >
              <RefreshCw className="h-4 w-4" />
            </UiIconButton>
            <UiIconButton
              aria-label={t("agent_options.contact.remove_friend")}
              onClick={onRemove}
              size="sm"
              title={t("agent_options.contact.remove_friend")}
              tone="danger"
              variant="ghost"
            >
              <Trash2 className="h-4 w-4" />
            </UiIconButton>
          </div>
        )}
      />
    </div>
  );
}

function ContactConversation({
  agent,
  agentsById,
  conversationId,
  events,
  hasMoreHistory,
  historyPrependToken,
  isHistoryLoading,
  isLoading,
  isSending,
  onLoadOlderMessages,
  onRetryRead,
  onSend,
  readFailure,
  targetId,
}: {
  agent: Agent;
  agentsById: Map<string, Agent>;
  conversationId: string | null;
  events: AgentPrivateEvent[];
  hasMoreHistory: boolean;
  historyPrependToken: number;
  isHistoryLoading: boolean;
  isLoading: boolean;
  isSending: boolean;
  onLoadOlderMessages: () => Promise<boolean>;
  onRetryRead: (kind: AgentCommunicationReadFailureKind) => void;
  onSend: (content: string) => Promise<void>;
  readFailure: AgentCommunicationReadFailure | null;
  targetId: string;
}) {
  const { t } = useI18n();
  const isCompactLayout = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
  const messages = useMemo(
    () => events
      .filter((event) => Boolean(event.content?.trim()))
      .sort((left, right) => left.timestamp - right.timestamp)
      .map((event) => toConversationMessage(event, agent.agent_id)),
    [agent.agent_id, events],
  );
  const sessionKey = conversationId
    ? `contact:${agent.agent_id}:${targetId}:${conversationId}`
    : null;
  const scrollContentKey = [
    messages[0]?.message_id ?? "",
    messages.at(-1)?.message_id ?? "",
    messages.length,
  ].join(":");
  const scroll = useFollowScroll({
    contentKey: scrollContentKey,
    historyPrependToken,
    messageCount: messages.length,
    sessionKey,
    topologyKey: scrollContentKey,
  });
  const history = useConversationHistoryLoader({
    cancelHistoryPrependRestore: scroll.cancelHistoryPrependRestore,
    hasMoreHistory,
    isHistoryLoading,
    isFollowingLatest: scroll.isFollowingLatest,
    isLoading,
    loadOlderMessages: onLoadOlderMessages,
    messageCount: messages.length,
    onScroll: scroll.onScroll,
    prepareHistoryPrependRestore: scroll.prepareHistoryPrependRestore,
    scrollRef: scroll.scrollRef,
  });

  return (
    <ConversationPanelLayout>
      <ConversationPanelViewportArea>
        <ConversationPanelViewport
          floatingDockOccupied={false}
          isMobileLayout={isCompactLayout}
          viewport={{
            isHistoryLoading,
            onPointerDown: scroll.onPointerDown,
            onScroll: history.handleScroll,
            onTouchEnd: scroll.onTouchEnd,
            onTouchMove: scroll.onTouchMove,
            onTouchStart: scroll.onTouchStart,
            onWheel: scroll.onWheel,
            scrollRef: scroll.scrollRef,
          }}
        >
          {isLoading && messages.length === 0 ? (
            <AgentCommunicationEmptyState
              label={t("agent_options.contact.loading_messages")}
              loading
            />
          ) : readFailure && !readFailure.stale ? (
            <AgentCommunicationReadFailureState
              failure={readFailure}
              onRetry={() => onRetryRead(readFailure.kind)}
            />
          ) : messages.length === 0 ? (
            <>
              {readFailure ? (
                <AgentCommunicationReadFailureState
                  compact
                  failure={readFailure}
                  onRetry={() => onRetryRead(readFailure.kind)}
                />
              ) : null}
              <AgentCommunicationEmptyState
                icon={MessageCircle}
                label={t("agent_options.contact.empty_messages")}
              />
            </>
          ) : (
            <>
              {readFailure ? (
                <AgentCommunicationReadFailureState
                  compact
                  failure={readFailure}
                  onRetry={() => onRetryRead(readFailure.kind)}
                />
              ) : null}
              <div
                className={`nexus-chat-feed ${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex min-h-full flex-col justify-end`}
                ref={scroll.feedRef}
              >
                {messages.map((message, index) => {
                  const source = agentsById.get(message.agent_id);
                  return (
                    <MessageItem
                      animateEntry={false}
                      assistantContentMode="dm_archived"
                      compact={isCompactLayout}
                      currentAgentAvatar={source?.avatar}
                      currentAgentName={source
                        ? getCommunicationAgentName(source)
                        : message.agent_id}
                      isLastRound={index === messages.length - 1}
                      key={message.message_id}
                      messages={[message]}
                      roundId={message.round_id}
                      workspaceAgentId={message.agent_id}
                    />
                  );
                })}
              </div>
            </>
          )}
        </ConversationPanelViewport>
      </ConversationPanelViewportArea>
      <ConversationPanelBottomArea
        isMobileLayout={isCompactLayout}
        isReconciling={false}
        onReconcile={() => undefined}
        providerWarningVisible={false}
        reliability={{
          failure: null,
          provider_retry: null,
          transport_phase: "healthy",
        }}
        roundIndexResource={UNAVAILABLE_ROUND_INDEX_RESOURCE}
        scrollToLatest={HIDDEN_SCROLL_CONTROL}
      >
        <ComposerPanel
          commandCatalog={EMPTY_COMMAND_CATALOG}
          compact={isCompactLayout}
          contextUsage={null}
          defaultDeliveryPolicy="queue"
          draftScopeKey={`contact:${agent.agent_id}:${targetId}:${conversationId ?? "new"}`}
          goalScopeLabel={getCommunicationAgentName(agent)}
          historyScopeKey={`contact:${agent.agent_id}:${targetId}`}
          inputQueueItems={EMPTY_INPUT_QUEUE}
          isLoading={isSending}
          onDeleteQueuedMessage={ignoreQueuedMessage}
          onEnqueueMessage={onSend}
          onGuideQueuedMessage={ignoreQueuedMessage}
          onPrepareAttachments={prepareNoAttachments}
          onReorderQueueMessages={ignoreQueueOrder}
          onSendMessage={onSend}
          queueWhenSessionBusy={false}
          roomMembers={[]}
          runtimeKind="nxs"
          runtimePhase={isSending ? "sending" : null}
          showActionMenu={false}
          tourAnchor=""
        />
      </ConversationPanelBottomArea>
    </ConversationPanelLayout>
  );
}

function buildCommunicationMutationFeedback(
  failure: AgentCommunicationMutationFailure,
  t: ReturnType<typeof useI18n>["t"],
  onRefresh: () => void,
  onClear: () => void,
): FeedbackBannerProps {
  const operationTitle = {
    add_contact: "agent_options.contact.add_contact_not_completed",
    create_conversation: "agent_options.contact.create_conversation_not_completed",
    remove_contact: "agent_options.contact.remove_contact_not_completed",
    send_message: "agent_options.contact.send_message_not_completed",
  }[failure.kind] as
    | "agent_options.contact.add_contact_not_completed"
    | "agent_options.contact.create_conversation_not_completed"
    | "agent_options.contact.remove_contact_not_completed"
    | "agent_options.contact.send_message_not_completed";
  const effectCopies: Record<
    AgentCommunicationMutationFailure["effect"],
    { impact: TranslationKey; tone: "error" | "warning" }
  > = {
    accepted: {
      impact: "agent_options.contact.mutation_accepted_impact",
      tone: "warning",
    },
    committed: {
      impact: "agent_options.contact.mutation_committed_impact",
      tone: "warning",
    },
    not_applied: {
      impact: "agent_options.contact.mutation_not_applied_impact",
      tone: "error",
    },
    unknown: {
      impact: "agent_options.contact.mutation_unknown_impact",
      tone: "warning",
    },
  };
  const effectCopy = effectCopies[failure.effect];
  return {
    action: {
      label: t("agent_options.contact.check_latest_state"),
      onClick: onRefresh,
    },
    impact: t(effectCopy.impact),
    ...(failure.blocksRepeat ? {} : { onDismiss: onClear }),
    title: t(operationTitle),
    tone: effectCopy.tone,
    urgency: failure.effect === "not_applied" ? "assertive" : "polite",
  };
}

function mutationRefreshKind(
  kind: AgentCommunicationMutationFailure["kind"],
): AgentCommunicationReadFailureKind {
  switch (kind) {
    case "add_contact":
    case "remove_contact":
      return "directory";
    case "create_conversation":
      return "channel";
    case "send_message":
      return "messages";
  }
}

function buildConversationViews(contexts: RoomContextAggregate[]): RoomConversationView[] {
  return contexts.map((context) => ({
    agent_id: undefined,
    conversation_id: context.conversation.id,
    conversation_type: context.conversation.conversation_type,
    created_at: timestamp(context.conversation.created_at),
    is_active: false,
    is_draft: context.conversation.is_draft,
    last_activity_at: timestamp(context.conversation.last_activity_at)
      || timestamp(context.conversation.updated_at)
      || timestamp(context.conversation.created_at),
    message_count: context.conversation.message_count ?? 0,
    options: {},
    room_id: context.room.id,
    session_id: null,
    session_key: buildRoomSharedSessionKey(context.conversation.id),
    title: context.conversation.title?.trim() || "",
  })).sort((left, right) => right.last_activity_at - left.last_activity_at);
}

function toConversationMessage(
  event: AgentPrivateEvent,
  currentAgentId: string,
): Message {
  const base = {
    agent_id: event.source_agent_id,
    conversation_id: event.conversation_id,
    message_id: event.message_id,
    room_id: event.room_id,
    round_id: event.message_id,
    session_key: event.conversation_id
      ? buildRoomSharedSessionKey(event.conversation_id)
      : "",
    timestamp: event.timestamp,
  };
  if (event.source_agent_id === currentAgentId) {
    return {
      ...base,
      content: event.content?.trim() ?? "",
      role: "user",
    };
  }
  return {
    ...base,
    content: [{ type: "text", text: event.content?.trim() ?? "" }],
    is_complete: true,
    role: "assistant",
    stop_reason: "end_turn",
  };
}

function timestamp(value?: string | null): number {
  const parsed = value ? new Date(value).getTime() : 0;
  return Number.isFinite(parsed) ? parsed : 0;
}

function ignoreAction(): void {}

async function ignoreConversationDelete(): Promise<null> {
  return null;
}

async function ignoreQueuedMessage(): Promise<void> {}

async function ignoreQueueOrder(_orderedIds: string[]): Promise<void> {}

async function prepareNoAttachments(): Promise<[]> {
  return [];
}
