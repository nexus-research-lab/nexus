"use client";

import { DmChatPanel } from "@/features/conversation/room/dm/panel/dm-chat-panel";
import { GroupChatPanel } from "@/features/conversation/room/group/chat/panel/group-chat-panel";
import { getAgentConversationIdentityKey } from "@/lib/conversation/agent-conversation-identity";
import type { Agent } from "@/types/agent/agent";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { ConversationSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";

import { RoomChatErrorBoundary } from "./room-chat-error-boundary";

interface RoomChatSurfaceProps {
  currentAgent: Agent;
  currentRoomType: string;
  currentAgentSessionIdentity: AgentConversationIdentity | null;
  conversationId: string | null;
  initialDraft?: string | null;
  layout: "desktop" | "mobile";
  onInitialDraftConsumed?: () => void;
  onConversationSnapshotChange: (snapshot: ConversationSnapshotPayload) => void;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onOpenAgentContact?: (agentId: string) => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onTodosChange: (todos: TodoItem[]) => void;
  roomHostAgentId: string | null;
  roomHostAutoReplyEnabled: boolean;
  roomId: string | null;
  roomMembers: Agent[];
}

export function RoomChatSurface({
  currentAgent: currentAgent,
  currentRoomType: currentRoomType,
  currentAgentSessionIdentity: currentAgentSessionIdentity,
  conversationId: conversationId,
  initialDraft: initialDraft,
  layout: layout,
  onInitialDraftConsumed: onInitialDraftConsumed,
  onConversationSnapshotChange: onConversationSnapshotChange,
  onCreateConversation: onCreateConversation,
  onOpenAgentContact: onOpenAgentContact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  onRoomEvent: onRoomEvent,
  onTodosChange: onTodosChange,
  roomHostAgentId: roomHostAgentId,
  roomHostAutoReplyEnabled: roomHostAutoReplyEnabled,
  roomId: roomId,
  roomMembers: roomMembers,
}: RoomChatSurfaceProps) {
  const isDm = currentRoomType === "dm";
  const identityKey = getAgentConversationIdentityKey(currentAgentSessionIdentity)
    ?? `${roomId ?? "room"}:${conversationId ?? "conversation"}`;

  return (
    <RoomChatErrorBoundary resetKey={`${currentRoomType}:${identityKey}`}>
      {isDm ? (
        <DmChatPanel
          currentAgentName={currentAgent.name}
          currentAgentAvatar={currentAgent.avatar ?? null}
          currentAgentPermissionMode={currentAgent.options.permission_mode ?? null}
          initialDraft={initialDraft}
          layout={layout}
          onInitialDraftConsumed={onInitialDraftConsumed}
          onConversationSnapshotChange={onConversationSnapshotChange}
          onOpenAgentContact={onOpenAgentContact}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          onRoomEvent={onRoomEvent}
          onTodosChange={onTodosChange}
          sessionIdentity={currentAgentSessionIdentity}
        />
      ) : (
        <GroupChatPanel
          agentId={currentAgent.agent_id}
          conversationId={conversationId}
          currentAgentName={currentAgent.name}
          currentAgentAvatar={currentAgent.avatar ?? null}
          initialDraft={initialDraft}
          layout={layout}
          onInitialDraftConsumed={onInitialDraftConsumed}
          onConversationSnapshotChange={onConversationSnapshotChange}
          onCreateConversation={onCreateConversation}
          onOpenAgentContact={onOpenAgentContact}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          onRoomEvent={onRoomEvent}
          onTodosChange={onTodosChange}
          roomHostAgentId={roomHostAgentId}
          roomHostAutoReplyEnabled={roomHostAutoReplyEnabled}
          roomId={roomId}
          roomMembers={roomMembers}
        />
      )}
    </RoomChatErrorBoundary>
  );
}
