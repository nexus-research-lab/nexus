// INPUT: 真实历史资源/命令 Hook、共享 Dialog、受控读取接口与当前语言。
// OUTPUT: 证明实例标题隔离、仅当前 Job 事件触发对账刷新，以及语言切换后仍确认 exact run/投递 attempt。
// POS: Scheduled 历史装配 DOM 回归；不启动浏览器、不向真实任务提交命令。

import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { StrictMode, type ComponentProps } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { listScheduledTaskRunsApi } from "@/lib/api/capability/scheduled-task-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { useAppEventRefresh, type AppEventRefreshOptions } from "@/lib/websocket/use-app-event-refresh";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { createPendingCommandState } from "../controller/pending-command-model";
import { SCHEDULED_TASK_COMMAND_KINDS } from "../controller/scheduled-task-directory-model";
import { ScheduledTaskRunHistoryDialog } from "./scheduled-task-run-history-dialog";

vi.mock("@/lib/api/capability/scheduled-task-api", () => ({ listScheduledTaskRunsApi: vi.fn() }));
vi.mock("@/lib/websocket/use-app-event-refresh", () => ({ useAppEventRefresh: vi.fn() }));

function taskChanged(jobId: string) {
  return { event_type: "scheduled_task_changed", data: { job_id: jobId } } as unknown as Parameters<AppEventRefreshOptions["onEvent"]>[0];
}
// 取最近一次挂载里认领该 Job 事件的推送订阅，模拟服务端推送触发的刷新。
function pushOptions(jobId: string): AppEventRefreshOptions {
  const calls = vi.mocked(useAppEventRefresh).mock.calls.map(([options]) => options);
  const match = calls.reverse().find((options) => options.enabled !== false && options.onEvent(taskChanged(jobId)));
  if (!match) throw new Error(`no push subscription for ${jobId}`);
  return match;
}
function pushRefresh(jobId = TASK.job_id) {
  // 与真实 Hook 一致：推送刷新的失败由资源自身呈现，不向上抛出。
  void pushOptions(jobId).refresh().catch(() => undefined);
}

const TASK: ScheduledTaskItem = {
  agent_id: "current-agent", configuration_version: 7, delivery: { mode: "none" }, enabled: true,
  expires_at: null, failure_streak: 0, instruction: "Summarize", job_id: "job-1", last_error: null,
  last_run_at: null, name: "Daily report", next_run_at: null, running: false, running_started_at: null,
  schedule: { interval_seconds: 86_400, kind: "every" }, session_target: { kind: "main" }, source: { kind: "user_page" },
};
const RUN: ScheduledTaskRunItem = {
  attempts: 1, delivered_at: null, delivery_dead_letter_at: null, delivery_next_attempt_at: null,
  finished_at: 61_000, job_id: TASK.job_id, run_id: "original-run", scheduled_for: 1_000, started_at: 1_000, status: "succeeded",
};

type DialogProps = ComponentProps<typeof ScheduledTaskRunHistoryDialog>;
function props(overrides: Partial<DialogProps> = {}): DialogProps {
  return {
    isOpen: true, onClose: vi.fn(), onRecoverTaskRun: vi.fn(async () => ({ status: "completed" as const })),
    onRetryDelivery: vi.fn(async () => ({ status: "completed" as const })), onRetryTask: vi.fn(async () => ({ status: "completed" as const })),
    onRunHistoryReconciled: vi.fn(), scopeKey: "owner-1", task: TASK,
    unconfirmed: createPendingCommandState(SCHEDULED_TASK_COMMAND_KINDS), ...overrides,
  };
}
function view(dialogProps: DialogProps[], locale: "zh" | "en" = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <StrictMode><I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>
    {dialogProps.map((item, index) => <ScheduledTaskRunHistoryDialog key={index} {...item} />)}
  </I18N_CONTEXT.Provider></StrictMode>;
}

beforeEach(() => {
  vi.mocked(listScheduledTaskRunsApi).mockReset().mockResolvedValue([]);
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

describe("ScheduledTaskRunHistoryDialog", () => {
  it("keeps title associations unique and current when two history instances coexist", async () => {
    const first = props();
    const second = props({ task: { ...TASK, job_id: "job-2", name: "Weekly report", enabled: false } });
    const { rerender } = render(view([first, second]));
    await waitFor(() => expect(screen.getAllByText("No runs yet")).toHaveLength(2));
    const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
    expect(dialogs).toHaveLength(2);
    const ids = dialogs.map((dialog) => dialog.getAttribute("aria-labelledby"));
    expect(new Set(ids).size).toBe(2);
    expect(ids.every(Boolean)).toBe(true);
    for (const [index, dialog] of dialogs.entries()) {
      expect(dialog.contains(document.getElementById(ids[index]!))).toBe(true);
    }
    expect(document.getElementById(ids[0]!)?.textContent).toBe("Daily reportEnabled");
    expect(document.getElementById(ids[1]!)?.textContent).toBe("Weekly reportPaused");
    rerender(view([first, second], "zh"));
    expect(dialogs.map((dialog) => dialog.getAttribute("aria-labelledby"))).toEqual(ids);
    expect(document.getElementById(ids[0]!)?.textContent).toBe("Daily report已启用");
    expect(document.getElementById(ids[1]!)?.textContent).toBe("Weekly report已暂停");
  });

  it("refreshes only on the current Job's pushed change and reconciles the requested history", async () => {
    const callbacks = props();
    render(view([callbacks]));
    expect(await screen.findByText("No runs yet")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Refresh" })).toBeNull();
    const options = pushOptions(TASK.job_id);
    expect(options.onEvent(taskChanged("job-2"))).toBe(false);
    vi.mocked(listScheduledTaskRunsApi).mockClear();
    let resolve!: (runs: ScheduledTaskRunItem[]) => void;
    vi.mocked(listScheduledTaskRunsApi).mockReturnValueOnce(new Promise((done) => { resolve = done; }));
    act(() => pushRefresh());
    expect(listScheduledTaskRunsApi).toHaveBeenCalledExactlyOnceWith(TASK.job_id);
    await act(async () => resolve([RUN]));
    expect(callbacks.onRunHistoryReconciled).toHaveBeenCalledExactlyOnceWith(TASK, [RUN]);
    expect(callbacks.onRetryTask).not.toHaveBeenCalled();
    expect(callbacks.onRetryDelivery).not.toHaveBeenCalled();
    expect(callbacks.onRecoverTaskRun).not.toHaveBeenCalled();
  });

  it("presents a submitted action with stale history as a warning in the current language", async () => {
    const run: ScheduledTaskRunItem = { ...RUN, status: "failed" };
    vi.mocked(listScheduledTaskRunsApi).mockResolvedValue([run]);
    const callbacks = props();
    const { rerender } = render(view([callbacks]));
    const retry = await screen.findByRole("button", { name: "Run again" });
    vi.mocked(listScheduledTaskRunsApi).mockRejectedValue(new Error("private-read-failure"));
    fireEvent.click(retry);
    const title = await screen.findByText("Action submitted; history not refreshed");
    const feedback = title.closest('[data-resource-state="error"]')!;

    expect(feedback.textContent).toContain("do not repeat the action");
    expect(feedback.textContent).not.toContain("private-read-failure");
    rerender(view([callbacks], "zh"));
    expect(screen.getByText("操作已提交，历史尚未刷新")).toBeTruthy();
    expect(callbacks.onRetryTask).toHaveBeenCalledOnce();
    expect(callbacks.onRetryDelivery).not.toHaveBeenCalled();
    expect(callbacks.onRecoverTaskRun).not.toHaveBeenCalled();
  });

  it.each([401, 403])("hides an inaccessible snapshot and keeps its retry disabled while recovering from %s", async (status) => {
    const run = { ...RUN, result_text: "private historical result" };
    vi.mocked(listScheduledTaskRunsApi).mockResolvedValue([run]);
    const callbacks = props();
    render(view([callbacks]));
    expect(await screen.findByText(run.result_text)).toBeTruthy();
    vi.mocked(listScheduledTaskRunsApi).mockRejectedValue(new ApiRequestError("private denial detail", status));
    act(() => pushRefresh());
    expect(await screen.findByText("This account cannot access this content")).toBeTruthy();
    expect(screen.queryByText(run.result_text)).toBeNull();
    expect(document.body.textContent).not.toContain("private denial detail");
    vi.mocked(listScheduledTaskRunsApi).mockClear();
    let resolve!: (runs: ScheduledTaskRunItem[]) => void;
    vi.mocked(listScheduledTaskRunsApi).mockReturnValue(new Promise((done) => { resolve = done; }));
    const retry = screen.getByRole("button", { name: "Retry" }) as HTMLButtonElement;
    fireEvent.click(retry);
    expect(retry.disabled).toBe(true);
    expect(retry.getAttribute("aria-busy")).toBe("true");
    fireEvent.click(retry);
    expect(listScheduledTaskRunsApi).toHaveBeenCalledExactlyOnceWith(TASK.job_id);
    expect(screen.queryByText(run.result_text)).toBeNull();
    await act(async () => resolve([]));
    expect(screen.getByText("No runs yet")).toBeTruthy();
    expect(callbacks.onRunHistoryReconciled).toHaveBeenCalledExactlyOnceWith(TASK, []);
    expect(callbacks.onRetryTask).not.toHaveBeenCalled();
    expect(callbacks.onRetryDelivery).not.toHaveBeenCalled();
    expect(callbacks.onRecoverTaskRun).not.toHaveBeenCalled();
  });

  it.each(["recover", "delivery"] as const)("preserves the %s confirmation target across a language change", async (kind) => {
    const recovering = kind === "recover";
    const run: ScheduledTaskRunItem = recovering
      ? { ...RUN, status: "running", finished_at: null }
      : { ...RUN, delivery_status: "retrying", delivery_attempts: 3 };
    const task = { ...TASK, running: recovering };
    vi.mocked(listScheduledTaskRunsApi).mockResolvedValue([run]);
    const callbacks = props({ task });
    const { rerender } = render(view([callbacks]));
    fireEvent.click(await screen.findByRole("button", { name: recovering ? "Release run" : "Checked; deliver again" }));
    const callback = recovering ? callbacks.onRecoverTaskRun : callbacks.onRetryDelivery;
    expect(callback).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: recovering ? "Release this run" : "Confirm delivery retry" })).toBeTruthy();
    rerender(view([callbacks], "zh"));
    const confirmation = screen.getByRole("dialog", { name: recovering ? "释放运行占用" : "确认重新投递" });
    expect(confirmation.textContent).not.toContain(run.run_id);
    fireEvent.click(within(confirmation).getByRole("button", { name: recovering ? "释放占用" : "确认未收到，重新投递" }));
    await waitFor(() => expect(callback).toHaveBeenCalledOnce());
    if (recovering) {
      expect(callbacks.onRecoverTaskRun).toHaveBeenCalledExactlyOnceWith(task, run);
      expect(callbacks.onRetryDelivery).not.toHaveBeenCalled();
    } else {
      expect(callbacks.onRetryDelivery).toHaveBeenCalledExactlyOnceWith(task, run, expect.any(Function), { confirmUnverifiedAttempt: true });
      expect(callbacks.onRecoverTaskRun).not.toHaveBeenCalled();
    }
    expect(callbacks.onRetryTask).not.toHaveBeenCalled();
  });
});
