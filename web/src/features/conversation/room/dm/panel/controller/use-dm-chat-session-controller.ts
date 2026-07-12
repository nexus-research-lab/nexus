import { useCallback, useEffect } from "react";

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
import type { SessionSnapshotPayload } from "@/types/conversation/conversation";
import type { Message } from "@/types/conversation/message/entity";
import type { TodoItem } from "@/types/conversation/todo";

interface UseDmChatSessionControllerOptions {
  identity: AgentConversationIdentity | null;
  onConversationSnapshotChange?: (snapshot: SessionSnapshotPayload) => void;
  onGoalEvent: () => void;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
}

export function useDmChatSessionController({
  identity,
  onConversationSnapshotChange,
  onGoalEvent,
  onRoomEvent,
  onTodosChange,
}: UseDmChatSessionControllerOptions) {
  const handleRoomEvent = useCallback(
    (eventType: string, data: RoomEventPayload): void => {
      if (eventType.startsWith("goal_")) {
        onGoalEvent();
      }
      onRoomEvent?.(eventType, data);
    },
    [onGoalEvent, onRoomEvent],
  );
  const session = useConversationSession({
    chatType: "dm",
    debugName: "DmChatPanel",
    identity,
    onRoomEvent: handleRoomEvent,
  });

  useDmConversationObservers({
    identity,
    messages: session.conversation.messages,
    onConversationSnapshotChange,
    onTodosChange,
    sessionKey: session.sessionKey,
  });

  return session;
}

function useDmConversationObservers({
  identity,
  messages,
  onConversationSnapshotChange,
  onTodosChange,
  sessionKey,
}: {
  identity: AgentConversationIdentity | null;
  messages: Message[];
  onConversationSnapshotChange?: (snapshot: SessionSnapshotPayload) => void;
  onTodosChange?: (todos: TodoItem[]) => void;
  sessionKey: string | null;
}): void {
  const todos = useConversationTodos(messages, sessionKey);
  useEffect(() => onTodosChange?.(todos), [onTodosChange, todos]);
  const buildSnapshot = useCallback(
    (input: ConversationSnapshotBuildInput) => buildDmSnapshot(input, identity),
    [identity],
  );
  useConversationSnapshotReporter({
    build_snapshot: buildSnapshot,
    messages,
    on_snapshot_change: onConversationSnapshotChange,
    scope_key: sessionKey,
  });
}

function buildDmSnapshot(
  input: ConversationSnapshotBuildInput,
  identity: AgentConversationIdentity | null,
): SessionSnapshotPayload {
  const snapshotIdentity = projectDmSnapshotIdentity(identity);
  return {
    ...snapshotIdentity,
    ...buildConversationActivityPatch(input),
    session_id: input.last_message.session_id ?? null,
    session_key: input.scope_key,
  };
}

type DmSnapshotIdentity = Pick<
  SessionSnapshotPayload,
  "agent_id" | "conversation_id" | "room_id" | "room_session_id"
>;

const EMPTY_DM_SNAPSHOT_IDENTITY: DmSnapshotIdentity = {
  agent_id: null,
  conversation_id: null,
  room_id: null,
  room_session_id: null,
};

function projectDmSnapshotIdentity(
  identity: AgentConversationIdentity | null,
): DmSnapshotIdentity {
  if (!identity) {
    return EMPTY_DM_SNAPSHOT_IDENTITY;
  }
  return {
    agent_id: identity.agent_id ?? null,
    conversation_id: identity.conversation_id ?? null,
    room_id: identity.room_id ?? null,
    room_session_id: identity.room_session_id ?? null,
  };
}
