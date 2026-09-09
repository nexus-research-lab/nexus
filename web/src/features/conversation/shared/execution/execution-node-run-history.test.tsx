// INPUT: 多次 NodeRun、结构化交付引用与 Workspace 打开命令。
// OUTPUT: 证明可读运行状态、统一时间边界、展开保持、共享错误和精确文件引用。
// POS: Execution 节点运行历史 DOM 合同；路径安全规则仍归 interaction model。

import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useAgentStore } from "@/store/agent";
import type {
  ExecutionGraphNodeView,
  ExecutionWorkItemView,
} from "@/types/conversation/execution";

import { ExecutionNodeRunHistory } from "./execution-node-run-history";

const NODE: ExecutionGraphNodeView = {
  id: "tool-node",
  kind: "tool",
  position: 0,
  runs: [
    { duration_ms: 80, id: "run-1", status: "failed" },
    { duration_ms: 1_250, id: "run-2", result_summary: "完成", status: "succeeded" },
  ],
  visibility: "primary",
  work_item_id: "work-1",
};

const ITEM: ExecutionWorkItemView = {
  deliverable: "报告",
  id: "work-1",
  kind: "produce",
  logical_key: "report",
  objective: "生成报告",
  position: 0,
  required: true,
  status: "accepted",
  subject: "报告",
  submission: {
    assignment_id: "assignment-1",
    attempt_id: "attempt-1",
    created_at: "2026-09-04T00:00:00Z",
    evidence: ["https://example.com/report"],
    id: "submission-1",
    result_refs: ["output/report.md"],
    result_summary: "完成",
    submitter_agent_id: "agent-1",
  },
  updated_at: "2026-09-04T00:00:00Z",
};

function history(node: ExecutionGraphNodeView, locale: I18nContextValue["locale"] = "zh", onOpenWorkspaceFile = vi.fn()) {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return (
    <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>
      <ExecutionNodeRunHistory item={null} node={node} onOpenWorkspaceFile={onOpenWorkspaceFile} workspaceAgentId="fallback-agent" />
    </I18N_CONTEXT.Provider>
  );
}

describe("ExecutionNodeRunHistory", () => {
  it("shares disclosure and button chrome without weakening reference safety", async () => {
    const onOpenWorkspaceFile = vi.fn();
    const user = userEvent.setup();
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <ExecutionNodeRunHistory
          item={ITEM}
          node={NODE}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId="agent-1"
        />
      </I18N_CONTEXT.Provider>,
    );

    const runs = container.querySelectorAll("[data-execution-node-run]");
    expect(runs).toHaveLength(2);
    expect((runs[0] as HTMLDetailsElement).open).toBe(false);
    expect((runs[1] as HTMLDetailsElement).open).toBe(true);
    expect(runs[1].querySelector("summary")?.className).toContain("ui-type-supporting");

    const safeReference = screen.getByTitle("output/report.md");
    expect(safeReference.className).toContain("radius-control-xs");
    await user.click(safeReference);
    expect(onOpenWorkspaceFile).toHaveBeenCalledWith("output/report.md", "agent-1");

    expect(screen.getByTitle("https://example.com/report").hasAttribute("disabled"))
      .toBe(true);
  });

  it.each([undefined, "", "future-state/private-id", "constructor", "__proto__", "toString"])("does not expose an internal identity for an unavailable status: %s", (status) => {
    const { container, rerender } = render(history({ ...NODE, runs: [{ id: "private-run-id", status }] }));
    const detail = container.querySelector<HTMLDetailsElement>("details")!;
    expect(detail.open).toBe(true);
    expect(detail.getAttribute("data-execution-node-run")).toBe("private-run-id");
    expect(detail.querySelector("summary")?.textContent).toContain("运行状态未知");
    expect(detail.textContent).toContain("暂无可展示的运行详情。");
    expect(detail.textContent).not.toContain("private-run-id");
    if (status) expect(detail.textContent).not.toContain(status);

    rerender(history({ ...NODE, runs: [{ id: "private-run-id", status }] }, "en"));
    expect(detail.querySelector("summary")?.textContent).toContain("Status unavailable");
    expect(detail.textContent).toContain("No run details are available.");
  });

  it.each([
    [0, "0 毫秒", "0ms"],
    [80, "80 毫秒", "80ms"],
    [999.9, "1.0 秒", "1.0s"],
    [1_250, "1.3 秒", "1.3s"],
    [59_999, "1 分 0 秒", "1m 0s"],
    [119_999, "2 分 0 秒", "2m 0s"],
  ] as const)("localizes %s milliseconds with valid unit carry", (duration_ms, zh, en) => {
    const node = { ...NODE, runs: [{ id: "run-time", status: "succeeded", duration_ms }] };
    const { rerender } = render(history(node));
    expect(screen.getByText(zh)).toBeTruthy();
    rerender(history(node, "en"));
    expect(screen.getByText(en)).toBeTruthy();
  });

  it.each([undefined, -1, Number.NaN, Number.POSITIVE_INFINITY, Number.MAX_VALUE])("falls back from invalid duration %s to a valid observation time in the current language", (duration_ms) => {
    const started_at = "2026-09-07T07:05:00Z";
    const node = { ...NODE, runs: [{ id: "run-time", duration_ms, finished_at: "invalid", started_at }] };
    const { container, rerender } = render(history(node));
    const date = new Date(started_at);
    expect(screen.getByText(new Intl.DateTimeFormat("zh", { hour: "2-digit", minute: "2-digit" }).format(date))).toBeTruthy();
    rerender(history(node, "en"));
    expect(screen.getByText(new Intl.DateTimeFormat("en", { hour: "2-digit", minute: "2-digit" }).format(date))).toBeTruthy();
    expect(container.textContent).not.toMatch(/NaN|Infinity|invalid/);
  });

  it("omits invalid timestamps instead of presenting them as run details", () => {
    const { container } = render(history({ ...NODE, runs: [{ id: "run-time", started_at: "invalid-start", finished_at: "invalid-finish" }] }));
    expect(container.textContent).not.toMatch(/invalid-start|invalid-finish|run-time/);
  });

  it("preserves chosen expansion across status, language and history updates", async () => {
    const user = userEvent.setup();
    const { container, rerender } = render(history(NODE));
    const details = container.querySelectorAll<HTMLDetailsElement>("details");
    await user.click(details[0].querySelector("summary")!);
    await user.click(details[1].querySelector("summary")!);
    expect(details[0].open).toBe(true);
    expect(details[1].open).toBe(false);
    rerender(history({
      ...NODE,
      runs: [...NODE.runs!, { id: "run-3", status: "running" }],
    }, "en"));
    expect(details[0].open).toBe(true);
    expect(details[1].open).toBe(false);
    expect(container.querySelector<HTMLDetailsElement>('[data-execution-node-run="run-3"]')?.open).toBe(true);
    expect(details[0].querySelector("summary")?.textContent).toContain("Failed");
  });

  it("uses static shared error details, preserving line breaks and code-only evidence", () => {
    const { container, rerender } = render(history({
      ...NODE,
      runs: [{ id: "run-error", status: "failed", error_summary: "First line\n<script>never execute</script>", error_code: "TOOL_FAILED", result_summary: "Result\ncontinues" }],
    }));
    const note = screen.getByRole("note");
    expect(note.getAttribute("aria-live")).toBe("off");
    expect(note.getAttribute("data-inline-notice-tone")).toBe("warning");
    expect(note.textContent).toContain("First line\n<script>never execute</script>");
    expect(container.querySelector("script")).toBeNull();
    expect(within(note).getByText("TOOL_FAILED")).toBeTruthy();
    expect(screen.getByText("Result continues").className).toContain("whitespace-pre-wrap");

    rerender(history({ ...NODE, runs: [{ id: "run-error", status: "failed", error_code: "CODE_WITHOUT_SUMMARY" }] }));
    expect(screen.getByRole("note").textContent).toBe("CODE_WITHOUT_SUMMARY");
    expect(screen.queryByText("暂无可展示的运行详情。")).toBeNull();
  });

  it("preserves the structured artifact owner ahead of the node workspace fallback", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();
    render(history({
      ...NODE,
      runs: [{
        id: "run-artifacts", status: "succeeded",
        artifacts: [{ type: "workspace_file_artifact", path: "output/result.md", workspace_agent_id: "artifact-owner" }],
      }],
    }, "en", onOpen));
    await user.click(screen.getByTitle("output/result.md"));
    expect(onOpen).toHaveBeenCalledWith("output/result.md", "artifact-owner");
  });

  it("uses a known node workspace for legacy artifacts and disables opening when its source is unavailable", async () => {
    const onOpen = vi.fn();
    const node: ExecutionGraphNodeView = { ...NODE, runs: [{ id: "legacy-run", artifacts: [{ type: "workspace_file_artifact", path: "output/legacy.md" }] }] };
    const { rerender } = render(history(node, "en", onOpen));
    const user = userEvent.setup();
    await user.click(screen.getByTitle("output/legacy.md"));
    expect(onOpen).toHaveBeenCalledExactlyOnceWith("output/legacy.md", "fallback-agent");
    act(() => useAgentStore.setState({ current_agent_id: "unrelated-viewer" }));
    try {
      rerender(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => MESSAGES.en[key] }}><ExecutionNodeRunHistory item={null} node={node} onOpenWorkspaceFile={onOpen} workspaceAgentId={null} /></I18N_CONTEXT.Provider>);
      const button = screen.getByTitle("output/legacy.md");
      expect(button.hasAttribute("disabled")).toBe(true);
      await user.click(button);
      expect(onOpen).toHaveBeenCalledOnce();
      expect(screen.queryByRole("button", { name: /Download/ })).toBeNull();
      expect(screen.getByText("The source workspace is unavailable, so this file cannot be opened.")).toBeTruthy();
    } finally {
      act(() => useAgentStore.setState({ current_agent_id: null }));
    }
  });
  it("does not open plain output references without the source workspace owner", async () => {
    const open = vi.fn();
    render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
      <ExecutionNodeRunHistory item={ITEM} node={NODE} onOpenWorkspaceFile={open} />
    </I18N_CONTEXT.Provider>);
    const button = screen.getByRole("button", { name: "output/report.md" }) as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    await userEvent.setup().click(button);
    expect(open).not.toHaveBeenCalled();
  });
});
