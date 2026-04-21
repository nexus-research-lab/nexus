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

export interface LauncherSuggestion {
  type: "agent" | "room";
  id: string;
  name: string;
  avatar?: string;
  last_activity?: string;
}

export interface LauncherSuggestionsResponse {
  agents: LauncherSuggestion[];
  rooms: LauncherSuggestion[];
}

export async function get_launcher_bootstrap_api(): Promise<LauncherBootstrapResponse> {
  return request_api<LauncherBootstrapResponse>(
    `${get_agent_api_base_url()}/launcher/bootstrap`,
    {
      method: "GET",
    },
  );
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

/**
 * 获取 Launcher 推荐列表
 */
export async function get_launcher_suggestions(): Promise<LauncherSuggestionsResponse> {
  return request_api<LauncherSuggestionsResponse>(
    `${get_agent_api_base_url()}/launcher/suggestions`,
    {
      method: "GET",
    },
  );
}
