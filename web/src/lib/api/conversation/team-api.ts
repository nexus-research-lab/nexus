/** Nexus Team gateway 的 M1 真人消息、同步与 stream 换代协议。 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

const TEAM_API_BASE_URL = `${getAgentApiBaseUrl()}/team`;

export interface TeamMessageContent {
  version: 1;
  blocks: Array<{ type: "markdown"; text: string }>;
}

export interface TeamMessage {
  id: string;
  conversation_id: string;
  message_seq: number;
  author_type: "user";
  author_user_id: string;
  author_username: string;
  author_display_name: string;
  client_message_id: string;
  content: TeamMessageContent;
  created_at: string;
}

export interface TeamBootstrap {
  team: { id: string; deployment_id: string; name: string };
  room: { id: string; team_id: string; name: string };
  conversation: {
    id: string;
    room_id: string;
    type: string;
    high_water_message_seq: number;
    sync_stream_id: string;
    stream_epoch: string;
    high_water_sync_event_seq: number;
  };
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

export function bootstrapTeam(signal?: AbortSignal): Promise<TeamBootstrap> {
  return requestApi<TeamBootstrap>(`${TEAM_API_BASE_URL}/bootstrap`, {
    method: "POST",
    signal,
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
): Promise<TeamMessageCommit> {
  return requestApi<TeamMessageCommit>(
    `${TEAM_API_BASE_URL}/conversations/${encodeURIComponent(conversationId)}/messages`,
    {
      body: {
        content: { version: 1, blocks: [{ type: "markdown", text }] },
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
