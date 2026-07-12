import { useCallback, useEffect, useMemo } from "react";

import { useConversationSession } from "@/features/conversation/shared/session/use-conversation-session";
import { useConversationTodos } from "@/features/conversation/shared/todos/use-conversation-todos";
import {
  buildConversationActivityPatch,
  useConversationSnapshotReporter,
  type ConversationSnapshotBuildInput,
} from "@/features/conversation/shared/use-conversation-snapshot-reporter";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { RoomConversationSnapshotPayload } from "@/types/conversation/conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { TodoItem } from "@/types/conversation/todo";

interface UseGroupChatSessionControllerOptions {
  agentId: string | null;
  conversationId: string | null;
  onConversationSnapshotChange?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  onGoalEvent: () => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  roomId: string | null;
  sessionKey: string | null;
}

export function useGroupChatSessionController({
  agentId,
  conversationId,
  onConversationSnapshotChange,
  onGoalEvent,
  onRoomEvent,
  onTodosChange,
  roomId,
  sessionKey,
}: UseGroupChatSessionControllerOptions) {
  const identity = useMemo<AgentConversationIdentity | null>(() => {
    if (!conversationId || !sessionKey) {
      return null;
    }
    return {
      agent_id: agentId,
      chat_type: "group",
      conversation_id: conversationId,
      room_id: roomId,
      session_key: sessionKey,
    };
  }, [agentId, conversationId, roomId, sessionKey]);
  const handleRoomEvent = useCallback(
    (eventType: string, data: RoomEventPayload) => {
      if (eventType.startsWith("goal_")) {
        onGoalEvent();
      }
      onRoomEvent?.(eventType, data);
    },
    [onGoalEvent, onRoomEvent],
  );
  const session = useConversationSession({
    chatType: "group",
    debugName: "GroupChatPanel",
    identity,
    onRoomEvent: handleRoomEvent,
  });

  useGroupConversationObservers({
    conversationId,
    messages: session.conversation.messages,
    onConversationSnapshotChange,
    onTodosChange,
    sessionKey: session.sessionKey,
  });

  return session;
}

function useGroupConversationObservers({
  conversationId,
  messages,
  onConversationSnapshotChange,
  onTodosChange,
  sessionKey,
}: {
  conversationId: string | null;
  messages: Message[];
  onConversationSnapshotChange?: (
    snapshot: RoomConversationSnapshotPayload,
  ) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  sessionKey: string | null;
}): void {
  const todos = useConversationTodos(messages, sessionKey);
  useEffect(() => onTodosChange?.(todos), [onTodosChange, todos]);
  useConversationSnapshotReporter({
    build_snapshot: buildRoomSnapshot,
    messages,
    on_snapshot_change: onConversationSnapshotChange,
    scope_key: conversationId,
  });
}

function buildRoomSnapshot(
  input: ConversationSnapshotBuildInput,
): RoomConversationSnapshotPayload {
  return {
    ...buildConversationActivityPatch(input),
    conversation_id: input.scope_key,
    session_id: input.last_message.session_id ?? null,
  };
}
