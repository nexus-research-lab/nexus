"use client";

import { Activity, RefreshCw, TimerReset, Zap } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
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
    return "最近会话";
  }
  if (mode === "explicit") {
    return "显式目标";
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
    <section className="glass-panel-subtle radius-shell-xl flex min-h-[280px] flex-col overflow-hidden">
      <div className="border-b border-[var(--divider-subtle-color)] px-5 py-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <div className="glass-chip flex h-9 w-9 items-center justify-center rounded-2xl">
                <Activity className="h-4 w-4 text-slate-900/80" />
              </div>
              <div>
                <h2 className="text-[15px] font-semibold tracking-[-0.03em] text-[color:var(--text-strong)]">
                  Heartbeat 设置
                </h2>
                <p className="text-xs text-[color:var(--text-default)]">
                  查看当前自动唤醒状态并手动触发一次执行。
                </p>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <WorkspacePillButton
              density="compact"
              disabled={is_loading}
              onClick={() => void on_refresh()}
              size="sm"
              variant="outlined"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </WorkspacePillButton>
            <WorkspacePillButton
              density="compact"
              disabled={is_loading || wake_pending}
              onClick={() => void on_wake()}
              size="sm"
              variant="primary"
            >
              <Zap className="h-3.5 w-3.5" />
              {wake_pending ? "唤醒中" : "立即唤醒"}
            </WorkspacePillButton>
          </div>
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col p-5">
        {is_loading ? (
          <div className="grid gap-3 sm:grid-cols-2">
            {Array.from({ length: 4 }).map((_, index) => (
              <div
                key={index}
                className="h-20 animate-pulse rounded-[20px] bg-white/45"
              />
            ))}
          </div>
        ) : error_message ? (
          <div className="flex min-h-[180px] flex-1 flex-col items-center justify-center rounded-[24px] border border-rose-500/15 bg-rose-500/6 px-5 text-center">
            <p className="text-sm font-semibold text-rose-500">Heartbeat 加载失败</p>
            <p className="mt-2 max-w-sm text-sm leading-6 text-[color:var(--text-default)]">
              {error_message}
            </p>
          </div>
        ) : heartbeat ? (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <WorkspaceStatusBadge
                label={heartbeat.enabled ? "已启用" : "未启用"}
                tone={heartbeat.enabled ? "active" : "idle"}
              />
              <WorkspaceStatusBadge
                label={heartbeat.running ? "运行中" : "待命"}
                tone={heartbeat.running ? "running" : "idle"}
              />
              {heartbeat.pending_wake ? (
                <WorkspaceStatusBadge label="唤醒已排队" tone="default" />
              ) : null}
            </div>

            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <div className="rounded-[22px] border border-[var(--divider-subtle-color)] bg-white/55 px-4 py-3 shadow-[0_12px_30px_rgba(15,23,42,0.06)]">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[color:var(--text-muted)]">
                  执行间隔
                </p>
                <p className="mt-2 text-base font-semibold text-[color:var(--text-strong)]">
                  {format_interval(heartbeat.every_seconds)}
                </p>
                <p className="mt-1 text-xs text-[color:var(--text-default)]">
                  下一次 {format_datetime(heartbeat.next_run_at)}
                </p>
              </div>

              <div className="rounded-[22px] border border-[var(--divider-subtle-color)] bg-white/55 px-4 py-3 shadow-[0_12px_30px_rgba(15,23,42,0.06)]">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-[color:var(--text-muted)]">
                  投递策略
                </p>
                <p className="mt-2 text-base font-semibold text-[color:var(--text-strong)]">
                  {get_target_mode_label(heartbeat.target_mode)}
                </p>
                <p className="mt-1 text-xs text-[color:var(--text-default)]">
                  ACK 上限 {heartbeat.ack_max_chars} 字
                </p>
              </div>
            </div>

            <div className="mt-4 space-y-3 rounded-[24px] border border-[var(--divider-subtle-color)] bg-white/45 p-4">
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-[color:var(--text-default)]">最近心跳</span>
                <span className="font-medium text-[color:var(--text-strong)]">
                  {format_datetime(heartbeat.last_heartbeat_at)}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-[color:var(--text-default)]">最近 ACK</span>
                <span className="font-medium text-[color:var(--text-strong)]">
                  {format_datetime(heartbeat.last_ack_at)}
                </span>
              </div>
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="text-[color:var(--text-default)]">下一次调度</span>
                <span className="font-medium text-[color:var(--text-strong)]">
                  {format_datetime(heartbeat.next_run_at)}
                </span>
              </div>
            </div>

            {heartbeat.delivery_error ? (
              <div className="mt-4 rounded-[20px] border border-amber-500/20 bg-amber-500/8 px-4 py-3 text-sm text-amber-700">
                <div className="flex items-center gap-2 font-semibold">
                  <TimerReset className="h-4 w-4" />
                  最近一次投递异常
                </div>
                <p className="mt-1 leading-6">{heartbeat.delivery_error}</p>
              </div>
            ) : null}
          </>
        ) : (
          <div className="flex min-h-[180px] flex-1 flex-col items-center justify-center rounded-[24px] border border-dashed border-[var(--divider-subtle-color)] px-5 text-center">
            <p className="text-sm font-semibold text-[color:var(--text-strong)]">
              当前 Agent 还没有 heartbeat 配置
            </p>
            <p className="mt-2 max-w-sm text-sm leading-6 text-[color:var(--text-default)]">
              当后端启用 heartbeat 后，这里会展示运行状态、下一次执行时间和唤醒入口。
            </p>
          </div>
        )}
      </div>
    </section>
  );
}
