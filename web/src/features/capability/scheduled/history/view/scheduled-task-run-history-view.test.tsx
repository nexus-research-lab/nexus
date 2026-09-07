// INPUT: 失败运行、所属任务与诊断/重跑动作。
// OUTPUT: 证明历史行共享外观/动作，文件归属绑定历史执行且迟到反馈不能跨 run。
// POS: Scheduled 运行历史 DOM 合同；可执行动作集合归 history model。

import type { ComponentProps, ReactNode } from "react";

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { useAgentStore } from "@/store/agent";
import type { ScheduledTaskRunItem } from "@/types/capability/scheduled-task/run";
import type { ScheduledTaskItem } from "@/types/capability/scheduled-task/task";

import { ScheduledTaskRunHistoryItem } from "./scheduled-task-run-history-item";
import { ScheduledTaskRunDetails } from "./scheduled-task-run-details";
import { ScheduledTaskRunActions } from "./scheduled-task-run-actions";
import { buildRunDiagnostic } from "../scheduled-task-run-diagnostic-model";
import { formatDuration, getDeliveryStatusMeta, getStatusMeta } from "../scheduled-task-run-history-model";
import { formatScheduledDatetime } from "../../scheduled-formatters";

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

describe("ScheduledTaskRunHistoryItem", () => {
  it("uses the historical execution Session for images and leaves unknown scopes unbound", () => {
    useAgentStore.setState({ current_agent_id: "selected-agent" });
    const run = { ...RUN, result_text: "![Chart](images/chart.png)", session_key: "agent:run-agent:automation:dm:scheduled-task:task-1:run-1" };
    const view = (sessionKey: string | null) => localized(
      <ScheduledTaskRunDetails isCopied={false} onCopyDiagnostic={vi.fn()} run={{ ...run, session_key: sessionKey }} />
    );
    const { rerender } = render(view(run.session_key));
    expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toContain("/agents/run-agent/workspace/download?");

    act(() => useAgentStore.setState({ current_agent_id: "other-agent" }));
    expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toContain("/agents/run-agent/workspace/download?");
    for (const unknownSession of [null, "agent:malformed", "room:group:conversation-one"]) {
      rerender(view(unknownSession));
      expect(screen.getByRole("img", { name: "Chart" }).getAttribute("src")).toBe("images/chart.png");
    }
  });

  it("uses shared history chrome and preserves diagnostic and retry actions", async () => {
    const onCopyDiagnostic = vi.fn();
    const onRetry = vi.fn();
    const user = userEvent.setup();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={language()}
      >
        <ScheduledTaskRunHistoryItem
          defaultOpen
          isCopied={false}
          isRecovering={false}
          isRecoveryUnconfirmed={false}
          isRetryDeliveryUnconfirmed={false}
          isRetryUnconfirmed={false}
          isRetrying={false}
          isRetryingDelivery={false}
          onCopyDiagnostic={onCopyDiagnostic}
          onRecover={vi.fn()}
          onRetry={onRetry}
          onRetryDelivery={vi.fn()}
          run={RUN}
          task={TASK}
        />
      </I18N_CONTEXT.Provider>,
    );

    expect(container.querySelector("summary.radius-control-md")).toBeTruthy();
    expect(screen.getByText("诊断详情").closest("summary")?.className)
      .toContain("ui-type-caption");
    expect(container.querySelectorAll("section.surface-radius-sm").length).toBeGreaterThan(0);

    const copyButton = screen.getByRole("button", { name: "复制诊断" });
    expect(copyButton.className).toContain("radius-control-xs");
    await user.click(copyButton);
    expect(onCopyDiagnostic).toHaveBeenCalledWith(RUN);

    const retryButton = screen.getByRole("button", { name: "重新运行" });
    expect(retryButton.className).toContain("focus-visible:ring-2");
    await user.click(retryButton);
    expect(onRetry).toHaveBeenCalledWith(RUN);
  });
});

describe("run history language and command states", () => {
  function actionView(overrides: Partial<ComponentProps<typeof ScheduledTaskRunActions>> = {}, locale: "zh" | "en" = "en") {
    return localized(<ScheduledTaskRunActions
      run={RUN} task={TASK} isRecovering={false} isRecoveryUnconfirmed={false}
      isRetrying={false} isRetryUnconfirmed={false} isRetryingDelivery={false} isRetryDeliveryUnconfirmed={false}
      onRecover={vi.fn()} onRetry={vi.fn()} onRetryDelivery={vi.fn()} {...overrides}
    />, locale);
  }

  it("updates open history, duration, delivery errors and copy controls without resetting disclosure", () => {
    const onCopyDiagnostic = vi.fn();
    const run = { ...RUN, delivery_status: "failed", delivery_error: "destination unavailable" };
    const view = (locale: "zh" | "en") => localized(<ScheduledTaskRunHistoryItem
      defaultOpen isCopied={false} run={run} task={TASK}
      isRecovering={false} isRecoveryUnconfirmed={false} isRetrying={false} isRetryUnconfirmed={false}
      isRetryingDelivery={false} isRetryDeliveryUnconfirmed={false}
      onCopyDiagnostic={onCopyDiagnostic} onRecover={vi.fn()} onRetry={vi.fn()} onRetryDelivery={vi.fn()}
    />, locale);
    const { rerender } = render(view("zh"));
    const diagnostic = screen.getByText("诊断详情").closest("details")!;
    fireEvent.click(screen.getByText("诊断详情"));
    expect(diagnostic.open).toBe(true);
    expect(screen.getByText("59 秒")).toBeTruthy();
    rerender(view("en"));
    expect(screen.getByText("Diagnostics").closest("details")).toBe(diagnostic);
    expect(diagnostic.open).toBe(true);
    expect(screen.getByText("59s")).toBeTruthy();
    expect(screen.getByText("Failed")).toBeTruthy();
    expect(screen.getByText("Delivery failed: destination unavailable")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Copy diagnostics" }));
    expect(onCopyDiagnostic).toHaveBeenCalledExactlyOnceWith(run);
    expect(screen.getByRole("button", { name: "Run again" }).getAttribute("title"))
      .toBe("Run once with the current task configuration.");
    expect(screen.queryByText("59 秒")).toBeNull();
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

  it("accepts zero timestamps and falls back for missing or invalid dates without crashing history", () => {
    const { t } = language("en");
    expect(formatDuration(0, 0, t)).toBe("0s");
    expect(formatDuration(0, 61_000, t)).toBe("1m 1s");
    expect(formatDuration(2_000, 1_000, t)).toBe("0s");
    expect(formatDuration(null, 1_000, t)).toBe("Unfinished");
    expect(formatDuration(1_000, NaN, t)).toBe("Unfinished");
    for (const invalid of [null, NaN, Infinity, 9e15]) {
      expect(formatScheduledDatetime(invalid, { locale: "en", emptyLabel: "Not recorded" })).toBe("Not recorded");
    }
    for (const locale of ["zh", "en"] as const) {
      const formatted = formatScheduledDatetime(0, { locale, includeSeconds: true });
      expect(formatted).toBe(new Intl.DateTimeFormat(locale, { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(0));
    }
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
  it("distinguishes current task configuration from historical execution in copied diagnostics", () => {
    const diagnostic = buildRunDiagnostic(TASK, ARTIFACT_RUN, language());
    expect(diagnostic).toContain("Current Task Agent ID: agent-1\n");
    expect(diagnostic).toContain("Run Agent ID: run-agent\n");
    const unknown = buildRunDiagnostic(TASK, { ...ARTIFACT_RUN, session_key: null }, language());
    expect(unknown).toContain("Run Agent ID: \n");
    expect(unknown).not.toContain("Run Agent ID: agent-1");
  });

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

  it("keeps failure feedback in the exact run even when two runs use the same file", async () => {
    let rejectOld!: (error: Error) => void;
    vi.mocked(downloadWorkspaceFileApi).mockReturnValueOnce(new Promise<void>((_resolve, reject) => { rejectOld = reject; }));
    const view = render(artifactActions(ARTIFACT_RUN));
    fireEvent.click(screen.getByRole("button", { name: "Download report.md" }));
    const nextRun = { ...ARTIFACT_RUN, run_id: "run-2" };
    view.rerender(artifactActions(nextRun));
    await act(async () => rejectOld(new Error("old run diagnostic")));
    expect(screen.queryByText(MESSAGES.en["workspace_file.external_action_failed"])).toBeNull();
    expect(console.error).not.toHaveBeenCalled();
    vi.mocked(downloadWorkspaceFileApi).mockRejectedValueOnce(new Error("current native diagnostic"));
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Download report.md" })));
    expect(screen.getByText(MESSAGES.en["workspace_file.external_action_failed"])).toBeTruthy();
    expect(screen.queryByText("current native diagnostic")).toBeNull();
    view.rerender(artifactActions(nextRun, TASK, "zh"));
    expect(screen.getByText(MESSAGES.zh["workspace_file.external_action_failed"])).toBeTruthy();
    expect(downloadWorkspaceFileApi).toHaveBeenCalledTimes(2);
  });
});
