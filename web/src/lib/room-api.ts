import { getAgentApiBaseUrl } from "@/config/options";
import { ApiResponse } from "@/types/api";
import {
  CreateProtocolRunParams,
  ProtocolChannelAggregate,
  ProtocolRunControlParams,
  ProtocolRunDetail,
  ProtocolRunListItem,
  RoomAggregate,
  RoomActionEnvelope,
  RoomArtifactRecord,
  RoomEventRecord,
  RoomMemberSpec,
  RoomMessageEnvelope,
  RoomRuntimeView,
  SubmitProtocolActionParams,
} from "@/types";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

async function extractApiError(response: Response, fallback_message: string): Promise<never> {
  try {
    const payload = await response.json();
    const detail = payload?.data?.detail || payload?.message || response.statusText;
    throw new Error(`${fallback_message}: ${detail}`);
  } catch (error) {
    if (error instanceof Error) {
      throw error;
    }
    throw new Error(`${fallback_message}: ${response.statusText}`);
  }
}

export async function getRoom(room_id: string): Promise<RoomAggregate> {
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    return await extractApiError(response, "读取 room 失败");
  }
  const result: ApiResponse<RoomAggregate> = await response.json();
  return result.data;
}

export async function listRooms(limit = 20): Promise<RoomAggregate[]> {
  const search_params = new URLSearchParams({ limit: String(limit) });
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms?${search_params.toString()}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    return await extractApiError(response, "读取 rooms 列表失败");
  }
  const result: ApiResponse<RoomAggregate[]> = await response.json();
  return result.data;
}

export async function createRoom(params: {
  mode?: "protocol" | "open";
  agent_ids?: string[];
  member_specs?: RoomMemberSpec[];
  name?: string;
  description?: string;
  title?: string;
  ruleset_slug?: string;
  goal?: string;
}): Promise<RoomRuntimeView> {
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      mode: params.mode ?? "open",
      agent_ids: params.agent_ids ?? [],
      member_specs: params.member_specs ?? [],
      name: params.name,
      description: params.description ?? "",
      title: params.title,
      ruleset_slug: params.ruleset_slug,
      goal: params.goal ?? "",
    }),
  });
  if (!response.ok) {
    return await extractApiError(response, "创建 room 失败");
  }
  const result: ApiResponse<RoomRuntimeView> = await response.json();
  return result.data;
}

export async function getRoomView(
  room_id: string,
  member_id?: string | null,
): Promise<RoomRuntimeView> {
  const suffix = member_id
    ? `/rooms/${encodeURIComponent(room_id)}/members/${encodeURIComponent(member_id)}/view`
    : `/rooms/${encodeURIComponent(room_id)}/view`;
  const response = await fetch(`${AGENT_API_BASE_URL}${suffix}`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    return await extractApiError(response, "读取 room 视图失败");
  }
  const result: ApiResponse<RoomRuntimeView> = await response.json();
  return result.data;
}

async function postRoomRuntimeCommand(
  room_id: string,
  command: "start" | "tick" | "run-phase" | "run-until-finished",
): Promise<RoomRuntimeView> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/${command}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "执行 room runtime 命令失败");
  }
  const result: ApiResponse<RoomRuntimeView> = await response.json();
  return result.data;
}

export function startRoom(room_id: string): Promise<RoomRuntimeView> {
  return postRoomRuntimeCommand(room_id, "start");
}

export function tickRoom(room_id: string): Promise<RoomRuntimeView> {
  return postRoomRuntimeCommand(room_id, "tick");
}

export function runRoomPhase(room_id: string): Promise<RoomRuntimeView> {
  return postRoomRuntimeCommand(room_id, "run-phase");
}

export function runRoomUntilFinished(room_id: string): Promise<RoomRuntimeView> {
  return postRoomRuntimeCommand(room_id, "run-until-finished");
}

export async function postRoomMessage(
  room_id: string,
  params: RoomMessageEnvelope,
): Promise<RoomRuntimeView> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/messages`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        scope: params.scope ?? "broadcast",
        content: params.content,
        sender_member_id: params.sender_member_id,
        target_member_ids: params.target_member_ids ?? [],
        metadata: params.metadata ?? {},
      }),
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "发送 room 消息失败");
  }
  const result: ApiResponse<RoomRuntimeView> = await response.json();
  return result.data;
}

export async function postRoomAction(
  room_id: string,
  params: RoomActionEnvelope,
): Promise<RoomRuntimeView> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/actions`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        action_type: params.action_type,
        actor_member_id: params.actor_member_id,
        target_member_id: params.target_member_id,
        payload: params.payload ?? {},
      }),
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "提交 room 动作失败");
  }
  const result: ApiResponse<RoomRuntimeView> = await response.json();
  return result.data;
}

export async function addRoomMember(
  room_id: string,
  params: RoomMemberSpec,
): Promise<RoomRuntimeView> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/members`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        existing_agent_id: params.existing_agent_id,
        create_spec: params.create_spec,
        role_hint: params.role_hint,
        source: params.source ?? "existing",
        workspace_binding: params.workspace_binding ?? true,
      }),
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "添加 room 成员失败");
  }
  const result: ApiResponse<RoomRuntimeView> = await response.json();
  return result.data;
}

export async function listRoomEvents(room_id: string): Promise<RoomEventRecord[]> {
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/events`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    return await extractApiError(response, "读取 room 时间线失败");
  }
  const result: ApiResponse<RoomEventRecord[]> = await response.json();
  return result.data;
}

export async function listRoomArtifacts(room_id: string): Promise<RoomArtifactRecord[]> {
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/artifacts`, {
    method: "GET",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    return await extractApiError(response, "读取 room 产物失败");
  }
  const result: ApiResponse<RoomArtifactRecord[]> = await response.json();
  return result.data;
}

export async function listRoomProtocolRuns(room_id: string): Promise<ProtocolRunListItem[]> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/protocol-runs`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "读取 protocol runs 失败");
  }
  const result: ApiResponse<ProtocolRunListItem[]> = await response.json();
  return result.data;
}

export async function createRoomProtocolRun(
  room_id: string,
  params: CreateProtocolRunParams,
): Promise<ProtocolRunDetail> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(room_id)}/protocol-runs`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        definition_slug: params.definition_slug ?? "werewolf_demo",
        title: params.title,
        run_config: params.run_config ?? {},
      }),
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "创建 protocol run 失败");
  }
  const result: ApiResponse<ProtocolRunDetail> = await response.json();
  return result.data;
}

export async function getProtocolRun(
  run_id: string,
  viewer_agent_id?: string | null,
): Promise<ProtocolRunDetail> {
  const search_params = new URLSearchParams();
  if (viewer_agent_id) {
    search_params.set("viewer_agent_id", viewer_agent_id);
  }
  const search_string = search_params.toString();
  const response = await fetch(
    `${AGENT_API_BASE_URL}/protocol-runs/${encodeURIComponent(run_id)}${search_string ? `?${search_string}` : ""}`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "读取 protocol run 失败");
  }
  const result: ApiResponse<ProtocolRunDetail> = await response.json();
  return result.data;
}

export async function listProtocolRunChannels(
  run_id: string,
  viewer_agent_id?: string | null,
): Promise<ProtocolChannelAggregate[]> {
  const search_params = new URLSearchParams();
  if (viewer_agent_id) {
    search_params.set("viewer_agent_id", viewer_agent_id);
  }
  const search_string = search_params.toString();
  const response = await fetch(
    `${AGENT_API_BASE_URL}/protocol-runs/${encodeURIComponent(run_id)}/channels${search_string ? `?${search_string}` : ""}`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "读取 protocol channels 失败");
  }
  const result: ApiResponse<ProtocolChannelAggregate[]> = await response.json();
  return result.data;
}

export async function submitProtocolAction(
  run_id: string,
  params: SubmitProtocolActionParams,
): Promise<ProtocolRunDetail> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/protocol-runs/${encodeURIComponent(run_id)}/actions`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(params),
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "提交 protocol action 失败");
  }
  const result: ApiResponse<ProtocolRunDetail> = await response.json();
  return result.data;
}

export async function controlProtocolRun(
  run_id: string,
  params: ProtocolRunControlParams,
): Promise<ProtocolRunDetail> {
  const response = await fetch(
    `${AGENT_API_BASE_URL}/protocol-runs/${encodeURIComponent(run_id)}/control`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        operation: params.operation,
        payload: params.payload ?? {},
      }),
    },
  );
  if (!response.ok) {
    return await extractApiError(response, "执行 protocol control 失败");
  }
  const result: ApiResponse<ProtocolRunDetail> = await response.json();
  return result.data;
}
