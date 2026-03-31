/**
 * Activity 类型定义
 */

export type ActivityEventType =
  | 'agent_created'
  | 'agent_updated'
  | 'room_created'
  | 'room_message'
  | 'dm_message'
  | 'skill_installed'
  | 'skill_uninstalled'
  | 'task_completed'
  | 'task_failed';

export interface ActivityEvent {
  id: string;
  event_type: ActivityEventType;
  actor_type: 'user' | 'agent' | 'system';
  actor_id?: string | null;
  target_type?: string | null;
  target_id?: string | null;
  summary?: string | null;
  metadata_json?: Record<string, any> | null;
  created_at: string;
}

export interface ActivityItem {
  event: ActivityEvent;
  icon: string;
  title: string;
  subtitle?: string;
  action_url?: string;
}
