// INPUT: 多次 NodeRun、结构化交付引用与 Workspace 打开命令。
// OUTPUT: 证明最新运行默认展开、安全引用可打开且不安全引用保持禁用。
// POS: Execution 节点运行历史 DOM 合同；路径安全规则仍归 interaction model。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
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
    expect(runs[1].querySelector("summary")?.className).toContain("ui-type-caption");

    const safeReference = screen.getByTitle("output/report.md");
    expect(safeReference.className).toContain("radius-control-xs");
    await user.click(safeReference);
    expect(onOpenWorkspaceFile).toHaveBeenCalledWith("output/report.md", "agent-1");

    expect(screen.getByTitle("https://example.com/report").hasAttribute("disabled"))
      .toBe(true);
  });
});
