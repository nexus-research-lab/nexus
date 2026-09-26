/** Nexus Team gateway 的 M1 真人消息、同步与 stream 换代协议。 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { ResultSummary } from "@/types/conversation/message/entity";

const TEAM_API_BASE_URL = `${getAgentApiBaseUrl()}/team`;

export function cancelTeamDelivery(roomId: string, deliveryId: string) {
  return requestApi(`${TEAM_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/deliveries/${encodeURIComponent(deliveryId)}/cancel`, {method: "POST"});
}

export function getTeamCommands(signal: AbortSignal) {
  return requestApi<import("@/types/generated/protocol").CommandCatalogData>(`${TEAM_API_BASE_URL}/commands`, {signal});
}

export interface TeamMessageContent {
  attachments?: Array<{id: string; name: string; size: number; sha256: string}>;
  version: 1;
  execution?: {
    model?: string;
    result_summary?: Pick<ResultSummary, "duration_ms" | "duration_api_ms" | "num_turns" | "total_cost_usd" | "usage">;
  };
  blocks: Array<{ type: "markdown" | "room_invitation"; text: string; room_id?: string; invitee_user_id?: string; invited_at?: string }>;
}

export interface TeamMessage {
  id: string;
  conversation_id: string;
  message_seq: number;
  author_type: "user" | "agent";
  author_user_id: string;
  author_agent_id?: string;
  delivery_id?: string;
  output_kind?: "assistant" | "final";
  author_username: string;
  author_display_name: string;
  client_message_id: string;
  content: TeamMessageContent;
  mentions?: Array<{ member_type: "agent"; member_id: string }>;
  created_at: string;
}

export interface TeamRoomView {
  last_read_message_seq?: number;
  unread_count?: number;
  room: {
    id: string;
    organization_id: string;
    direct_user_id?: string;
    team_id?: string;
    name: string;
    description: string;
    avatar: string;
    coordinator_agent_id?: string;
    host_auto_reply_enabled: boolean;
    private_messages_enabled: boolean;
    skill_names: string[];
    configuration_version: number;
    membership_version: number;
    created_at: string;
    updated_at: string;
  };
  conversation: {
    id: string;
    room_id: string;
    type: string;
    high_water_message_seq: number;
    last_activity_at: string | null;
    sync_stream_id: string;
    stream_epoch: string;
    high_water_sync_event_seq: number;
  };
  current_user_role: "owner" | "admin" | "member";
}

export interface TeamRoomList {
  rooms: TeamRoomView[];
}

export interface TeamRoomMember {
  room_id: string;
  member_type: "user" | "agent";
  member_id: string;
  role: "owner" | "admin" | "member";
  state: "invited" | "active" | "left" | "removed";
  agent_owner_user_id?: string;
  agent_paused?: boolean;
  invited_by_user_id: string;
  joined_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface TeamRoomDetails extends TeamRoomView {
  next_member_cursor?: string;
  members: TeamRoomMember[];
  deliveries?: TeamDeliveryStatus[];
}

export interface TeamDeliveryStatus {
  execution_state?: "running" | "waiting_input";
  id: string;
  message_id: string;
  agent_id: string;
  state: "pending" | "leased" | "completed" | "failed" | "cancelled";
  failure_code?: string;
}

export interface TeamRoomInvitation {
  room: TeamRoomView["room"];
  invited_by_user_id: string;
  created_at: string;
}

export interface TeamRoomInvitationList {
  invitations: TeamRoomInvitation[];
  recovery_rooms: TeamRoomRecovery[];
}

export interface TeamRoomRecovery { id: string; name: string; membership_version: number }

export interface TeamRoomMembershipMutation {
  room_id: string;
  membership_version: number;
  replayed: boolean;
}

export interface TeamRoomConfigurationMutation {
  room_id: string;
  configuration_version: number;
  replayed: boolean;
}

export interface TeamSnapshot {
  conversation_id: string;
  stream_id: string;
  stream_epoch: string;
  snapshot_seq: number;
  through_message_seq: number;
  after_message_seq: number;
  next_message_seq: number;
  has_more: boolean;
  messages: TeamMessage[];
}

export interface TeamDifference {
  stream_id: string;
  stream_epoch: string;
  after_seq: number;
  next_seq: number;
  high_water_seq: number;
  min_retained_seq: number;
  has_more: boolean;
  events: Array<{ event_seq: number; type: string; message: TeamMessage }>;
}

export interface TeamMessageCommit {
  message: TeamMessage;
  stream_id: string;
  stream_epoch: string;
  event_seq: number;
  high_water_seq: number;
  replayed: boolean;
}

export interface TeamStreamUpdated {
  type: "stream.updated";
  stream_id: string;
  stream_epoch: string;
  high_water_seq: number;
}

export interface TeamStreamResetRequired {
  type: "stream.reset_required";
  stream_id: string;
  reason: "full_snapshot_required";
}

export function listTeamRooms(signal?: AbortSignal): Promise<TeamRoomList> {
  return requestApi<TeamRoomList>(`${TEAM_API_BASE_URL}/rooms`, { method: "GET", signal });
}

export function createTeamRoom(
  input: {
    direct_user_id?: string;
		agent_ids: string[];
		avatar?: string;
		coordinator_agent_id?: string;
		host_auto_reply_enabled?: boolean;
    member_user_ids: string[];
    name: string;
    private_messages_enabled: boolean;
    skill_names: string[];
  },
  commandId: string,
): Promise<TeamRoomView> {
  return requestApi<TeamRoomView>(`${TEAM_API_BASE_URL}/rooms`, {
    body: input,
    headers: { "Idempotency-Key": commandId },
    method: "POST",
  });
}

export async function getTeamRoom(roomId: string, signal?: AbortSignal): Promise<TeamRoomDetails> {
  const base = `${TEAM_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}`;
  const result = await requestApi<TeamRoomDetails>(base, {method: "GET", signal});
  // ponytail: 当前管理与 mention 界面需要完整名单；大群按需检索时再取消客户端汇总。
  while (result.next_member_cursor) {
    const query = new URLSearchParams({after: result.next_member_cursor, membership_version: String(result.room.membership_version), stream_epoch: result.conversation.stream_epoch});
    const page = await requestApi<{members: TeamRoomMember[]; next_cursor?: string}>(`${base}/members?${query}`, {signal});
    if (page.next_cursor === result.next_member_cursor) throw new Error("成员分页游标未推进");
    result.members.push(...page.members);
    result.next_member_cursor = page.next_cursor;
  }
  const roles = {owner: 0, admin: 1, member: 2};
  const states = {active: 0, invited: 1, left: 2, removed: 2};
  result.members.sort((a, b) => roles[a.role] - roles[b.role] || states[a.state] - states[b.state] || a.member_type.localeCompare(b.member_type) || a.member_id.localeCompare(b.member_id));
  return result;
}

export function listTeamInvitations(signal?: AbortSignal): Promise<TeamRoomInvitationList> {
  return requestApi<TeamRoomInvitationList>(`${TEAM_API_BASE_URL}/invitations`, { method: "GET", signal });
}

export function inviteTeamRoomMember(roomId: string, userId: string, version: number, commandId: string): Promise<TeamRoomMembershipMutation> {
  return teamMembershipMutation(roomId, "/invitations", "POST", { user_id: userId, expected_membership_version: version }, commandId);
}

export function addTeamRoomAgent(roomId: string, agentId: string, version: number, commandId: string): Promise<TeamRoomMembershipMutation> {
	return teamMembershipMutation(roomId, "/agents", "POST", { agent_id: agentId, expected_membership_version: version }, commandId);
}

export function removeTeamRoomAgent(roomId: string, agentId: string, version: number, commandId: string): Promise<TeamRoomMembershipMutation> {
	return teamMembershipMutation(roomId, `/agents/${encodeURIComponent(agentId)}`, "DELETE", { expected_membership_version: version }, commandId);
}

export function updateTeamRoomAgent(roomId: string, agentId: string, paused: boolean, version: number, commandId: string): Promise<TeamRoomMembershipMutation> {
  return teamMembershipMutation(roomId, `/agents/${encodeURIComponent(agentId)}`, "PATCH", { paused, expected_membership_version: version }, commandId);
}

export function updateTeamRoomCoordinator(roomId: string, agentId: string, version: number, commandId: string): Promise<TeamRoomConfigurationMutation> {
  return updateTeamRoomSettings(roomId, { coordinator_agent_id: agentId }, version, commandId);
}

export function updateTeamRoomSettings(roomId: string, change: { name?: string; avatar?: string; coordinator_agent_id?: string; host_auto_reply_enabled?: boolean; dissolve?: boolean; hide_direct?: boolean }, version: number, commandId: string): Promise<TeamRoomConfigurationMutation> {
  return requestApi<TeamRoomConfigurationMutation>(`${TEAM_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}`, {
    body: { ...change, expected_configuration_version: version },
    headers: { "Idempotency-Key": commandId },
    method: "PATCH",
  });
}

export function resolveTeamRoomInvitation(roomId: string, version: number, resolution: "accept" | "reject", commandId: string): Promise<TeamRoomMembershipMutation> {
  return teamMembershipMutation(roomId, `/invitations/${resolution}`, "POST", { expected_membership_version: version }, commandId);
}

export function revokeTeamRoomInvitation(roomId: string, userId: string, version: number, commandId: string): Promise<TeamRoomMembershipMutation> {
  return teamMembershipMutation(roomId, `/invitations/${encodeURIComponent(userId)}`, "DELETE", { expected_membership_version: version }, commandId);
}

export function updateTeamRoomMember(roomId: string, userId: string, version: number, change: { role: "admin" | "member" } | { remove: true }, commandId: string): Promise<TeamRoomMembershipMutation> {
  return teamMembershipMutation(roomId, `/members/${encodeURIComponent(userId)}`, "PATCH", { ...change, expected_membership_version: version }, commandId);
}

export function transferTeamRoomOwnership(roomId: string, userId: string, version: number, commandId: string, takeover = false): Promise<TeamRoomMembershipMutation> {
  return teamMembershipMutation(roomId, "/transfer", "POST", { new_owner_user_id: userId, expected_membership_version: version, ...(takeover ? {takeover: true} : {}) }, commandId);
}

function teamMembershipMutation(
  roomId: string,
  suffix: string,
  method: "DELETE" | "PATCH" | "POST",
  body: Record<string, unknown>,
  commandId: string,
): Promise<TeamRoomMembershipMutation> {
  return requestApi<TeamRoomMembershipMutation>(`${TEAM_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}${suffix}`, {
    body,
    headers: { "Idempotency-Key": commandId },
    method,
  });
}

export function getTeamSnapshot(
  conversationId: string,
  query: URLSearchParams,
  signal?: AbortSignal,
): Promise<TeamSnapshot> {
  return requestApi<TeamSnapshot>(
    `${TEAM_API_BASE_URL}/conversations/${encodeURIComponent(conversationId)}/snapshot?${query}`,
    { method: "GET", signal },
  );
}

export function getTeamDifference(
  streamId: string,
  afterSeq: number,
  streamEpoch: string,
  signal?: AbortSignal,
): Promise<TeamDifference> {
  const query = new URLSearchParams({
    after_seq: String(afterSeq),
    limit: "100",
    stream_epoch: streamEpoch,
  });
  return requestApi<TeamDifference>(
    `${TEAM_API_BASE_URL}/sync-streams/${encodeURIComponent(streamId)}/difference?${query}`,
    { method: "GET", signal },
  );
}

export function postTeamMessage(
  conversationId: string,
  text: string,
  clientMessageId: string,
	options?: { agentIds: string[]; expectedMembershipVersion: number },
  attachments: NonNullable<TeamMessageContent["attachments"]> = [],
): Promise<TeamMessageCommit> {
  return requestApi<TeamMessageCommit>(
    `${TEAM_API_BASE_URL}/conversations/${encodeURIComponent(conversationId)}/messages`,
    {
      body: {
        content: { version: 1, blocks: [{ type: "markdown", text }], ...(attachments.length ? {attachments} : {}) },
		mentions: options?.agentIds.map((memberId) => ({ member_type: "agent", member_id: memberId })) ?? [],
		expected_membership_version: options?.agentIds.length ? options.expectedMembershipVersion : undefined,
      },
      headers: { "Idempotency-Key": clientMessageId },
      method: "POST",
    },
  );
}

export function buildTeamStreamUrl(streamId: string, streamEpoch: string): string {
  const url = new URL(`${TEAM_API_BASE_URL}/stream`);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.searchParams.set("stream_id", streamId);
  url.searchParams.set("stream_epoch", streamEpoch);
  return url.toString();
}

// 阅读确认仅携带消息水位，真人身份由同源 Gateway 提供。
export function markTeamRoomRead(roomId: string, messageSeq: number, streamEpoch: string, signal?: AbortSignal): Promise<{last_read_message_seq: number}> {
  return requestApi(`${TEAM_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/read-state`, {
    method: "PUT", signal, body: JSON.stringify({message_seq: messageSeq, stream_epoch: streamEpoch}),
  });
}

/** 每批只查询至多 100 条已加载消息，避免 Room 刷新随全部历史增长。 */
export async function getTeamDeliveryStatuses(roomId: string, messageIds: string[], signal?: AbortSignal): Promise<TeamDeliveryStatus[]> {
  const result: TeamDeliveryStatus[] = [];
  const ids = [...new Set(messageIds)];
  for (let offset = 0; offset < ids.length; offset += 100) {
    const query = new URLSearchParams();
    for (const id of ids.slice(offset, offset + 100)) query.append("message_id", id);
    result.push(...await requestApi<TeamDeliveryStatus[]>(`${TEAM_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/deliveries?${query}`, {signal}));
  }
  return result;
}
