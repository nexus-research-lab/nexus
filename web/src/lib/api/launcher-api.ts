/**
 * Launcher API 客户端
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type { LauncherBootstrapResponse } from "@/types/app/launcher";

export interface LauncherQueryParams {
  query: string;
}

export interface LauncherQueryResponse {
  action_type: "open_agent_dm" | "open_room" | "open_app";
  target_id: string;
  initial_message?: string;
}

let launcher_bootstrap_inflight: Promise<LauncherBootstrapResponse> | null = null;

export async function get_launcher_bootstrap_api(): Promise<LauncherBootstrapResponse> {
  if (launcher_bootstrap_inflight) {
    return launcher_bootstrap_inflight;
  }

  launcher_bootstrap_inflight = request_api<LauncherBootstrapResponse>(
    `${get_agent_api_base_url()}/launcher/bootstrap`,
    {
      method: "GET",
    },
  ).finally(() => {
    launcher_bootstrap_inflight = null;
  });
  return launcher_bootstrap_inflight;
}

/**
 * 解析 Launcher 查询
 */
export async function query_launcher(
  params: LauncherQueryParams,
): Promise<LauncherQueryResponse> {
  return request_api<LauncherQueryResponse>(
    `${get_agent_api_base_url()}/launcher/query`,
    {
      method: "POST",
      body: JSON.stringify(params),
    },
  );
}
