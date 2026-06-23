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
  on_loading_change: (is_loading: boolean) => void;
  on_open_agent_contact: (agent_id: string) => void;
  on_open_workspace_file: (path: string) => void;
  on_room_event?: (event_type: string, data: RoomEventPayload) => void;
  on_todos_change: (todos: TodoItem[]) => void;
  room_host_agent_id?: string | null;
  room_host_auto_reply_enabled: boolean;
  room_id: string | null;
  room_members: Agent[];
}

export function RoomChatSurface({
  current_agent,
  current_room_type,
  current_agent_session_identity,
  conversation_id,
  initial_draft,
  on_initial_draft_consumed,
  on_conversation_snapshot_change,
  on_create_conversation,
  on_loading_change,
  on_open_agent_contact,
  on_open_workspace_file,
  on_room_event,
  on_todos_change,
  room_host_agent_id,
  room_host_auto_reply_enabled,
  room_id,
  room_members,
}: RoomChatSurfaceProps) {
  const is_dm = current_room_type === "dm";

  return (
    <GroupChatErrorBoundary>
      {is_dm ? (
        <DmChatPanel
          current_agent_name={current_agent.name}
          current_agent_avatar={current_agent.avatar ?? null}
          current_agent_permission_mode={current_agent.options.permission_mode ?? null}
          initial_draft={initial_draft}
          on_initial_draft_consumed={on_initial_draft_consumed}
          on_conversation_snapshot_change={on_conversation_snapshot_change}
          on_loading_change={on_loading_change}
          on_open_agent_contact={on_open_agent_contact}
          on_open_workspace_file={on_open_workspace_file}
          on_room_event={on_room_event}
          on_todos_change={on_todos_change}
          session_identity={current_agent_session_identity}
        />
      ) : (
        <GroupChatPanel
          agent_id={current_agent.agent_id}
          conversation_id={conversation_id}
          current_agent_name={current_agent.name}
          current_agent_avatar={current_agent.avatar ?? null}
          initial_draft={initial_draft}
          on_initial_draft_consumed={on_initial_draft_consumed}
          on_conversation_snapshot_change={on_conversation_snapshot_change}
          on_create_conversation={on_create_conversation}
          on_loading_change={on_loading_change}
          on_open_agent_contact={on_open_agent_contact}
          on_open_workspace_file={on_open_workspace_file}
          on_room_event={on_room_event}
          on_todos_change={on_todos_change}
          room_host_agent_id={room_host_agent_id}
          room_host_auto_reply_enabled={room_host_auto_reply_enabled}
          room_id={room_id}
          room_members={room_members}
        />
      )}
    </GroupChatErrorBoundary>
  );
}
