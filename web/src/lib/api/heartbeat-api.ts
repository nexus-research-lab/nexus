/**
 * Heartbeat 自动化 API 封装
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
import { toTimestampOrNull } from "@/lib/api/timestamp-utils";
import type {
  ApiHeartbeatStatus,
  ApiHeartbeatWakeResult,
  HeartbeatConfig,
  HeartbeatUpdateInput,
  HeartbeatWakeResult,
  WakeHeartbeatRequest,
} from "@/types/capability/heartbeat";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();
const HEARTBEAT_API_BASE_URL = `${AGENT_API_BASE_URL}/automation/heartbeat`;

function transformHeartbeatConfig(
  apiConfig: ApiHeartbeatStatus,
): HeartbeatConfig {
  return {
    ...apiConfig,
    next_run_at: toTimestampOrNull(apiConfig.next_run_at),
    last_heartbeat_at: toTimestampOrNull(apiConfig.last_heartbeat_at),
    last_ack_at: toTimestampOrNull(apiConfig.last_ack_at),
  };
}

export async function getHeartbeatConfigApi(
  agentId: string,
): Promise<HeartbeatConfig> {
  const result = await requestApi<ApiHeartbeatStatus>(
    `${HEARTBEAT_API_BASE_URL}/${encodeURIComponent(agentId)}`,
    {
      method: "GET",
    },
  );

  return transformHeartbeatConfig(result);
}

export async function updateHeartbeatApi(
  agentId: string,
  payload: HeartbeatUpdateInput,
): Promise<HeartbeatConfig> {
  const result = await requestApi<ApiHeartbeatStatus>(
    `${HEARTBEAT_API_BASE_URL}/${encodeURIComponent(agentId)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );

  return transformHeartbeatConfig(result);
}

export async function wakeHeartbeatApi(
  agentId: string,
  params: WakeHeartbeatRequest = {},
): Promise<HeartbeatWakeResult> {
  const result = await requestApi<ApiHeartbeatWakeResult>(
    `${HEARTBEAT_API_BASE_URL}/${encodeURIComponent(agentId)}/wake`,
    {
      method: "POST",
      body: JSON.stringify({
        mode: params.mode ?? "now",
        text: params.text,
      }),
    },
  );

  return result;
}
