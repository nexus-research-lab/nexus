/**
 * INPUT: Launcher 查询参数、可选 AbortSignal 与运行时 API 根地址。
 * OUTPUT: 保持 wire 兼容的 bootstrap/query 响应。
 * POS: Launcher 的无状态 HTTP 协议客户端；请求合并归调用方资源状态机。
 */

import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
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

export async function getLauncherBootstrapApi(
  signal?: AbortSignal,
): Promise<LauncherBootstrapResponse> {
  return requestApi<LauncherBootstrapResponse>(
    `${getAgentApiBaseUrl()}/launcher/bootstrap`,
    {
      method: "GET",
      signal,
    },
  );
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
