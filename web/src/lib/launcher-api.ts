/**
 * Launcher API 客户端
 */

import { getAgentApiBaseUrl } from '@/config/options';
import { ApiResponse } from '@/types/api';

export interface LauncherQueryParams {
  query: string;
}

export interface LauncherQueryResponse {
  action_type: 'open_agent_dm' | 'open_room' | 'open_app';
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
  const response = await fetch(`${getAgentApiBaseUrl()}/launcher/query`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(params),
  });

  if (!response.ok) {
    throw new Error(`Launcher 查询失败: ${response.statusText}`);
  }
  const result: ApiResponse<LauncherQueryResponse> = await response.json();
  return result.data;
}

/**
 * 获取 Launcher 推荐列表
 */
export async function getLauncherSuggestions(): Promise<LauncherSuggestionsResponse> {
  const response = await fetch(`${getAgentApiBaseUrl()}/launcher/suggestions`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`获取推荐列表失败: ${response.statusText}`);
  }
  const result: ApiResponse<LauncherSuggestionsResponse> = await response.json();
  return result.data;
}
