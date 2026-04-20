"use client";

import { useEffect, useState } from "react";
import { Activity, Check, Pencil, RefreshCw, TimerReset, X, Zap } from "lucide-react";

import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type {
  HeartbeatConfig,
  HeartbeatTargetMode,
  HeartbeatUpdateInput,
} from "@/types/capability/heartbeat";
import { format_scheduled_datetime } from "./scheduled-formatters";

// 预置间隔档位，覆盖日常高频选择；自定义值仍允许通过秒数输入。
const INTERVAL_PRESETS: Array<{ label: string; seconds: number }> = [
  { label: "1 分钟", seconds: 60 },
  { label: "5 分钟", seconds: 300 },
  { label: "15 分钟", seconds: 900 },
  { label: "30 分钟", seconds: 1800 },
  { label: "1 小时", seconds: 3600 },
  { label: "6 小时", seconds: 21600 },
  { label: "24 小时", seconds: 86400 },
];

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

function get_target_mode_label(mode: HeartbeatTargetMode): string {
  if (mode === "last") {
    return "回到最近会话";
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
  on_save?: (payload: HeartbeatUpdateInput) => Promise<HeartbeatConfig>;
}

export function HeartbeatSettingsCard({
  heartbeat,
  is_loading,
  error_message,
  wake_pending = false,
  on_refresh,
  on_wake,
  on_save,
}: HeartbeatSettingsCardProps) {
  const is_editable = Boolean(on_save);
  const [is_editing, set_is_editing] = useState(false);
  const [draft, set_draft] = useState<HeartbeatUpdateInput | null>(null);
  const [save_pending, set_save_pending] = useState(false);
  const [save_error, set_save_error] = useState<string | null>(null);

  // 切换 agent 或刷新 heartbeat 时，丢弃本地草稿与编辑态，避免跨配置串写。
  useEffect(() => {
    set_is_editing(false);
    set_draft(null);
    set_save_error(null);
  }, [heartbeat?.agent_id]);

  function enter_edit() {
    if (!heartbeat) {
      return;
    }
    set_draft({
      enabled: heartbeat.enabled,
      every_seconds: heartbeat.every_seconds,
      target_mode: heartbeat.target_mode,
      ack_max_chars: heartbeat.ack_max_chars,
    });
    set_save_error(null);
    set_is_editing(true);
  }

  function cancel_edit() {
    set_is_editing(false);
    set_draft(null);
    set_save_error(null);
  }

  async function handle_save() {
    if (!on_save || !draft) {
      return;
    }
    if (draft.every_seconds < 1) {
      set_save_error("轮询间隔必须是不小于 1 秒的整数");
      return;
    }
    if (draft.ack_max_chars < 0) {
      set_save_error("ACK 字数上限不能为负数");
      return;
    }
    set_save_pending(true);
    set_save_error(null);
    try {
      await on_save(draft);
      set_is_editing(false);
      set_draft(null);
    } catch (error) {
      set_save_error(error instanceof Error ? error.message : "保存心跳配置失败");
    } finally {
      set_save_pending(false);
    }
  }

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
          {is_editing ? (
            <>
              <WorkspaceSurfaceToolbarAction
                disabled={save_pending}
                onClick={cancel_edit}
              >
                <X className="h-3.5 w-3.5" />
                取消
              </WorkspaceSurfaceToolbarAction>
              <WorkspaceSurfaceToolbarAction
                disabled={save_pending}
                onClick={() => void handle_save()}
                tone="primary"
              >
                <Check className="h-3.5 w-3.5" />
                {save_pending ? "保存中" : "保存"}
              </WorkspaceSurfaceToolbarAction>
            </>
          ) : (
            <>
              <WorkspaceSurfaceToolbarAction
                disabled={is_loading}
                onClick={() => void on_refresh()}
              >
                <RefreshCw className="h-3.5 w-3.5" />
                刷新
              </WorkspaceSurfaceToolbarAction>
              {is_editable ? (
                <WorkspaceSurfaceToolbarAction
                  disabled={is_loading || !heartbeat}
                  onClick={enter_edit}
                >
                  <Pencil className="h-3.5 w-3.5" />
                  编辑
                </WorkspaceSurfaceToolbarAction>
              ) : null}
              <WorkspaceSurfaceToolbarAction
                disabled={is_loading || wake_pending}
                onClick={() => void on_wake()}
                tone="primary"
              >
                <Zap className="h-3.5 w-3.5" />
                {wake_pending ? "唤醒中" : "立即唤醒"}
              </WorkspaceSurfaceToolbarAction>
            </>
          )}
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
          is_editing && draft ? (
            <HeartbeatEditForm
              draft={draft}
              on_change={set_draft}
              save_error={save_error}
            />
          ) : (
            <HeartbeatReadOnlyView heartbeat={heartbeat} />
          )
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

function HeartbeatReadOnlyView({ heartbeat }: { heartbeat: HeartbeatConfig }) {
  return (
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
            下一次 {format_scheduled_datetime(heartbeat.next_run_at)}
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
            {format_scheduled_datetime(heartbeat.last_heartbeat_at)}
          </span>
        </div>
        <div className="flex items-center justify-between gap-3 text-sm">
          <span className="text-(--text-default)">最近 ACK</span>
          <span className="font-medium text-(--text-strong)">
            {format_scheduled_datetime(heartbeat.last_ack_at)}
          </span>
        </div>
        <div className="flex items-center justify-between gap-3 text-sm">
          <span className="text-(--text-default)">下一次调度</span>
          <span className="font-medium text-(--text-strong)">
            {format_scheduled_datetime(heartbeat.next_run_at)}
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
  );
}

interface HeartbeatEditFormProps {
  draft: HeartbeatUpdateInput;
  on_change: (next: HeartbeatUpdateInput) => void;
  save_error: string | null;
}

const FIELD_LABEL_CLASS =
  "text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)";
const INPUT_CLASS =
  "mt-2 h-9 w-full rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 text-sm text-(--text-strong) outline-none focus:border-(--accent)";

function HeartbeatEditForm({ draft, on_change, save_error }: HeartbeatEditFormProps) {
  const matched_preset = INTERVAL_PRESETS.find((preset) => preset.seconds === draft.every_seconds);
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 border-y border-(--divider-subtle-color) py-4 sm:grid-cols-2">
        <label className="min-w-0 text-sm">
          <span className={FIELD_LABEL_CLASS}>启用心跳</span>
          <div className="mt-2 flex items-center gap-2">
            <input
              checked={draft.enabled}
              className="h-4 w-4 accent-(--accent)"
              onChange={(event) => on_change({ ...draft, enabled: event.target.checked })}
              type="checkbox"
            />
            <span className="text-(--text-default)">
              {draft.enabled ? "会按下方间隔轮询唤醒主会话" : "暂停心跳（保留配置）"}
            </span>
          </div>
        </label>

        <label className="min-w-0 text-sm">
          <span className={FIELD_LABEL_CLASS}>回复方式</span>
          <select
            className={INPUT_CLASS}
            onChange={(event) =>
              on_change({ ...draft, target_mode: event.target.value as HeartbeatTargetMode })
            }
            value={draft.target_mode}
          >
            <option value="none">不投递</option>
            <option value="last">回到最近会话</option>
          </select>
        </label>

        <label className="min-w-0 text-sm">
          <span className={FIELD_LABEL_CLASS}>轮询间隔</span>
          <select
            className={INPUT_CLASS}
            onChange={(event) => {
              const value = event.target.value;
              if (value === "custom") {
                return;
              }
              on_change({ ...draft, every_seconds: Number(value) });
            }}
            value={matched_preset ? String(matched_preset.seconds) : "custom"}
          >
            {INTERVAL_PRESETS.map((preset) => (
              <option key={preset.seconds} value={preset.seconds}>
                {preset.label}
              </option>
            ))}
            <option value="custom">自定义（秒）</option>
          </select>
          <input
            className={INPUT_CLASS}
            min={1}
            onChange={(event) =>
              on_change({ ...draft, every_seconds: Math.max(1, Number(event.target.value) || 0) })
            }
            type="number"
            value={draft.every_seconds}
          />
        </label>

        <label className="min-w-0 text-sm">
          <span className={FIELD_LABEL_CLASS}>ACK 字数上限</span>
          <input
            className={INPUT_CLASS}
            min={0}
            onChange={(event) =>
              on_change({ ...draft, ack_max_chars: Math.max(0, Number(event.target.value) || 0) })
            }
            type="number"
            value={draft.ack_max_chars}
          />
          <p className="mt-1 text-xs text-(--text-default)">
            主会话回执文本会被截断到此长度；0 表示不限制。
          </p>
        </label>
      </div>

      {save_error ? (
        <div className="rounded-[16px] border border-[color:color-mix(in_srgb,var(--destructive)_20%,transparent)] px-4 py-3 text-sm text-(--destructive)">
          {save_error}
        </div>
      ) : null}
    </div>
  );
}
