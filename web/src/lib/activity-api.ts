/**
 * Activity API 客户端
 */

import { getAgentApiBaseUrl } from '@/config/options';
import { ApiResponse } from '@/types/api';
import type { ActivityEvent, ActivityEventType } from '../types/activity';

export interface ListActivityParams {
  limit?: number;
  offset?: number;
  event_type?: ActivityEventType;
  unreadOnly?: boolean;
}

/**
 * 获取活动事件列表
 */
export async function getActivityEvents(params: ListActivityParams = {}): Promise<ActivityEvent[]> {
  const searchParams = new URLSearchParams();
  if (params.limit !== undefined) searchParams.set('limit', String(params.limit));
  if (params.offset !== undefined) searchParams.set('offset', String(params.offset));
  if (params.event_type !== undefined) searchParams.set('event_type', params.event_type);
  if (params.unreadOnly) searchParams.set('unread_only', 'true');

  const response = await fetch(`${getAgentApiBaseUrl()}/activity?${searchParams}`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`获取活动列表失败: ${response.statusText}`);
  }
  const result: ApiResponse<ActivityEvent[]> = await response.json();
  return result.data;
}

/**
 * 标记事件为已读
 */
export async function markActivityAsRead(eventIds: string[]): Promise<void> {
  const response = await fetch(`${getAgentApiBaseUrl()}/activity/read`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ event_ids: eventIds }),
  });

  if (!response.ok) {
    throw new Error(`标记已读失败: ${response.statusText}`);
  }
  await response.json();
}

/**
 * 获取未读事件数量
 */
export async function getUnreadActivityCount(): Promise<number> {
  const response = await fetch(`${getAgentApiBaseUrl()}/activity/unread-count`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`获取未读数量失败: ${response.statusText}`);
  }
  const result: ApiResponse<{ unread_count: number }> = await response.json();
  return result.data.unread_count ?? 0;
}
