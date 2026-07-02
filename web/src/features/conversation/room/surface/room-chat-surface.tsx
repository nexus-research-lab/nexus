"use client";

import { DmChatPanel } from "@/features/conversation/room/dm/dm-chat-panel";
import { GroupChatPanel } from "@/features/conversation/room/group/chat/group-chat-panel";
import { GroupChatErrorBoundary } from "@/features/conversation/room/group/chat/group-chat-error-boundary";
import type { Agent } from "@/types/agent/agent";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { ConversationSnapshotPayload } from "@/types/conversation/conversation";
import type { TodoItem } from "@/types/conversation/todo";

interface RoomChatSurfaceProps {
  current_agent: Agent;
  current_room_type: string;
  current_agent_session_identity: AgentConversationIdentity | null;
  conversation_id: string | null;
  initial_draft?: string | null;
  on_initial_draft_consumed?: () => void;
  on_conversation_snapshot_change: (snapshot: ConversationSnapshotPayload) => void;
  on_create_conversation: (title?: string) => Promise<string | null>;
  on_loading_change: (isLoading: boolean) => void;
  on_open_agent_contact: (agentId: string) => void;
  on_open_workspace_file: (path: string) => void;
  on_room_event?: (eventType: string, data: RoomEventPayload) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled: boolean;
  room_id: string | null;
  room_members: Agent[];
}

export function RoomChatSurface({
  current_agent: currentAgent,
  current_room_type: currentRoomType,
  current_agent_session_identity: currentAgentSessionIdentity,
  conversation_id: conversationId,
  initial_draft: initialDraft,
  on_initial_draft_consumed: onInitialDraftConsumed,
  on_conversation_snapshot_change: onConversationSnapshotChange,
  on_create_conversation: onCreateConversation,
  on_loading_change: onLoadingChange,
  on_open_agent_contact: onOpenAgentContact,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_room_event: onRoomEvent,
  on_todos_change: onTodosChange,
  room_host_agent_id: roomHostAgentId,
  room_host_auto_reply_enabled: roomHostAutoReplyEnabled,
  room_id: roomId,
  room_members: roomMembers,
}: RoomChatSurfaceProps) {
  const isDm = currentRoomType === "dm";

  return (
    <GroupChatErrorBoundary>
      {isDm ? (
        <DmChatPanel
          current_agent_name={currentAgent.name}
          current_agent_avatar={currentAgent.avatar ?? null}
          current_agent_permission_mode={currentAgent.options.permission_mode ?? null}
          initial_draft={initialDraft}
          on_initial_draft_consumed={onInitialDraftConsumed}
          on_conversation_snapshot_change={onConversationSnapshotChange}
          on_loading_change={onLoadingChange}
          on_open_agent_contact={onOpenAgentContact}
          on_open_workspace_file={onOpenWorkspaceFile}
          on_room_event={onRoomEvent}
          on_todos_change={onTodosChange}
          session_identity={currentAgentSessionIdentity}
        />
      ) : (
        <GroupChatPanel
          agent_id={currentAgent.agent_id}
          conversation_id={conversationId}
          current_agent_name={currentAgent.name}
          current_agent_avatar={currentAgent.avatar ?? null}
          initial_draft={initialDraft}
          on_initial_draft_consumed={onInitialDraftConsumed}
          on_conversation_snapshot_change={onConversationSnapshotChange}
          on_create_conversation={onCreateConversation}
          on_loading_change={onLoadingChange}
          on_open_agent_contact={onOpenAgentContact}
          on_open_workspace_file={onOpenWorkspaceFile}
          on_room_event={onRoomEvent}
          on_todos_change={onTodosChange}
          room_host_agent_id={roomHostAgentId}
          room_host_auto_reply_enabled={roomHostAutoReplyEnabled}
          room_id={roomId}
          room_members={roomMembers}
        />
      )}
    </GroupChatErrorBoundary>
  );
}
