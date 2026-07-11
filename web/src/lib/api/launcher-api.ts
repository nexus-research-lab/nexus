/**
 * Launcher API 客户端
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/core/http";
import type { LauncherBootstrapResponse } from "@/types/app/launcher";

export interface LauncherQueryParams {
  query: string;
}

export interface LauncherQueryResponse {
  action_type: "open_agent_dm" | "open_room" | "open_app";
  target_id: string;
  initial_message?: string;
}

let launcherBootstrapInflight: Promise<LauncherBootstrapResponse> | null = null;

export async function getLauncherBootstrapApi(): Promise<LauncherBootstrapResponse> {
  if (launcherBootstrapInflight) {
    return launcherBootstrapInflight;
  }

  launcherBootstrapInflight = requestApi<LauncherBootstrapResponse>(
    `${getAgentApiBaseUrl()}/launcher/bootstrap`,
    {
      method: "GET",
    },
  ).finally(() => {
    launcherBootstrapInflight = null;
  });
  return launcherBootstrapInflight;
}

/**
 * 解析 Launcher 查询
 */
export async function queryLauncher(
  params: LauncherQueryParams,
): Promise<LauncherQueryResponse> {
  return requestApi<LauncherQueryResponse>(
    `${getAgentApiBaseUrl()}/launcher/query`,
    {
      method: "POST",
      body: JSON.stringify(params),
    },
  );
}
