/**
 * =====================================================
 * @File   ：room-page-controller-core.ts
 * @Date   ：2026-04-08 11:42:07
 * @Author ：leemysw
 * 2026-04-08 11:42:07   Create
 * =====================================================
 */

import { build_room_agent_session_key, build_room_shared_session_key } from "@/lib/conversation/session-key";
import { Agent } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { Conversation, RoomConversationView } from "@/types/conversation/conversation";
import { RoomContextAggregate } from "@/types/conversation/room";

function to_timestamp(value?: string | null): number {
  if (!value) {
    return 0;
  }

  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : 0;
}

function get_context_last_activity_timestamp(context: RoomContextAggregate): number {
  const session_timestamps = context.sessions.map((session) => (
    to_timestamp(session.last_activity_at)
  ));
  const latest_session_timestamp = Math.max(0, ...session_timestamps);

  return latest_session_timestamp ||
    to_timestamp(context.conversation.updated_at) ||
    to_timestamp(context.conversation.created_at);
}

function get_room_conversation_session_key(
  context: RoomContextAggregate,
  latest_conversation: Conversation | undefined,
  fallback_session: RoomContextAggregate["sessions"][number] | undefined,
): string {
  if (context.room.room_type === "dm") {
    if (latest_conversation?.session_key) {
      return latest_conversation.session_key;
    }

    if (fallback_session?.agent_id) {
      return build_room_agent_session_key(
        context.conversation.id,
        fallback_session.agent_id,
        "dm",
      );
    }
  }

  return build_room_shared_session_key(context.conversation.id);
}

function build_fallback_room_member_agent(
  agent_id: string,
  room_contexts: RoomContextAggregate[],
): Agent {
  const primary_context = room_contexts[0] ?? null;
  const is_dm = primary_context?.room.room_type === "dm";
  const fallback_name = (
    is_dm
      ? primary_context?.room.name?.trim() ||
        primary_context?.conversation.title?.trim() ||
        agent_id
      : agent_id
  );

  return {
    agent_id,
    name: fallback_name,
    workspace_path: "",
    options: {},
    created_at: 0,
    status: "active",
    avatar: null,
    description: null,
    vibe_tags: [],
    skills_count: null,
  };
}

export function build_room_conversation_views(
  room_contexts: RoomContextAggregate[],
  conversations: Conversation[],
): RoomConversationView[] {
  return room_contexts
    .filter((context) => Boolean(context.conversation.id))
    .map((context) => {
      const session_conversations = conversations.filter(
        (conversation) =>
          conversation.room_id === context.room.id &&
          conversation.conversation_id === context.conversation.id,
      );
      const latest_conversation = [...session_conversations].sort(
        (left, right) => right.last_activity_at - left.last_activity_at,
      )[0];
      const context_last_activity_at = get_context_last_activity_timestamp(context);
      const fallback_session = [...context.sessions].sort((left, right) => {
        const left_timestamp = (
          to_timestamp(left.last_activity_at) ||
          to_timestamp(left.updated_at) ||
          to_timestamp(left.created_at)
        );
        const right_timestamp = (
          to_timestamp(right.last_activity_at) ||
          to_timestamp(right.updated_at) ||
          to_timestamp(right.created_at)
        );
        return right_timestamp - left_timestamp;
      })[0];

      return {
        session_key: get_room_conversation_session_key(
          context,
          latest_conversation,
          fallback_session,
        ),
        room_id: context.room.id,
        conversation_id: context.conversation.id,
        conversation_type: context.conversation.conversation_type,
        session_id: latest_conversation?.session_id ?? fallback_session?.sdk_session_id ?? null,
        agent_id: latest_conversation?.agent_id ?? fallback_session?.agent_id,
        title: context.conversation.title?.trim() || context.room.name || "未命名对话",
        options: latest_conversation?.options ?? {},
        created_at:
          latest_conversation?.created_at ??
          (to_timestamp(context.conversation.created_at) || context_last_activity_at),
        last_activity_at:
          latest_conversation?.last_activity_at ?? context_last_activity_at,
        is_active: latest_conversation?.is_active,
        message_count:
          latest_conversation?.message_count ??
          session_conversations.reduce(
            (count, conversation) => count + (conversation.message_count ?? 0),
            0,
          ),
      } satisfies RoomConversationView;
    })
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

export function resolve_selected_conversation_id(
  route_conversation_id: string | null | undefined,
  room_conversations: RoomConversationView[],
): string | null {
  if (
    route_conversation_id &&
    room_conversations.some((conversation) => conversation.conversation_id === route_conversation_id)
  ) {
    return route_conversation_id;
  }

  return room_conversations[0]?.conversation_id ?? null;
}

export function resolve_current_room_context(
  room_contexts: RoomContextAggregate[],
  selected_conversation_id: string | null,
): RoomContextAggregate | null {
  return room_contexts.find((context) => context.conversation.id === selected_conversation_id) ??
    room_contexts[0] ??
    null;
}

export function resolve_selected_member_agent_id(
  current_room_context: RoomContextAggregate | null,
  current_selected_member_agent_id: string | null,
): string | null {
  const member_agent_ids =
    current_room_context?.sessions
      .map((session) => session.agent_id)
      .filter(Boolean) ?? [];

  if (!member_agent_ids.length) {
    return null;
  }

  if (
    current_selected_member_agent_id &&
    member_agent_ids.includes(current_selected_member_agent_id)
  ) {
    return current_selected_member_agent_id;
  }

  return member_agent_ids[0];
}

export function resolve_current_agent_session_identity(params: {
  current_agent_conversation: Conversation | null;
  current_room_id: string | null;
  active_room_session: RoomContextAggregate["sessions"][number] | null;
  current_room_type: string;
}): AgentConversationIdentity | null {
  const {
    current_agent_conversation,
    current_room_id,
    active_room_session,
    current_room_type,
  } = params;

  const resolved_agent_id = current_agent_conversation?.agent_id ?? active_room_session?.agent_id ?? null;
  const resolved_conversation_id = (
    current_agent_conversation?.conversation_id ??
    active_room_session?.conversation_id ??
    null
  );
  const resolved_room_id = current_agent_conversation?.room_id ?? current_room_id ?? null;
  const resolved_room_session_id = current_agent_conversation?.room_session_id ?? active_room_session?.id ?? null;

  let resolved_session_key = current_agent_conversation?.session_key ?? null;
  if (!resolved_session_key && resolved_conversation_id) {
    resolved_session_key = (
      current_room_type === "dm" && resolved_agent_id
        ? build_room_agent_session_key(resolved_conversation_id, resolved_agent_id, "dm")
        : build_room_shared_session_key(resolved_conversation_id)
    );
  }

  if (!resolved_session_key) {
    return null;
  }

  return {
    session_key: resolved_session_key,
    agent_id: resolved_agent_id,
    room_id: resolved_room_id,
    conversation_id: resolved_conversation_id,
    room_session_id: resolved_room_session_id,
    chat_type: current_room_type === "dm" ? "dm" : "group",
  };
}

export function resolve_room_member_agents(
  agents: Agent[],
  room_contexts: RoomContextAggregate[],
): Agent[] {
  const member_agent_ids =
    room_contexts[0]?.members
      .filter((member) => member.member_type === "agent")
      .map((member) => member.member_agent_id)
      .filter((member_agent_id): member_agent_id is string => Boolean(member_agent_id)) ?? [];

  return member_agent_ids.map((agent_id) => (
    agents.find((agent) => agent.agent_id === agent_id) ??
    build_fallback_room_member_agent(agent_id, room_contexts)
  ));
}
