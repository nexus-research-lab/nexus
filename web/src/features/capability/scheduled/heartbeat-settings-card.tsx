"use client";

import { Activity, RefreshCw, TimerReset, Zap } from "lucide-react";

import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/workspace-surface-header";
import type { HeartbeatConfig } from "@/types/heartbeat";

function format_datetime(value: number | null): string {
  if (!value) {
    return "未记录";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(value);
}

function format_interval(seconds: number): string {
  if (seconds % 86400 === 0) {
    return `${seconds / 86400} 天`;
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600} 小时`;
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60} 分钟`;
  }
  return `${seconds} 秒`;
}

function get_target_mode_label(mode: HeartbeatConfig["target_mode"]): string {
  if (mode === "last") {
    return "回到最近会话";
  }
  if (mode === "explicit") {
    return "回到指定位置";
  }
  return "不投递";
}

interface HeartbeatSettingsCardProps {
  heartbeat: HeartbeatConfig | null;
  is_loading: boolean;
  error_message: string | null;
  wake_pending?: boolean;
  on_refresh: () => void | Promise<void>;
  on_wake: () => void | Promise<void>;
}

export function HeartbeatSettingsCard({
  heartbeat,
  is_loading,
  error_message,
  wake_pending = false,
  on_refresh,
  on_wake,
}: HeartbeatSettingsCardProps) {
  return (
    <section className="surface-card flex min-h-[280px] flex-col rounded-[22px] px-4 py-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <Activity className="h-4 w-4 text-(--icon-default)" />
            <div>
              <h2 className="text-[15px] font-semibold tracking-[-0.03em] text-(--text-strong)">
                主会话轮询
              </h2>
              <p className="text-xs text-(--text-default)">
                按固定间隔唤醒主会话；适合让同一会话持续接住自动化任务和回复。
              </p>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <WorkspaceSurfaceToolbarAction
            disabled={is_loading}
            onClick={() => void on_refresh()}
          >
            <RefreshCw className="h-3.5 w-3.5" />
            刷新
          </WorkspaceSurfaceToolbarAction>
          <WorkspaceSurfaceToolbarAction
            disabled={is_loading || wake_pending}
            onClick={() => void on_wake()}
            tone="primary"
          >
            <Zap className="h-3.5 w-3.5" />
            {wake_pending ? "唤醒中" : "立即唤醒"}
          </WorkspaceSurfaceToolbarAction>
        </div>
      </div>

      <div className="mt-4 flex min-h-0 flex-1 flex-col gap-4">
        {is_loading ? (
          <div className="grid gap-3 sm:grid-cols-2">
            {Array.from({ length: 4 }).map((_, index) => (
              <div
                key={index}
                className="h-20 animate-pulse rounded-[16px] border border-(--divider-subtle-color)"
              />
            ))}
          </div>
        ) : error_message ? (
          <div className="flex min-h-[180px] flex-1 flex-col items-center justify-center rounded-[18px] border border-[color:color-mix(in_srgb,var(--destructive)_15%,transparent)] px-5 text-center">
            <p className="text-sm font-semibold text-(--destructive)">Heartbeat 加载失败</p>
            <p className="mt-2 max-w-sm text-sm leading-6 text-(--text-default)">
              {error_message}
            </p>
          </div>
        ) : heartbeat ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <WorkspaceStatusBadge
                label={heartbeat.enabled ? "已加入轮询" : "未加入轮询"}
                tone={heartbeat.enabled ? "active" : "idle"}
              />
              <WorkspaceStatusBadge
                label={heartbeat.running ? "调度器在线" : "调度器离线"}
                tone={heartbeat.running ? "running" : "idle"}
              />
              {heartbeat.pending_wake ? (
                <WorkspaceStatusBadge label="唤醒已排队" tone="default" />
              ) : null}
            </div>

            <div className="grid gap-4 border-y border-(--divider-subtle-color) py-4 sm:grid-cols-2">
              <div className="min-w-0">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                  轮询间隔
                </p>
                <p className="mt-2 text-base font-semibold text-(--text-strong)">
                  {format_interval(heartbeat.every_seconds)}
                </p>
                <p className="mt-1 text-xs text-(--text-default)">
                  下一次 {format_datetime(heartbeat.next_run_at)}
                </p>
              </div>

              <div className="min-w-0">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
                  回复方式
                </p>
                <p className="mt-2 text-base font-semibold text-(--text-strong)">
                  {get_target_mode_label(heartbeat.target_mode)}
                </p>
                <p className="mt-1 text-xs text-(--text-default)">
                  ACK 上限 {heartbeat.ack_max_chars} 字
                </p>
              </div>
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-(--text-default)">最近轮询</span>
                <span className="font-medium text-(--text-strong)">
                  {format_datetime(heartbeat.last_heartbeat_at)}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-(--text-default)">最近 ACK</span>
                <span className="font-medium text-(--text-strong)">
                  {format_datetime(heartbeat.last_ack_at)}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-(--text-default)">下一次调度</span>
                <span className="font-medium text-(--text-strong)">
                  {format_datetime(heartbeat.next_run_at)}
                </span>
              </div>
            </div>

            {heartbeat.delivery_error ? (
              <div className="rounded-[16px] border border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] px-4 py-3 text-sm text-(--warning)">
                <div className="flex items-center gap-2 font-semibold">
                  <TimerReset className="h-4 w-4" />
                  最近一次投递异常
                </div>
                <p className="mt-1 leading-6">{heartbeat.delivery_error}</p>
              </div>
            ) : null}
          </>
        ) : (
          <div className="flex min-h-[180px] flex-1 flex-col items-center justify-center rounded-[18px] border border-dashed border-(--divider-subtle-color) px-5 text-center">
            <p className="text-sm font-semibold text-(--text-strong)">
              当前 Agent 还没有 heartbeat 配置
            </p>
            <p className="mt-2 max-w-sm text-sm leading-6 text-(--text-default)">
              当后端启用 heartbeat 后，这里会展示运行状态、下一次执行时间和唤醒入口。
            </p>
          </div>
        )}
      </div>
    </section>
  );
}
