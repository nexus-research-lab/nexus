// INPUT: Real Execution canvas and isolated local graph fixtures.
// OUTPUT: Exact node/edge selection and independent file action regression evidence.
// POS: DOM integration tests; browser tests separately verify inverse zoom and opaque inspector geometry.

import { fireEvent, render, screen, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { ExecutionView, ExecutionWorkItemView } from "@/types/conversation/execution";
import { ExecutionWorkGraphCanvas } from "./execution-workgraph-canvas";
import { I18nProvider } from "@/shared/i18n/i18n-provider";

const item: ExecutionWorkItemView = {
  id: "item", logical_key: "item", kind: "produce", subject: "Report",
  objective: "Create report", deliverable: "report.md", required: true,
  position: 0, status: "running", owner_agent_id: "current-owner", updated_at: "2026-09-05",
  submission: { id: "submission", assignment_id: "new", attempt_id: "new", submitter_agent_id: "author", result_summary: "Done", result_refs: ["report.md"], created_at: "2026-09-05" },
  attempts: ["old", "new"].map((id) => ({
    id, assignment_id: id, executor_kind: "agent", executor_agent_id: "author",
    agent_round_id: id, status: "succeeded", created_at: "2026-09-05",
  })),
};
const execution: ExecutionView = {
  id: "execution", session_key: "dm:test", scope_kind: "dm", status: "active", version: 1,
  objective: "Report", created_at: "2026-09-05", updated_at: "2026-09-05",
  progress: { total: 1, required: 1, accepted: 0, running: 1, blocked: 0, submitted: 0, ready: 0, waiting: 0, changes_requested: 0, failed: 0, cancelled: 0 },
  work_items: [item],
  graph: { nodes: [], edges: [], runtime_node_total: 0, runtime_edge_total: 0, runtime_nodes_truncated: false, runtime_edges_truncated: false },
};
function renderCanvas(value: ExecutionView, onOpenWorkspaceFile = vi.fn()) {
  return render(<I18nProvider><ExecutionWorkGraphCanvas currentId={null} directory={{}}
    execution={value} onOpenWorkspaceFile={onOpenWorkspaceFile} taskRuns={["old", "new"].map((id) => ({
      agentId: "author", agentRoundId: id, latestTaskEventIndex: 0,
      todos: [{ content: `${id} task`, status: "pending" }],
    }))} /></I18nProvider>);
}

it("retains legacy work-item selection when no graph nodes are present", () => {
  // Legacy persisted data can predate the now-required graph field.
  const legacyExecution = { ...execution, graph: undefined } as unknown as ExecutionView;
  const { container } = renderCanvas(legacyExecution);
  fireEvent.click(container.querySelector('[data-execution-graph-node-id="item"]')!);
  expect(screen.getByRole("complementary").textContent).toContain("Report");
});

it("uses the selected historical attempt's task run instead of the latest attempt", () => {
  const { container } = renderCanvas({ ...execution, graph: {
    runtime_node_total: 1, runtime_edge_total: 0, runtime_nodes_truncated: false, runtime_edges_truncated: false,
    nodes: [{ id: "historical", kind: "agent", visibility: "primary", work_item_id: "item", attempt_id: "old", agent_id: "author", position: 0 }], edges: [],
  } });
  fireEvent.click(container.querySelector('[data-execution-graph-node-id="historical"]')!);
  expect(screen.getByRole("complementary").textContent).toContain("old task");
  expect(screen.getByRole("complementary").textContent).not.toContain("new task");
});

it("opens a subagent output in the actual node workspace, not its synthetic avatar identity", () => {
  const open = vi.fn();
  const { container } = renderCanvas({ ...execution, graph: {
    runtime_node_total: 1, runtime_edge_total: 0, runtime_nodes_truncated: false, runtime_edges_truncated: false,
    nodes: [{ id: "child", kind: "subagent", visibility: "primary", subject_id: "task", work_item_id: "item", agent_id: "author", position: 0 }], edges: [],
  } }, open);
  fireEvent.click(container.querySelector('[data-execution-graph-node-id="child"]')!);
  fireEvent.click(within(screen.getByRole("complementary")).getByRole("button", { name: /report.md/ }));
  expect(open).toHaveBeenCalledWith("report.md", "author");
});

it("cancels native modified scrolling while preserving ordinary viewport scroll", () => {
  const { container } = renderCanvas(execution);
  const viewport = container.querySelector('[data-execution-board-grid]')!;
  const modified = new WheelEvent("wheel", { bubbles: true, cancelable: true, ctrlKey: true, deltaY: 30 });
  fireEvent(viewport, modified);
  expect(modified.defaultPrevented).toBe(true);
  const ordinary = new WheelEvent("wheel", { bubbles: true, cancelable: true, deltaY: 30 });
  fireEvent(viewport, ordinary);
  expect(ordinary.defaultPrevented).toBe(false);
});
