/**
 * Heartbeat 自动化类型定义
 */

export type HeartbeatTargetMode = "none" | "last" | "explicit";
export type HeartbeatWakeMode = "now" | "next-heartbeat";

export interface ApiHeartbeatStatus {
  agent_id: string;
  enabled: boolean;
  every_seconds: number;
  target_mode: HeartbeatTargetMode;
  ack_max_chars: number;
  running: boolean;
  pending_wake: boolean;
  next_run_at?: string | null;
  last_heartbeat_at?: string | null;
  last_ack_at?: string | null;
  delivery_error?: string | null;
}

export interface HeartbeatConfig extends Omit<ApiHeartbeatStatus, "next_run_at" | "last_heartbeat_at" | "last_ack_at"> {
  next_run_at: number | null;
  last_heartbeat_at: number | null;
  last_ack_at: number | null;
}

export interface WakeHeartbeatRequest {
  mode?: HeartbeatWakeMode;
  text?: string | null;
}

export interface ApiHeartbeatWakeResult {
  agent_id: string;
  mode: HeartbeatWakeMode;
  scheduled: boolean;
}

export interface HeartbeatWakeResult extends ApiHeartbeatWakeResult {}
