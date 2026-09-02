/**
 * INPUT: 当前 Agent、好友目录、Session、私信事件、读取失败事实与页面命令。
 * OUTPUT: 以当前 Agent 视角操作的联系人列表、共享聊天面板和就地恢复动作。
 * POS: Contacts 详情“联络”栏目的纯视图与局部交互状态。
 */
"use client";

import {
  ArrowLeft,
  Check,
  LoaderCircle,
  MessageCircle,
  RefreshCw,
  Trash2,
  UserRoundPlus,
  UsersRound,
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
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import { CONVERSATION_FOCUS_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogFormShell,
  UiDialogHeader,
  UiDialogPortal,
} from "@/shared/ui/dialog/dialog";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiField, UiInput, UiSearchInput } from "@/shared/ui/form/form-control";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
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
  const [query, setQuery] = useResettableState("", agent.agent_id);
  const [addDialogOpen, setAddDialogOpen] = useResettableState(false, agent.agent_id);
  const [removeDialogOpen, setRemoveDialogOpen] = useResettableState(false, agent.agent_id);
  const agentsById = useMemo(
    () => new Map(agents.map((item) => [item.agent_id, item])),
    [agents],
  );
  const contacts = useMemo(
    () => filterContacts(state.contacts, query),
    [query, state.contacts],
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
  const availableAgents = useMemo(() => {
    const contactIds = new Set(state.contacts.map((contact) => contact.contact_agent_id));
    return agents.filter((candidate) => (
      candidate.agent_id !== agent.agent_id
      && !candidate.is_main
      && !contactIds.has(candidate.agent_id)
    ));
  }, [agent.agent_id, agents, state.contacts]);
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
      <aside className={cn(
        "min-h-0 min-w-0 flex-col overflow-hidden bg-(--surface-raised-background) md:flex",
        state.selectedContactId ? "hidden" : "flex",
      )}>
        <div className="flex shrink-0 items-center gap-2 px-2 py-3">
          <UiSearchInput
            className="min-w-0 flex-1"
            controlSize="sm"
            onChange={setQuery}
            placeholder={t("agent_options.contact.search_contacts")}
            value={query}
          />
          <UiIconButton
            aria-label={t("agent_options.contact.add_friend")}
            onClick={() => setAddDialogOpen(true)}
            size="lg"
            title={t("agent_options.contact.add_friend")}
            variant="ghost"
          >
            <UserRoundPlus className="h-[22px] w-[22px]" />
          </UiIconButton>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto p-2">
          {state.isDirectoryLoading
          && contacts.length === 0
          && !state.directoryFailure ? (
            <EmptyState icon={LoaderCircle} label={t("agent_options.contact.loading_address_book")} spin />
          ) : state.directoryFailure && !state.directoryFailure.stale ? (
            <CommunicationReadFailure
              failure={state.directoryFailure}
              onRetry={() => onRefresh("directory")}
            />
          ) : contacts.length === 0 ? (
            <>
              {state.directoryFailure ? (
                <CommunicationReadFailure
                  compact
                  failure={state.directoryFailure}
                  onRetry={() => onRefresh("directory")}
                />
              ) : null}
              <EmptyState
                icon={query ? MessageCircle : UsersRound}
                label={query
                  ? t("agent_options.contact.no_search_results")
                  : t("agent_options.contact.empty_directory")}
              />
            </>
          ) : (
            <>
              {state.directoryFailure ? (
                <CommunicationReadFailure
                  compact
                  failure={state.directoryFailure}
                  onRetry={() => onRefresh("directory")}
                />
              ) : null}
              <div className="space-y-0.5">
                {contacts.map((contact) => (
                  <ContactRow
                    contact={contact}
                    isSelected={state.selectedContactId === contact.contact_agent_id}
                    key={contact.contact_agent_id}
                    onSelect={() => onSelectContact(contact.contact_agent_id)}
                  />
                ))}
              </div>
            </>
          )}
        </div>
      </aside>

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
          <EmptyState icon={MessageCircle} label={t("agent_options.contact.select_contact")} />
        )}
      </main>

      {addDialogOpen ? (
        <AddContactDialog
          agentId={agent.agent_id}
          agents={availableAgents}
          isPending={Boolean(state.pendingAgentId)}
          onAdd={onAddContact}
          onClose={() => setAddDialogOpen(false)}
        />
      ) : null}
      <ConfirmDialog
        confirmText={t("agent_options.contact.remove_friend")}
        isOpen={removeDialogOpen && selectedContact !== null}
        message={selectedContact
          ? t("agent_options.contact.remove_friend_confirm", { name: contactLabel(selectedContact) })
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

function ContactRow({
  contact,
  isSelected,
  onSelect,
}: {
  contact: AgentContact;
  isSelected: boolean;
  onSelect: () => void;
}) {
  const label = contactLabel(contact);
  return (
    <button
      className={cn(
        "flex w-full min-w-0 items-center gap-2.5 rounded-[9px] border border-transparent px-2.5 py-2.5 text-left transition-colors",
        isSelected
          ? SIDEBAR_SELECTION_CLASS_NAME
          : "hover:bg-(--surface-interactive-hover-background)",
      )}
      onClick={onSelect}
      type="button"
    >
      <UiAgentAvatar avatar={contact.avatar} name={label} size="md" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-semibold text-(--text-strong)">
          {label}
        </span>
        {contact.alias?.trim() ? (
          <span className="mt-0.5 block truncate text-xs text-(--text-soft)">
            {contact.display_name?.trim() || contact.name}
          </span>
        ) : null}
      </span>
    </button>
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
  const label = contactLabel(contact);
  return (
    <div className="shrink-0">
      <WorkspaceSurfaceHeader
        leading={(
          <>
            <button
              aria-label={t("common.back")}
              className="flex h-full w-full items-center justify-center md:hidden"
              onClick={onBack}
              title={t("common.back")}
              type="button"
            >
              <ArrowLeft className="h-4 w-4" />
            </button>
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
          <WorkspaceConversationTabs
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
  const isCompactLayout = useMediaQuery(CONVERSATION_FOCUS_MEDIA_QUERY);
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
            <EmptyState icon={LoaderCircle} label={t("agent_options.contact.loading_messages")} spin />
          ) : readFailure && !readFailure.stale ? (
            <CommunicationReadFailure
              failure={readFailure}
              onRetry={() => onRetryRead(readFailure.kind)}
            />
          ) : messages.length === 0 ? (
            <>
              {readFailure ? (
                <CommunicationReadFailure
                  compact
                  failure={readFailure}
                  onRetry={() => onRetryRead(readFailure.kind)}
                />
              ) : null}
              <EmptyState icon={MessageCircle} label={t("agent_options.contact.empty_messages")} />
            </>
          ) : (
            <>
              {readFailure ? (
                <CommunicationReadFailure
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
                      currentAgentName={source ? agentName(source) : message.agent_id}
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
          goalScopeLabel={agentName(agent)}
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

function CommunicationReadFailure({
  compact = false,
  failure,
  onRetry,
}: {
  compact?: boolean;
  failure: AgentCommunicationReadFailure;
  onRetry: () => void;
}) {
  const { t } = useI18n();
  const copy = communicationFailureCopy(failure);
  return (
    <UiResourceState
      className={compact ? "mb-2 min-h-0 py-3" : undefined}
      impact={t(copy.impact)}
      primaryAction={{
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: t(copy.action),
        onClick: onRetry,
      }}
      size="sm"
      state="error"
      title={t(copy.title)}
      urgency="polite"
    />
  );
}

function communicationFailureCopy(failure: AgentCommunicationReadFailure) {
  switch (failure.kind) {
    case "directory":
      return {
        action: "agent_options.contact.retry_directory" as const,
        impact: failure.stale
          ? "agent_options.contact.directory_stale_impact" as const
          : "agent_options.contact.directory_unavailable_impact" as const,
        title: "agent_options.contact.directory_load_failed" as const,
      };
    case "channel":
      return {
        action: "agent_options.contact.retry_channel" as const,
        impact: failure.stale
          ? "agent_options.contact.channel_stale_impact" as const
          : "agent_options.contact.channel_unavailable_impact" as const,
        title: "agent_options.contact.channel_load_failed" as const,
      };
    case "history":
      return {
        action: "agent_options.contact.retry_history" as const,
        impact: failure.stale
          ? "agent_options.contact.history_stale_impact" as const
          : "agent_options.contact.history_unavailable_impact" as const,
        title: "agent_options.contact.history_load_failed" as const,
      };
    case "messages":
      return {
        action: "agent_options.contact.retry_messages" as const,
        impact: failure.stale
          ? "agent_options.contact.messages_stale_impact" as const
          : "agent_options.contact.messages_unavailable_impact" as const,
        title: "agent_options.contact.messages_load_failed" as const,
      };
  }
}

function AddContactDialog({
  agentId,
  agents,
  isPending,
  onAdd,
  onClose,
}: {
  agentId: string;
  agents: Agent[];
  isPending: boolean;
  onAdd: (contactAgentId: string, alias: string) => Promise<boolean>;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const [query, setQuery] = useResettableState("", agentId);
  const [selectedAgentId, setSelectedAgentId] = useResettableState("", agentId);
  const [alias, setAlias] = useResettableState("", agentId);
  const candidates = agents.filter((candidate) => (
    agentName(candidate).toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())
  ));
  const titleId = `add-agent-contact-${agentId}`;
  const submit = async () => {
    if (selectedAgentId && await onAdd(selectedAgentId, alias)) {
      onClose();
    }
  };
  return (
    <UiDialogPortal>
      <UiDialogBackdrop labelledBy={titleId} onClose={onClose}>
        <UiDialogFormShell
          className="max-h-[min(78dvh,620px)]"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
          size="sm"
        >
          <UiDialogHeader
            appearance="plain"
            onClose={onClose}
            title={t("agent_options.contact.add_friend")}
            titleId={titleId}
          />
          <UiDialogBody className="space-y-4" scrollable>
            <UiSearchInput
              className="w-full"
              controlSize="md"
              onChange={setQuery}
              placeholder={t("agent_options.contact.search_agents")}
              value={query}
              variant="dialog"
            />
            <div
              aria-label={t("agent_options.contact.search_agents")}
              className="soft-scrollbar surface-radius-md max-h-72 min-h-36 space-y-0.5 overflow-y-auto border border-(--surface-panel-border) bg-(--surface-panel-background) p-1.5"
              role="listbox"
            >
              {candidates.length === 0 ? (
                <p className="flex min-h-32 items-center justify-center px-3 text-center text-compact text-(--text-soft)">
                  {t("agent_options.contact.no_available_agents")}
                </p>
              ) : candidates.map((candidate) => (
                <button
                  aria-selected={selectedAgentId === candidate.agent_id}
                  className={cn(
                    "flex min-h-12 w-full items-center gap-3 radius-control-md border border-transparent px-2.5 py-2 text-left transition-[background,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]",
                    selectedAgentId === candidate.agent_id
                      ? "bg-(--surface-interactive-active-background)"
                      : "hover:bg-(--surface-interactive-hover-background)",
                  )}
                  key={candidate.agent_id}
                  onClick={() => setSelectedAgentId(candidate.agent_id)}
                  role="option"
                  type="button"
                >
                  <UiAgentAvatar avatar={candidate.avatar} name={agentName(candidate)} size="md" />
                  <span className="min-w-0 flex-1 truncate text-sm font-semibold text-(--text-strong)">
                    {agentName(candidate)}
                  </span>
                  <span className={cn(
                    "flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs",
                    selectedAgentId === candidate.agent_id
                      ? "bg-(--surface-interactive-hover-background) text-(--brand-action)"
                      : "text-transparent",
                  )}>
                    <Check className="h-3.5 w-3.5" />
                  </span>
                </button>
              ))}
            </div>
            <UiField
              htmlFor={`${titleId}-alias`}
              label={t("agent_options.contact.alias")}
            >
              <UiInput
                controlSize="md"
                disabled={!selectedAgentId || isPending}
                id={`${titleId}-alias`}
                maxLength={128}
                onChange={(event) => setAlias(event.target.value)}
                placeholder={t("agent_options.contact.alias_placeholder")}
                value={alias}
                variant="dialog"
              />
            </UiField>
          </UiDialogBody>
          <UiDialogFooter appearance="plain">
            <UiButton onClick={onClose} type="button" variant="ghost">
              {t("common.cancel")}
            </UiButton>
            <UiButton disabled={!selectedAgentId || isPending} tone="primary" type="submit">
              {isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
              {t("agent_options.contact.add_friend")}
            </UiButton>
          </UiDialogFooter>
        </UiDialogFormShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function EmptyState({
  icon: Icon,
  label,
  spin = false,
}: {
  icon: typeof MessageCircle;
  label: string;
  spin?: boolean;
}) {
  return (
    <div className="flex h-full min-h-44 flex-col items-center justify-center gap-2 px-5 text-center text-(--text-soft)">
      <Icon className={cn("h-5 w-5", spin && "animate-spin")} />
      <p className="text-compact font-semibold">{label}</p>
    </div>
  );
}

function filterContacts(contacts: AgentContact[], query: string): AgentContact[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  return contacts
    .filter((contact) => !normalizedQuery || [
      contact.alias,
      contact.display_name,
      contact.name,
    ].some((value) => value?.toLocaleLowerCase().includes(normalizedQuery)))
    .sort((left, right) => contactLabel(left).localeCompare(contactLabel(right)));
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

function contactLabel(contact: AgentContact): string {
  return contact.alias?.trim() || contact.display_name?.trim() || contact.name;
}

function agentName(agent: Agent): string {
  return agent.display_name?.trim() || agent.name;
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
