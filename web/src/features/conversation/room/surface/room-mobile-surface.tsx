"use client";

import { useMemo, useState } from "react";
import { ArrowLeft, Bot, Check, ChevronDown, MessageSquare, X } from "lucide-react";

import { formatRelativeTime, getIconAvatarSrc, getInitials } from "@/lib/utils";
import { Agent } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceTaskPanel } from "@/shared/ui/workspace/surface/workspace-task-strip";
import type { TodoItem } from "@/types/conversation/todo";

import { DmChatPanel } from "@/features/conversation/room/dm/dm-chat-panel";
import { SubagentTaskSurface } from "@/features/conversation/shared/subagent/subagent-task-surface";
import { GroupChatPanel } from "../group/chat/panel/group-chat-panel";
import { GroupThreadContextProvider } from "../group/thread/group-thread-context";
import { GroupThreadDetailPanel } from "../group/thread/group-thread-detail-panel";
import { useGroupThread } from "../group/thread/group-thread-state";
import { useRoomThreadPanel } from "../group/chat/use-room-thread-panel-data";

interface RoomMobileSurfaceProps {
  currentAgent: Agent;
  currentRoomType: string;
  roomId: string | null;
  roomMembers: Agent[];
  roomHostAgentId?: string | null;
  roomHostAutoReplyEnabled: boolean;
  currentRoomTitle: string;
  currentRoomConversation: RoomConversationView | null;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  currentRoomConversations: RoomConversationView[];
  currentTodos: TodoItem[];
  initialDraft?: string | null;
  onInitialDraftConsumed?: () => void;
  onBackToDirectory: () => void;
  onCreateConversation: (title?: string) => void | Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onLoadingChange: (isLoading: boolean) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onRoomEvent?: (eventType: string, data: import("@/types/agent/agent-conversation").RoomEventPayload) => void;
  onTodosChange: (todos: TodoItem[]) => void;
}

export function RoomMobileSurface({
  currentAgent: currentAgent,
  currentRoomType: currentRoomType,
  roomId: roomId,
  roomMembers: roomMembers,
  roomHostAgentId: roomHostAgentId,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled,
  currentRoomTitle: currentRoomTitle,
  currentRoomConversation: currentRoomConversation,
  currentAgentSessionIdentity: currentAgentSessionIdentity,
  conversationId: conversationId,
  currentRoomConversations: currentRoomConversations,
  currentTodos: currentTodos,
  initialDraft: initialDraft = null,
  onInitialDraftConsumed: onInitialDraftConsumed,
  onBackToDirectory: onBackToDirectory,
  onCreateConversation: onCreateConversation,
  onSelectConversation: onSelectConversation,
  onLoadingChange: onLoadingChange,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onConversationSnapshotChange: onConversationSnapshotChange,
  onRoomEvent: onRoomEvent,
  onTodosChange: onTodosChange,
}: RoomMobileSurfaceProps) {
  const { t } = useI18n();
  const [isConversationSheetOpen, setIsConversationSheetOpen] = useState(false);
  const [openSubagentSource, setOpenSubagentSource] = useState<SubagentTaskSource | null>(null);
  const isDm = currentRoomType === "dm";
  const currentAgentAvatarSrc = getIconAvatarSrc(currentAgent.avatar);
  const subagentTaskSource = useMemo<SubagentTaskSource | null>(() => {
    if (isDm) {
      const sessionKey = currentAgentSessionIdentity?.session_key?.trim();
      return sessionKey ? { kind: "session", session_key: sessionKey } : null;
    }
    if (!roomId || !conversationId) {
      return null;
    }
    return {
      kind: "room",
      room_id: roomId,
      conversation_id: conversationId,
    };
  }, [conversationId, currentAgentSessionIdentity?.session_key, isDm, roomId]);

  const currentRoomConversationTitle = useMemo(() => {
    if (currentRoomConversation?.title?.trim()) {
      return currentRoomConversation.title;
    }
    return "新会话";
  }, [currentRoomConversation]);

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background/90">
      <div className="px-2 pb-2 pt-2">
        <div className="surface-radius-lg flex items-center gap-2 px-2 py-2">
          <button
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-(--text-strong) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
            onClick={onBackToDirectory}
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>

          <button
            className="flex min-w-0 flex-1 items-center gap-3 rounded-[12px] border border-(--divider-subtle-color) px-3 py-2 text-left transition hover:bg-(--interaction-hover-background)"
            onClick={() => setIsConversationSheetOpen(true)}
            type="button"
          >
            <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[11px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
              {currentAgentAvatarSrc ? (
                <img
                  alt={currentAgent.name}
                  className="h-full w-full object-cover"
                  src={currentAgentAvatarSrc}
                />
              ) : (
                getInitials(currentAgent.name, "DM", 2)
              )}
            </div>

            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-semibold text-(--text-strong)">{currentAgent.name}</p>
              <p className="truncate text-[12px] text-(--text-muted)">
                {currentRoomTitle || currentRoomConversationTitle}
              </p>
            </div>

            <ChevronDown className="h-4 w-4 shrink-0 text-(--text-muted)" />
          </button>

          <button
            aria-label={t("subagents.open_panel")}
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl border border-(--divider-subtle-color) text-(--text-muted) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
            disabled={!subagentTaskSource}
            onClick={() => setOpenSubagentSource(subagentTaskSource)}
            title={t("subagents.open_panel")}
            type="button"
          >
            <Bot className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="relative min-h-0 min-w-0 flex-1 overflow-hidden">
        {isDm ? (
          <DmChatPanel
            currentAgentName={currentAgent.name}
            currentAgentAvatar={currentAgent.avatar ?? null}
            currentAgentPermissionMode={currentAgent.options.permission_mode ?? null}
            initialDraft={initialDraft}
            layout="mobile"
            onConversationSnapshotChange={onConversationSnapshotChange}
            onInitialDraftConsumed={onInitialDraftConsumed}
            onLoadingChange={onLoadingChange}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            onRoomEvent={onRoomEvent}
            onTodosChange={onTodosChange}
            sessionIdentity={currentAgentSessionIdentity}
          />
        ) : (
          <GroupThreadContextProvider>
            <GroupChatPanel
              agentId={currentAgent.agent_id}
              conversationId={conversationId}
              currentAgentName={currentAgent.name}
              currentAgentAvatar={currentAgent.avatar ?? null}
              initialDraft={initialDraft}
              layout="mobile"
              onConversationSnapshotChange={onConversationSnapshotChange}
              onCreateConversation={onCreateConversation}
              onInitialDraftConsumed={onInitialDraftConsumed}
              onLoadingChange={onLoadingChange}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              onRoomEvent={onRoomEvent}
              onTodosChange={onTodosChange}
              roomHostAgentId={roomHostAgentId}
              roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
              roomId={roomId}
              roomMembers={roomMembers}
            />
            <MobileThreadOverlay />
          </GroupThreadContextProvider>
        )}
        <WorkspaceTaskPanel key={conversationId ?? "mobile-conversation-tasks"} todos={currentTodos} />
      </div>

      {isConversationSheetOpen ? (
        <>
          <button
            aria-label="关闭会话列表"
            className="absolute inset-0 z-30 bg-(--dialog-backdrop-color)"
            onClick={() => setIsConversationSheetOpen(false)}
            type="button"
          />

          <div className="absolute inset-x-0 bottom-0 z-40 rounded-t-[28px] border-t border-(--surface-panel-border) bg-(--surface-panel-background) px-4 pb-6 pt-3 shadow-[0_-20px_40px_rgba(0,0,0,0.12)]">
            <div className="mx-auto mb-4 h-1.5 w-12 rounded-full bg-(--divider-strong-color)" />

            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold text-(--text-strong)">切换会话</p>
                <p className="text-xs text-(--text-muted)">
                  {currentRoomConversations.length} 个会话
                </p>
              </div>

              <button
                className="inline-flex h-9 w-9 items-center justify-center rounded-full text-(--text-muted) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
                onClick={() => setIsConversationSheetOpen(false)}
                type="button"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="max-h-[50vh] space-y-2 overflow-y-auto pr-1">
              {currentRoomConversations.map((conversation) => {
                const isActive = conversation.conversation_id === conversationId;
                return (
                  <button
                    key={conversation.conversation_id}
                    className="flex w-full items-start gap-3 rounded-2xl border border-(--divider-subtle-color) px-3 py-3 text-left transition hover:bg-(--interaction-hover-background)"
                    onClick={() => {
                      onSelectConversation(conversation.conversation_id);
                      setIsConversationSheetOpen(false);
                    }}
                    type="button"
                  >
                    <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color) text-(--text-strong)">
                      {isActive ? <Check className="h-4 w-4" /> : <MessageSquare className="h-4 w-4" />}
                    </div>

                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-(--text-strong)">
                        {conversation.title?.trim() || "未命名会话"}
                      </p>
                      <p className="mt-1 text-xs text-(--text-muted)">
                        {formatRelativeTime(conversation.last_activity_at)}
                      </p>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        </>
      ) : null}

      {openSubagentSource === subagentTaskSource && openSubagentSource ? (
        <div className="fixed inset-0 z-50 bg-(--surface-panel-background)">
          <SubagentTaskSurface
            layout="mobile"
            onClose={() => setOpenSubagentSource(null)}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            source={openSubagentSource}
          />
        </div>
      ) : null}
    </section>
  );
}

/** 移动端 Thread 全屏覆盖 — 在 GroupThreadContextProvider 内部使用 */
function MobileThreadOverlay() {
  const { activeThread, closeThread } = useGroupThread();
  const threadPanelData = useRoomThreadPanel();

  if (!activeThread || !threadPanelData) return null;

  return (
    <div className="fixed inset-0 z-50 bg-(--surface-panel-background)">
      <GroupThreadDetailPanel
        roundId={activeThread.roundId}
        agentId={activeThread.agentId}
        agentName={threadPanelData.agentName ?? activeThread.agentId}
        agentAvatar={threadPanelData.agentAvatar}
        userAvatar={threadPanelData.userAvatar}
        messages={threadPanelData.messages}
        pendingPermissions={threadPanelData.pendingPermissions}
        onPermissionResponse={threadPanelData.onPermissionResponse}
        onClose={closeThread}
        onStopMessage={threadPanelData.onStopMessage}
        onOpenWorkspaceFile={threadPanelData.onOpenWorkspaceFile}
        isLoading={threadPanelData.isLoading}
        layout="mobile"
      />
    </div>
  );
}
