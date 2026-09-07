// INPUT: 失败运行、所属任务与诊断/重跑动作。
// OUTPUT: 证明历史行共享外观/动作，文件归属绑定历史执行且迟到反馈不能跨 run。
// POS: Scheduled 运行历史 DOM 合同；可执行动作集合归 history model。

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
    const view = (sessionKey: string | null) => (
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
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
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

const ARTIFACT_RUN: ScheduledTaskRunItem = {
  ...RUN, status: "succeeded", error_message: null, artifact_path: "outputs/report.md",
  session_key: "agent:run-agent:automation:dm:scheduled-task:task-1:run-1",
};

function artifactActions(run: ScheduledTaskRunItem, task = TASK, locale: "zh" | "en" = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>
    <ScheduledTaskRunActions run={run} task={task} isRecovering={false} isRecoveryUnconfirmed={false}
      isRetrying={false} isRetryUnconfirmed={false} isRetryingDelivery={false} isRetryDeliveryUnconfirmed={false}
      onRecover={vi.fn()} onRetry={vi.fn()} onRetryDelivery={vi.fn()} />
  </I18N_CONTEXT.Provider>;
}

describe("historical artifact actions", () => {
  it("distinguishes current task configuration from historical execution in copied diagnostics", () => {
    const diagnostic = buildRunDiagnostic(TASK, ARTIFACT_RUN);
    expect(diagnostic).toContain("Current Task Agent ID: agent-1\n");
    expect(diagnostic).toContain("Run Agent ID: run-agent\n");
    const unknown = buildRunDiagnostic(TASK, { ...ARTIFACT_RUN, session_key: null });
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
