/**
 * Heartbeat 自动化 API 封装
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type {
  ApiHeartbeatStatus,
  ApiHeartbeatWakeResult,
  HeartbeatConfig,
  HeartbeatWakeResult,
  WakeHeartbeatRequest,
} from "@/types/capability/heartbeat";

const AGENT_API_BASE_URL = get_agent_api_base_url();
const HEARTBEAT_API_BASE_URL = `${AGENT_API_BASE_URL}/automation/heartbeat`;

function to_timestamp(value?: string | null): number | null {
  if (!value) {
    return null;
  }
  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : null;
}

function transform_heartbeat_config(api_config: ApiHeartbeatStatus): HeartbeatConfig {
  return {
    ...api_config,
    next_run_at: to_timestamp(api_config.next_run_at),
    last_heartbeat_at: to_timestamp(api_config.last_heartbeat_at),
    last_ack_at: to_timestamp(api_config.last_ack_at),
  };
}

export async function get_heartbeat_config_api(
  agent_id: string,
): Promise<HeartbeatConfig> {
  const result = await request_api<ApiHeartbeatStatus>(
    `${HEARTBEAT_API_BASE_URL}/${encodeURIComponent(agent_id)}`,
    {
      method: "GET",
      headers: { "Content-Type": "application/json" },
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
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        mode: params.mode ?? "now",
        text: params.text,
      }),
    },
  );

  return result;
}
