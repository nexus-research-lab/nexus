/**
 * Launcher API 客户端
 */

import { getAgentApiBaseUrl } from '@/config/options';
import { request_api } from '@/lib/http';

export interface LauncherQueryParams {
  query: string;
}

export interface LauncherQueryResponse {
  action_type: 'open_agent_dm' | 'open_room';
  target_id: string;
  initial_message?: string;
}

export interface LauncherSuggestion {
  type: 'agent' | 'room';
  id: string;
  name: string;
  avatar?: string;
  last_activity?: string;
}

export interface LauncherSuggestionsResponse {
  agents: LauncherSuggestion[];
  rooms: LauncherSuggestion[];
}

/**
 * 解析 Launcher 查询
 */
export async function queryLauncher(params: LauncherQueryParams): Promise<LauncherQueryResponse> {
  return request_api<LauncherQueryResponse>(`${getAgentApiBaseUrl()}/launcher/query`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(params),
  });
}

/**
 * 获取 Launcher 推荐列表
 */
export async function getLauncherSuggestions(): Promise<LauncherSuggestionsResponse> {
  return request_api<LauncherSuggestionsResponse>(`${getAgentApiBaseUrl()}/launcher/suggestions`, {
    method: 'GET',
  });
}
