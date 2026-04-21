/**
 * Heartbeat 自动化 API 封装
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import { to_timestamp_or_null } from "@/lib/api/timestamp-utils";
import type {
  ApiHeartbeatStatus,
  ApiHeartbeatWakeResult,
  HeartbeatConfig,
  HeartbeatUpdateInput,
  HeartbeatWakeResult,
  WakeHeartbeatRequest,
} from "@/types/capability/heartbeat";

const AGENT_API_BASE_URL = get_agent_api_base_url();
const HEARTBEAT_API_BASE_URL = `${AGENT_API_BASE_URL}/automation/heartbeat`;

function transform_heartbeat_config(
  api_config: ApiHeartbeatStatus,
): HeartbeatConfig {
  return {
    ...api_config,
    next_run_at: to_timestamp_or_null(api_config.next_run_at),
    last_heartbeat_at: to_timestamp_or_null(api_config.last_heartbeat_at),
    last_ack_at: to_timestamp_or_null(api_config.last_ack_at),
  };
}

export async function get_heartbeat_config_api(
  agent_id: string,
): Promise<HeartbeatConfig> {
  const result = await request_api<ApiHeartbeatStatus>(
    `${HEARTBEAT_API_BASE_URL}/${encodeURIComponent(agent_id)}`,
    {
      method: "GET",
    },
  );

  return transform_heartbeat_config(result);
}

export async function update_heartbeat_api(
  agent_id: string,
  payload: HeartbeatUpdateInput,
): Promise<HeartbeatConfig> {
  const result = await request_api<ApiHeartbeatStatus>(
    `${HEARTBEAT_API_BASE_URL}/${encodeURIComponent(agent_id)}`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    },
  );

  return transform_heartbeat_config(result);
}

export async function wake_heartbeat_api(
  agent_id: string,
  params: WakeHeartbeatRequest = {},
): Promise<HeartbeatWakeResult> {
  const result = await request_api<ApiHeartbeatWakeResult>(
    `${HEARTBEAT_API_BASE_URL}/${encodeURIComponent(agent_id)}/wake`,
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
