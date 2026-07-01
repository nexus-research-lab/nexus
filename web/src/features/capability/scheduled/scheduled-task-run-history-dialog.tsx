"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { History, RefreshCw, X } from "lucide-react";

import { write_text_to_clipboard } from "@/hooks/ui/clipboard";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { list_scheduled_task_runs_api } from "@/lib/api/scheduled-task-api";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { close_on_escape } from "@/shared/ui/dialog/dialog-keyboard";
import { UiSkeletonCardList } from "@/shared/ui/skeleton";
import { UiStateBlock } from "@/shared/ui/state-block";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/controls/workspace-status-badge";
import type { ScheduledTaskItem, ScheduledTaskRunItem } from "@/types/capability/scheduled-task";
import { ScheduledTaskRunHistoryItem } from "./scheduled-task-run-history-item";
import { build_run_diagnostic } from "./scheduled-task-run-history-model";

interface ScheduledTaskRunHistoryDialogProps {
  task: ScheduledTaskItem | null;
  is_open: boolean;
  on_close: () => void;
  on_retry_task?: (task: ScheduledTaskItem) => void | Promise<void>;
  on_retry_delivery?: (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => void | Promise<void>;
  on_recover_task_run?: (task: ScheduledTaskItem, run: ScheduledTaskRunItem) => void | Promise<void>;
}

interface RunHistoryDialogState {
  action_message: string | null;
  copied_run_id: string | null;
  error_message: string | null;
  is_loading: boolean;
  recovering_run_id: string | null;
  retrying_delivery_run_id: string | null;
  retrying_run_id: string | null;
  runs: ScheduledTaskRunItem[];
}

export function ScheduledTaskRunHistoryDialog({
  task,
  is_open,
  on_close,
  on_retry_task,
  on_retry_delivery,
  on_recover_task_run,
}: ScheduledTaskRunHistoryDialogProps) {
  const active_task_job_id_ref = useRef<string | null>(null);
  const runs_request_token_ref = useRef(0);
  const task_job_id = task?.job_id ?? null;
  const [state, set_state] = useResettableState<RunHistoryDialogState>(
    {
      action_message: null,
      copied_run_id: null,
      error_message: null,
      is_loading: Boolean(is_open && task_job_id),
      recovering_run_id: null,
      retrying_delivery_run_id: null,
      retrying_run_id: null,
      runs: [],
    },
    is_open && task_job_id ? task_job_id : "closed",
  );
  const {
    action_message,
    copied_run_id,
    error_message,
    is_loading,
    recovering_run_id,
    retrying_delivery_run_id,
    retrying_run_id,
    runs,
  } = state;

  const load_runs = useCallback(async (job_id: string) => {
    const request_token = runs_request_token_ref.current + 1;
    runs_request_token_ref.current = request_token;
    set_state((current) => ({ ...current, error_message: null, is_loading: true }));
    try {
      const result = await list_scheduled_task_runs_api(job_id);
      if (active_task_job_id_ref.current !== job_id || runs_request_token_ref.current !== request_token) {
        return;
      }
      set_state((current) => ({ ...current, runs: result }));
    } catch (error) {
      if (active_task_job_id_ref.current !== job_id || runs_request_token_ref.current !== request_token) {
        return;
      }
      set_state((current) => ({
        ...current,
        error_message: error instanceof Error ? error.message : "加载运行历史失败",
        runs: [],
      }));
    } finally {
      if (active_task_job_id_ref.current !== job_id || runs_request_token_ref.current !== request_token) {
        return;
      }
      set_state((current) => ({ ...current, is_loading: false }));
    }
  }, [set_state]);

  useEffect(() => {
    if (!is_open) {
      active_task_job_id_ref.current = null;
      runs_request_token_ref.current += 1;
      return;
    }
    const on_key_down = (event: KeyboardEvent) => close_on_escape(event, on_close);
    window.addEventListener("keydown", on_key_down);
    return () => {
      window.removeEventListener("keydown", on_key_down);
    };
  }, [is_open, on_close]);

  useEffect(() => {
    if (!is_open || !task_job_id) {
      active_task_job_id_ref.current = null;
      runs_request_token_ref.current += 1;
      return;
    }
    active_task_job_id_ref.current = task_job_id;
    void load_runs(task_job_id);
  }, [is_open, load_runs, task_job_id]);

  if (!is_open || !task) {
    return null;
  }

  const handle_refresh = () => {
    void load_runs(task_job_id ?? "");
  };

  const handle_copy_diagnostic = async (run: ScheduledTaskRunItem) => {
    const diagnostic = build_run_diagnostic(task, run);
    if (await write_text_to_clipboard(diagnostic)) {
      set_state((current) => ({
        ...current,
        action_message: "诊断信息已复制",
        copied_run_id: run.run_id,
      }));
      return;
    }
    set_state((current) => ({
      ...current,
      action_message: "浏览器未允许写入剪贴板，请使用运行产物查看完整诊断",
    }));
  };

  const handle_retry = async (run: ScheduledTaskRunItem) => {
    if (!on_retry_task || !task_job_id) {
      return;
    }
    set_state((current) => ({ ...current, action_message: null, retrying_run_id: run.run_id }));
    try {
      await on_retry_task(task);
      await load_runs(task_job_id);
      set_state((current) => ({ ...current, action_message: "已触发重新运行" }));
    } catch (error) {
      set_state((current) => ({
        ...current,
        action_message: error instanceof Error ? error.message : "重新运行失败",
      }));
    } finally {
      set_state((current) => ({ ...current, retrying_run_id: null }));
    }
  };

  const handle_retry_delivery = async (run: ScheduledTaskRunItem) => {
    if (!on_retry_delivery || !task_job_id) {
      return;
    }
    set_state((current) => ({ ...current, action_message: null, retrying_delivery_run_id: run.run_id }));
    try {
      await on_retry_delivery(task, run);
      await load_runs(task_job_id);
      set_state((current) => ({ ...current, action_message: "已重试投递" }));
    } catch (error) {
      set_state((current) => ({
        ...current,
        action_message: error instanceof Error ? error.message : "重试投递失败",
      }));
    } finally {
      set_state((current) => ({ ...current, retrying_delivery_run_id: null }));
    }
  };

  const handle_recover = async (run: ScheduledTaskRunItem) => {
    if (!on_recover_task_run || !task_job_id) {
      return;
    }
    if (!window.confirm(`确认释放 run ${run.run_id} 的运行占用吗？该 run 会被标记为 cancelled。`)) {
      return;
    }
    set_state((current) => ({ ...current, action_message: null, recovering_run_id: run.run_id }));
    try {
      await on_recover_task_run(task, run);
      await load_runs(task_job_id);
      set_state((current) => ({ ...current, action_message: "已释放运行占用" }));
    } catch (error) {
      set_state((current) => ({
        ...current,
        action_message: error instanceof Error ? error.message : "释放运行占用失败",
      }));
    } finally {
      set_state((current) => ({ ...current, recovering_run_id: null }));
    }
  };

  return (
    <div
      aria-labelledby="scheduled-task-run-history-title"
      aria-modal="true"
      className="dialog-backdrop"
      data-modal-root="true"
      role="dialog"
    >
      <div className="dialog-shell surface-radius-md flex h-[82vh] w-full max-w-4xl flex-col overflow-hidden">
        <div className="dialog-header">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="dialog-title" id="scheduled-task-run-history-title">
                {task.name} 运行历史
              </h3>
              <WorkspaceStatusBadge
                label={task.running ? "运行中" : task.enabled ? "已启用" : "已暂停"}
                size="compact"
                tone={task.running ? "running" : task.enabled ? "active" : "idle"}
              />
            </div>
            <p className="dialog-subtitle mt-1">
              Job ID: {task.job_id}
            </p>
            {action_message ? (
              <p className="mt-2 text-xs font-medium text-(--text-default)">
                {action_message}
              </p>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            <UiButton
              onClick={() => void handle_refresh()}
              size="xs"
              type="button"
              variant="text"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              刷新
            </UiButton>
            <UiIconButton
              aria-label="关闭"
              onClick={on_close}
              size="md"
              type="button"
            >
              <X className="h-4 w-4" />
            </UiIconButton>
          </div>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-6 py-5">
          {is_loading ? (
            <UiSkeletonCardList card_class_name="min-h-[108px]" count={4} />
          ) : error_message ? (
            <UiStateBlock description={error_message} title="运行历史加载失败" tone="danger" />
          ) : runs.length === 0 ? (
            <UiStateBlock
              description="手动执行或等调度器首次触发后，这里会显示每次运行的状态、耗时和错误信息。"
              icon={<History className="h-6 w-6 text-(--icon-strong)" />}
              title="还没有运行记录"
            />
          ) : (
            <div className="divide-y divide-(--divider-subtle-color)">
              {runs.map((run) => (
                <ScheduledTaskRunHistoryItem
                  can_recover_task_run={Boolean(on_recover_task_run)}
                  can_retry_delivery={Boolean(on_retry_delivery)}
                  can_retry_task={Boolean(on_retry_task)}
                  copied_run_id={copied_run_id}
                  key={run.run_id}
                  on_copy_diagnostic={handle_copy_diagnostic}
                  on_recover={handle_recover}
                  on_retry={handle_retry}
                  on_retry_delivery={handle_retry_delivery}
                  recovering_run_id={recovering_run_id}
                  retrying_delivery_run_id={retrying_delivery_run_id}
                  retrying_run_id={retrying_run_id}
                  run={run}
                  task={task}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
