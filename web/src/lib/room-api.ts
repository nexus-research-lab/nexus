import { getAgentApiBaseUrl } from "@/config/options";
import { ApiResponse } from "@/types/api";
import {
  CreateProtocolRunParams,
  ProtocolChannelAggregate,
  ProtocolRunControlParams,
  ProtocolRunDetail,
  ProtocolRunListItem,
  RoomAggregate,
  RoomConversationContext,
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

export async function createRoom(params: {
  agent_ids: string[];
  name?: string;
  description?: string;
  title?: string;
}): Promise<RoomConversationContext> {
  const response = await fetch(`${AGENT_API_BASE_URL}/rooms`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      agent_ids: params.agent_ids,
      name: params.name,
      description: params.description ?? "",
      title: params.title,
    }),
  });
  if (!response.ok) {
    return await extractApiError(response, "创建 room 失败");
  }
  const result: ApiResponse<RoomConversationContext> = await response.json();
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
