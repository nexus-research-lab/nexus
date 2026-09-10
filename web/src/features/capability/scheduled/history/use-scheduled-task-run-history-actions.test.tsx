// INPUT: 真实历史动作 Hook、受控命令/剪贴板 Promise、进入代次与当前语言。
// OUTPUT: 证明迟到结果不跨进入代次，最新操作独占反馈，同步失败不遗留防重项且反馈不重放命令。
// POS: Scheduled 历史异步行为回归；所有副作用通过注入命令或共享剪贴板 mock 隔离。

import { act, cleanup, renderHook } from "@testing-library/react";
import { StrictMode, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiRequestError } from "@/lib/api/core/http-error";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { writeTextToClipboard } from "@/shared/lib/browser/clipboard";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { useScheduledTaskRunHistoryActions, type ScheduledTaskRunHistoryActionResult } from "./use-scheduled-task-run-history-actions";

vi.mock("@/shared/lib/browser/clipboard", () => ({ writeTextToClipboard: vi.fn() }));

const TASK: ScheduledTaskItem = {
  agent_id: "agent-1", configuration_version: 2, delivery: { mode: "none" }, enabled: true,
  expires_at: null, failure_streak: 0, instruction: "Summarize", job_id: "job-1", last_error: null,
  last_run_at: null, name: "Report", next_run_at: null, running: false, running_started_at: null,
  schedule: { interval_seconds: 60, kind: "every" }, session_target: { kind: "main" }, source: { kind: "user_page" },
};
const RUN: ScheduledTaskRunItem = {
  attempts: 1, delivered_at: null, delivery_dead_letter_at: null, delivery_next_attempt_at: null,
  finished_at: 61_000, job_id: TASK.job_id, run_id: "run-1", scheduled_for: 1_000, started_at: 1_000, status: "failed",
};
const COMPLETED: ScheduledTaskRunHistoryActionResult = { status: "completed" };

function pending<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  return { promise: new Promise<T>((done, fail) => { resolve = done; reject = fail; }), resolve, reject };
}
type Options = Parameters<typeof useScheduledTaskRunHistoryActions>[0];
function mount(overrides: Partial<Options> = {}) {
  const commands = {
    onRetryTask: vi.fn<Options["onRetryTask"]>(async () => COMPLETED),
    onRetryDelivery: vi.fn<Options["onRetryDelivery"]>(async () => COMPLETED),
    onRecoverTaskRun: vi.fn<Options["onRecoverTaskRun"]>(async () => COMPLETED),
    reconcileHistory: vi.fn(async () => undefined), refresh: vi.fn(async () => []),
  };
  let options: Options = { ...commands, scopeKey: "owner-1", task: TASK, ...overrides };
  let locale: "zh" | "en" = "en";
  function Wrapper({ children }: { children: ReactNode }) {
    const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
    return <StrictMode><I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider></StrictMode>;
  }
  const hook = renderHook(useScheduledTaskRunHistoryActions, { initialProps: options, wrapper: Wrapper });
  return {
    ...hook, commands,
    rerender(patch: Partial<Options> = {}, nextLocale = locale) {
      options = { ...options, ...patch }; locale = nextLocale; hook.rerender(options);
    },
  };
}

beforeEach(() => { vi.mocked(writeTextToClipboard).mockReset().mockResolvedValue(true); });
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

describe("history command feedback", () => {
  it("uses the current language before and after completion without replaying the action", async () => {
    const request = pending<ScheduledTaskRunHistoryActionResult>();
    const hook = mount();
    hook.commands.onRetryTask.mockReturnValueOnce(request.promise);
    let operation!: Promise<void>;
    act(() => { operation = hook.result.current.retry(RUN); });
    hook.rerender({}, "zh");
    await act(async () => { request.resolve(COMPLETED); await operation; });
    expect(hook.result.current.feedback?.title).toBe("已触发重新运行");
    hook.rerender({}, "en");
    expect(hook.result.current.feedback?.title).toBe("Run requested");
    expect(hook.commands.onRetryTask).toHaveBeenCalledExactlyOnceWith(TASK, hook.commands.reconcileHistory);
    expect(hook.commands.refresh).toHaveBeenCalledOnce();
  });

  it("keeps a submitted action successful when only the subsequent history read fails", async () => {
    const hook = mount();
    hook.commands.refresh.mockRejectedValueOnce(new Error("private-job-id: read failed"));
    await act(() => hook.result.current.retry(RUN));
    expect(hook.result.current.feedback).toMatchObject({ tone: "warning", title: "Action submitted; history not refreshed" });
    expect(hook.result.current.feedback?.impact).toContain("do not repeat");
    hook.rerender({}, "zh");
    expect(hook.result.current.feedback?.title).toBe("操作已提交，历史尚未刷新");
    expect(JSON.stringify(hook.result.current.feedback)).not.toContain("private-job-id");
    expect(hook.commands.onRetryTask).toHaveBeenCalledOnce();
    expect(hook.commands.refresh).toHaveBeenCalledOnce();
  });

  it.each([
    { effect: "not_applied", title: "Run could not be restarted", tone: "error" },
    { effect: "accepted", title: "Request received; result not yet confirmed", tone: "warning" },
    { effect: "committed", title: "Change completed; state needs checking", tone: "warning" },
    { effect: "unknown", title: "Action result needs confirmation", tone: "warning" },
    { effect: "future_effect", title: "Action result needs confirmation", tone: "warning" },
  ])("projects $effect without exposing diagnostic details or repeating the command", async ({ effect, title, tone }) => {
    const hook = mount();
    hook.commands.onRetryTask.mockRejectedValueOnce(new ApiRequestError("private-agent-id", 500, {
      version: 1, code: "automation.failed", category: "conflict", effect,
    }));
    await act(() => hook.result.current.retry(RUN));
    expect(hook.result.current.feedback).toMatchObject({ title, tone });
    hook.rerender({}, "zh");
    expect(hook.result.current.feedback?.title).not.toBe(title);
    expect(JSON.stringify(hook.result.current.feedback)).not.toContain("private-agent-id");
    expect(hook.commands.onRetryTask).toHaveBeenCalledOnce();
    expect(hook.commands.refresh).not.toHaveBeenCalled();
    expect(hook.result.current.pending.get("retry")?.size ?? 0).toBe(0);
  });

  it("cleans up a synchronous throw so a later explicit action is not swallowed", async () => {
    const hook = mount();
    hook.commands.onRetryTask.mockImplementationOnce(() => { throw new Error("sync failure"); });
    await act(() => hook.result.current.retry(RUN));
    expect(hook.result.current.feedback?.title).toBe("Action result needs confirmation");
    await act(() => hook.result.current.retry(RUN));
    expect(hook.commands.onRetryTask).toHaveBeenCalledTimes(2);
    expect(hook.result.current.feedback?.title).toBe("Run requested");
  });

  it.each(["job", "owner", "closed"] as const)("isolates old commands after leaving and returning to the same %s scope", async (kind) => {
    const old = pending<ScheduledTaskRunHistoryActionResult>();
    const current = pending<ScheduledTaskRunHistoryActionResult>();
    const hook = mount();
    hook.commands.onRetryTask.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
    const staleRetry = hook.result.current.retry;
    let oldOperation!: Promise<void>;
    await act(async () => { oldOperation = staleRetry(RUN); });
    hook.rerender(kind === "job" ? { task: { ...TASK, job_id: "job-2" } } : kind === "owner" ? { scopeKey: "owner-2" } : { task: null });
    hook.rerender({ task: TASK, scopeKey: "owner-1" });
    let operation!: Promise<void>;
    await act(async () => { operation = hook.result.current.retry(RUN); });
    await act(async () => { old.resolve(COMPLETED); await oldOperation; });
    expect(hook.result.current.feedback).toBeNull();
    expect(hook.commands.refresh).not.toHaveBeenCalled();
    expect(hook.result.current.pending.get("retry")?.has(RUN.run_id)).toBe(true);
    act(() => { expect(hook.result.current.retry(RUN)).toBe(operation); });
    await act(() => staleRetry(RUN));
    expect(hook.commands.onRetryTask).toHaveBeenCalledTimes(2);
    await act(async () => { current.resolve(COMPLETED); await operation; });
    expect(hook.result.current.feedback?.title).toBe("Run requested");
    expect(hook.commands.refresh).toHaveBeenCalledOnce();
  });

  it("lets a newer copy own feedback while an older command settles independently", async () => {
    const old = pending<ScheduledTaskRunHistoryActionResult>();
    const hook = mount();
    hook.commands.onRetryTask.mockReturnValueOnce(old.promise);
    let operation!: Promise<void>;
    await act(async () => { operation = hook.result.current.retry(RUN); });
    await act(() => hook.result.current.copyDiagnostic({ ...RUN, run_id: "run-2" }));
    await act(async () => { old.reject(new Error("late-private-detail")); await operation; });
    expect(hook.result.current.feedback?.title).toBe("Diagnostics copied");
    expect(hook.result.current.copiedRunId).toBe("run-2");
    expect(hook.result.current.pending.get("retry")?.size ?? 0).toBe(0);
    expect(hook.commands.refresh).not.toHaveBeenCalled();
  });

  it("localizes clipboard failures and discards an older copy after reopening", async () => {
    const old = pending<boolean>();
    const hook = mount();
    vi.mocked(writeTextToClipboard).mockReturnValueOnce(old.promise);
    let operation!: Promise<void>;
    act(() => { operation = hook.result.current.copyDiagnostic(RUN); });
    hook.rerender({ task: null });
    hook.rerender({ task: TASK });
    vi.mocked(writeTextToClipboard).mockResolvedValueOnce(false);
    await act(() => hook.result.current.copyDiagnostic({ ...RUN, run_id: "run-2" }));
    await act(async () => { old.resolve(true); await operation; });
    expect(hook.result.current.copiedRunId).toBeNull();
    expect(hook.result.current.feedback?.title).toBe("Diagnostics could not be copied");
    hook.rerender({}, "zh");
    expect(hook.result.current.feedback?.title).toBe("无法复制诊断信息");
    expect(writeTextToClipboard).toHaveBeenCalledTimes(2);
  });

  it.each(["deleting", "review_required"] as const)("blocks mutation during %s with current-language guidance", async (deletion_state) => {
    const hook = mount({ task: { ...TASK, deletion_state } });
    await act(() => hook.result.current.retry(RUN));
    await act(() => hook.result.current.retryDelivery(RUN));
    await act(() => hook.result.current.recover(RUN));
    expect(hook.result.current.feedback?.title).toBe("Action not performed");
    expect(hook.result.current.feedback?.nextStep).toContain(deletion_state === "review_required" ? "confirm the original execution" : "Wait for deletion");
    hook.rerender({}, "zh");
    expect(hook.result.current.feedback?.title).toBe("操作未执行");
    expect(hook.commands.onRetryTask).not.toHaveBeenCalled();
    expect(hook.commands.onRetryDelivery).not.toHaveBeenCalled();
    expect(hook.commands.onRecoverTaskRun).not.toHaveBeenCalled();
  });

  it("ignores foreign runs, missing owner scope and callbacks after unmount", async () => {
    const hook = mount();
    const foreign = { ...RUN, job_id: "other-job" };
    await act(() => hook.result.current.retry(foreign));
    await act(() => hook.result.current.copyDiagnostic(foreign));
    hook.rerender({ scopeKey: null });
    await act(() => hook.result.current.retry(RUN));
    const retry = hook.result.current.retry;
    hook.unmount();
    await retry(RUN);
    expect(hook.commands.onRetryTask).not.toHaveBeenCalled();
    expect(writeTextToClipboard).not.toHaveBeenCalled();
  });
});
