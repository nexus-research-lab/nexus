// INPUT: 失败运行、所属任务与诊断/重跑动作。
// OUTPUT: 证明历史行共享外观/动作，文件归属绑定历史执行且迟到反馈不能跨 run。
// POS: Scheduled 运行历史 DOM 合同；可执行动作集合归 history model。

import type { ComponentProps, ReactNode } from "react";

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { useAgentStore } from "@/store/agent";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { ScheduledTaskRunDetails } from "./scheduled-task-run-details";
import { ScheduledTaskRunActions } from "./scheduled-task-run-actions";
import { buildRunDiagnostic } from "../scheduled-task-run-diagnostic-model";
import { getDeliveryStatusMeta, getStatusMeta } from "../scheduled-task-run-history-model";

vi.mock("@/config/desktop-runtime", async (original) => ({ ...await original<typeof import("@/config/desktop-runtime")>(), isDesktopRuntime: vi.fn(() => false) }));
vi.mock("@/lib/api/agent/agent-api", async (original) => ({ ...await original<typeof import("@/lib/api/agent/agent-api")>(), downloadWorkspaceFileApi: vi.fn(async () => undefined) }));

beforeEach(() => {
  vi.mocked(downloadWorkspaceFileApi).mockReset().mockResolvedValue(undefined);
  vi.mocked(isDesktopRuntime).mockReturnValue(false);
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

afterEach(() => {
  cleanup();
  useAgentStore.setState({ current_agent_id: null });
  vi.restoreAllMocks();
});

function language(locale: "zh" | "en" = "zh"): I18nContextValue {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return { locale, setLocale: vi.fn(), t };
}

function localized(children: ReactNode, locale: "zh" | "en" = "zh") {
  return <I18N_CONTEXT.Provider value={language(locale)}>{children}</I18N_CONTEXT.Provider>;
}

const TASK: ScheduledTaskItem = {
  agent_id: "agent-1",
  configuration_version: 1,
  delivery: { mode: "none" },
  enabled: true,
  expires_at: null,
  failure_streak: 1,
  instruction: "整理今日进展",
  job_id: "task-1",
  last_error: "network timeout",
  last_run_at: 1_735_689_600_000,
  name: "每日工作简报",
  next_run_at: null,
  running: false,
  running_started_at: null,
  schedule: { interval_seconds: 86_400, kind: "every" },
  session_target: { kind: "main" },
  source: { kind: "user_page" },
};

const RUN: ScheduledTaskRunItem = {
  attempts: 1,
  delivered_at: null,
  delivery_dead_letter_at: null,
  delivery_next_attempt_at: null,
  delivery_status: "not_required",
  error_message: "network timeout",
  finished_at: 1_735_689_660_000,
  job_id: TASK.job_id,
  run_id: "run-1",
  scheduled_for: 1_735_689_600_000,
  started_at: 1_735_689_601_000,
  status: "failed",
};

describe("run history language and command states", () => {
  function actionView(overrides: Partial<ComponentProps<typeof ScheduledTaskRunActions>> = {}, locale: "zh" | "en" = "en") {
    return localized(<ScheduledTaskRunActions
      run={RUN} task={TASK} isRecovering={false} isRecoveryUnconfirmed={false}
      isRetrying={false} isRetryUnconfirmed={false} isRetryingDelivery={false} isRetryDeliveryUnconfirmed={false}
      onRecover={vi.fn()} onRetry={vi.fn()} onRetryDelivery={vi.fn()} {...overrides}
    />, locale);
  }

  it("keeps raw run and delivery errors inside diagnostics while preserving the copied report", () => {
    const error = "runtime private-agent-id /private/workspace\n<script>private detail</script>";
    const deliveryError = "delivery private-session-id\nreceiver private-path";
    const run = { ...RUN, error_message: error, delivery_error: deliveryError };
    const { container, rerender } = render(localized(<ScheduledTaskRunDetails run={run} isCopied={false} onCopyDiagnostic={vi.fn()} />, "en"));
    const summary = screen.getByText("This run encountered a problem. View diagnostics for details.").closest("section")!;
    const delivery = screen.getByText("Result delivery failed. Expand diagnostics for details.").closest("section")!;
    expect(summary.textContent).not.toContain("private-");
    expect(delivery.textContent).not.toContain("private-");
    const diagnostics = screen.getByText("Diagnostics").closest("details")!;
    expect(diagnostics.open).toBe(false);
    expect(diagnostics.textContent).toContain(error);
    expect(diagnostics.textContent).toContain(deliveryError);
    expect(container.querySelector("script")).toBeNull();
    expect(buildRunDiagnostic(TASK, run, language("en"))).toContain(`Error:\n${error}`);
    expect(buildRunDiagnostic(TASK, run, language("en"))).toContain(`Delivery Error:\n${deliveryError}`);
    rerender(localized(<ScheduledTaskRunDetails run={run} isCopied={false} onCopyDiagnostic={vi.fn()} />, "zh"));
    expect(screen.getByText("运行遇到问题，可查看诊断了解详情")).toBeTruthy();
    expect(diagnostics.textContent).toContain(error);
  });

  it.each([
    { error: "previous run is still running; overlap_policy=skip", summary: "This schedule was skipped because the previous run is still active", zh: "上一次运行未结束，本次调度已跳过" },
    { error: "Permission request timeout", summary: "Timed out waiting for a permission response", zh: "等待权限响应超时" },
  ])("localizes the known error $error while retaining its technical evidence", ({ error, summary, zh }) => {
    const run = { ...RUN, error_message: error };
    const view = (locale: "zh" | "en") => localized(<ScheduledTaskRunDetails run={run} isCopied={false} onCopyDiagnostic={vi.fn()} />, locale);
    const { rerender } = render(view("en"));
    expect(screen.getByText(summary)).toBeTruthy();
    expect(screen.getByText("Diagnostics").closest("details")?.textContent).toContain(`Technical details: ${error}`);
    rerender(view("zh"));
    expect(screen.getByText(zh)).toBeTruthy();
    expect(screen.getByText("诊断详情").closest("details")?.textContent).toContain(`技术信息：${error}`);
  });

  it.each([
    { busy: "isRetrying", unconfirmed: "isRetryUnconfirmed", callback: "onRetry", pending: "Starting", unknown: "Run unconfirmed", ready: "Run again", run: RUN, task: TASK },
    { busy: "isRetryingDelivery", unconfirmed: "isRetryDeliveryUnconfirmed", callback: "onRetryDelivery", pending: "Delivering", unknown: "Delivery unconfirmed", ready: "Retry delivery", run: { ...RUN, status: "succeeded", delivery_status: "failed" }, task: TASK },
    { busy: "isRecovering", unconfirmed: "isRecoveryUnconfirmed", callback: "onRecover", pending: "Releasing", unknown: "Release unconfirmed", ready: "Release run", run: { ...RUN, status: "running" }, task: { ...TASK, running: true } },
  ] as const)("keeps $callback disabled while pending or unconfirmed, and marks only the request busy", (scenario) => {
    const callback = vi.fn();
    const props = { run: scenario.run, task: scenario.task, [scenario.callback]: callback };
    const { rerender } = render(actionView({ ...props, [scenario.busy]: true }));
    const pending = screen.getByRole("button", { name: scenario.pending }) as HTMLButtonElement;
    expect(pending.disabled).toBe(true);
    expect(pending.getAttribute("aria-busy")).toBe("true");
    fireEvent.click(pending);
    rerender(actionView({ ...props, [scenario.unconfirmed]: true }));
    const unknown = screen.getByRole("button", { name: scenario.unknown }) as HTMLButtonElement;
    expect(unknown.disabled).toBe(true);
    expect(unknown.getAttribute("aria-busy")).toBe("false");
    fireEvent.click(unknown);
    expect(callback).not.toHaveBeenCalled();
    rerender(actionView(props));
    fireEvent.click(screen.getByRole("button", { name: scenario.ready }));
    expect(callback).toHaveBeenCalledOnce();
  });

  it.each(["deleting", "review_required"] as const)("keeps every mutation blocked during %s", (deletion_state) => {
    const callback = vi.fn();
    const props = { task: { ...TASK, deletion_state }, onRetry: callback, onRetryDelivery: callback, onRecover: callback };
    const { rerender } = render(actionView({ ...props, run: { ...RUN, delivery_status: "failed" } }));
    const expectBlocked = () => {
      const buttons = screen.getAllByRole("button") as HTMLButtonElement[];
      expect(buttons).toHaveLength(2);
      for (const button of buttons) {
        expect(button.disabled).toBe(true);
        expect(button.getAttribute("aria-busy")).toBe("false");
        fireEvent.click(button);
      }
      expect(callback).not.toHaveBeenCalled();
    };
    expectBlocked();
    rerender(actionView({ ...props, task: { ...props.task, running: true }, run: { ...RUN, status: "running", delivery_status: "retrying", delivery_attempts: 1 } }));
    expectBlocked();
  });

  it("requires the retained delivery attempt before offering verification", () => {
    const onRetryDelivery = vi.fn();
    const run: ScheduledTaskRunItem = { ...RUN, status: "succeeded", delivery_status: "retrying", delivery_attempts: null };
    const { rerender } = render(actionView({ run, onRetryDelivery }));
    const unavailable = screen.getByRole("button", { name: "Refresh and check" }) as HTMLButtonElement;
    expect(unavailable.disabled).toBe(true);
    fireEvent.click(unavailable);
    expect(onRetryDelivery).not.toHaveBeenCalled();
    rerender(actionView({ run: { ...run, delivery_attempts: 2 }, onRetryDelivery }));
    fireEvent.click(screen.getByRole("button", { name: "Checked; deliver again" }));
    expect(onRetryDelivery).toHaveBeenCalledOnce();
  });

  it.each(["future-private-status", "__proto__", "constructor"])("keeps unknown wire status %s out of labels and action eligibility", (wireStatus) => {
    const run = { ...RUN, status: wireStatus as ScheduledTaskRunItem["status"], delivery_status: wireStatus };
    const { t } = language("en");
    expect(getStatusMeta(run.status, t).label).toBe("Unknown status");
    expect(getDeliveryStatusMeta(run.delivery_status, t)?.label).toBe("Unknown delivery status");
    render(actionView({ run }));
    expect(screen.queryByRole("button")).toBeNull();
  });
});

const ARTIFACT_RUN: ScheduledTaskRunItem = {
  ...RUN, status: "succeeded", error_message: null, artifact_path: "outputs/report.md",
  session_key: "agent:run-agent:automation:dm:scheduled-task:task-1:run-1",
};

function artifactActions(run: ScheduledTaskRunItem, task = TASK, locale: "zh" | "en" = "en") {
  return <I18N_CONTEXT.Provider value={language(locale)}>
    <ScheduledTaskRunActions run={run} task={task} isRecovering={false} isRecoveryUnconfirmed={false}
      isRetrying={false} isRetryUnconfirmed={false} isRetryingDelivery={false} isRetryDeliveryUnconfirmed={false}
      onRecover={vi.fn()} onRetry={vi.fn()} onRetryDelivery={vi.fn()} />
  </I18N_CONTEXT.Provider>;
}

describe("historical artifact actions", () => {

  it.each([false, true])("uses the persisted executor after task rebind, desktop=%s", async (desktop) => {
    vi.mocked(isDesktopRuntime).mockReturnValue(desktop);
    useAgentStore.setState({ current_agent_id: "ambient-agent" });
    const view = render(artifactActions(ARTIFACT_RUN, { ...TASK, agent_id: "new-task-agent" }));
    const name = MESSAGES.en[desktop ? "workspace_file.reveal_named" : "workspace_file.download_named"].replace("{name}", "report.md");
    await act(async () => fireEvent.click(screen.getByRole("button", { name })));
    expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith("run-agent", "outputs/report.md", "report.md");
    view.rerender(artifactActions(ARTIFACT_RUN, { ...TASK, agent_id: "another-task-agent" }));
    await act(async () => fireEvent.click(screen.getByRole("button", { name })));
    expect(vi.mocked(downloadWorkspaceFileApi).mock.calls[1]).toEqual(["run-agent", "outputs/report.md", "report.md"]);
  });

  it.each([{ sessionKey: null }, { sessionKey: "agent:malformed" }, { sessionKey: "room:group:one" }])("keeps unproven historical artifact scopes disabled: %j", async ({ sessionKey }) => {
    render(artifactActions({ ...ARTIFACT_RUN, session_key: sessionKey }, TASK, "zh"));
    const button = screen.getByRole("button", { name: "下载 report.md" });
    expect((button as HTMLButtonElement).disabled).toBe(true);
    expect(document.getElementById(button.getAttribute("aria-describedby")!)?.textContent).toBe(MESSAGES.zh["capability.scheduled_history_artifact_unavailable"]);
    await act(async () => fireEvent.click(button));
    expect(downloadWorkspaceFileApi).not.toHaveBeenCalled();
  });
});
