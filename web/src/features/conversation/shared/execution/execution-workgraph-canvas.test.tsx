// INPUT: Real Execution canvas and isolated local graph fixtures.
// OUTPUT: Exact node/edge selection and independent file action regression evidence.
// POS: DOM integration tests; pure geometry tests cover placement, with browser rendering checked separately.

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type { ExecutionGraphNodeDetailView, ExecutionView, ExecutionWorkItemView } from "@/types/conversation/execution";
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


it("keeps retry selection exact from the line and keyboard target, and closes it with Escape", () => {
  const view: ExecutionView = { ...execution, work_items: [], graph: {
    runtime_node_total: 3, runtime_edge_total: 2, runtime_nodes_truncated: false, runtime_edges_truncated: false,
    nodes: ["first", "second", "third"].map((id, position) => ({
      id, kind: "tool", visibility: "primary", work_item_id: "", name: `${id} run`, position,
    })),
    edges: ["second", "third"].map((id, position) => ({
      id: `retry:${id}`, kind: "retry", source_node_id: position === 0 ? "first" : "second", target_node_id: id,
    })),
  } };
  const { container } = renderCanvas(view);
  fireEvent.click(container.querySelector('[data-execution-edge-line-hit="retry:second"]')!);
  let details = screen.getByRole("complementary");
  expect(details.getAttribute("data-execution-selected-edge-detail")).toBe("retry:second");
  expect(details.textContent).toContain("first run");
  expect(details.textContent).toContain("second run");
  expect(details.textContent).not.toContain("third run");
  const target = container.querySelector('[data-execution-edge-hit-target="retry:third"]')!;
  fireEvent.keyDown(target, { key: "Enter" });
  details = screen.getByRole("complementary");
  expect(details.getAttribute("data-execution-selected-edge-detail")).toBe("retry:third");
  expect(details.textContent).not.toContain("first run");
  expect(details.textContent).toContain("third run");
  fireEvent.keyDown(container.querySelector('[data-execution-board-grid]')!, { key: "Escape" });
  expect(screen.queryByRole("complementary")).toBeNull();
});


it("offers retry on a submitted task selected from a completed graph", () => {
  const run = vi.fn();
  const node = { id: "item", kind: "agent" as const, visibility: "primary" as const, work_item_id: "item", attempt_id: "new", agent_id: "author", position: 0, can_retry: true, retry_downstream: true };
  const value: ExecutionView = { ...execution, status: "completed", graph: { ...execution.graph!, nodes: [node] } };
  const { container } = render(<I18nProvider><ExecutionWorkGraphCanvas currentId={null} directory={{}} execution={value} taskRuns={[]} recovery={{ run, states: {} }} /></I18nProvider>);
  fireEvent.click(container.querySelector('[data-execution-graph-node-id="item"]')!);
  fireEvent.click(screen.getByRole("button", { name: "Retry this task" }));
  expect(run).toHaveBeenCalledWith(node);
});

const skeleton: ExecutionView = { ...execution, work_items: [], graph: {
  runtime_node_total: 2, runtime_edge_total: 1, runtime_nodes_truncated: false, runtime_edges_truncated: false,
  nodes: [
    { id: "lead", kind: "agent", visibility: "primary", work_item_id: "", agent_id: "author", name: "Lead", lifecycle_status: "failed", run_status: "failed", position: 0 },
    { id: "fetch", kind: "tool", visibility: "nested", work_item_id: "", parent_node_id: "lead", name: "WebFetch", position: 1 },
  ],
  edges: [{ id: "invoke", kind: "invoke", source_node_id: "lead", target_node_id: "fetch" }],
} };

function renderSkeletonCanvas(loadNodeDetail: (nodeId: string) => Promise<ExecutionGraphNodeDetailView>) {
  return render(<I18nProvider><ExecutionWorkGraphCanvas currentId={null} directory={{}}
    execution={skeleton} loadNodeDetail={loadNodeDetail} taskRuns={[]} /></I18nProvider>);
}

it("loads run facts and detail children only when a skeleton node is opened", async () => {
  let resolve!: (value: ExecutionGraphNodeDetailView) => void;
  const load = vi.fn(() => new Promise<ExecutionGraphNodeDetailView>((done) => { resolve = done; }));
  const { container } = renderSkeletonCanvas(load);
  expect(load).not.toHaveBeenCalled();
  fireEvent.click(container.querySelector('[data-execution-graph-node-id="lead"]')!);
  const inspector = screen.getByRole("complementary");
  expect(load).toHaveBeenCalledWith("lead");
  // 骨架没有错误摘要时，加载期间不得闪现“未提供错误摘要”。
  expect(inspector.querySelector("[data-execution-node-detail-loading]")).not.toBeNull();
  expect(inspector.textContent).not.toContain("did not provide a displayable error summary");

  await act(async () => resolve({
    execution_id: "execution",
    node: { ...skeleton.graph!.nodes![0], error_summary: "Upstream timed out" },
    children: [
      { id: "fetch", kind: "tool", visibility: "nested", work_item_id: "", parent_node_id: "lead", name: "WebFetch", position: 1 },
      { id: "read", kind: "tool", visibility: "detail", work_item_id: "", parent_node_id: "lead", name: "Read", result_summary: "Read notes.md", position: 2 },
    ],
  }));
  expect(inspector.querySelector("[data-execution-node-detail-loading]")).toBeNull();
  expect(inspector.textContent).toContain("Upstream timed out");
  expect(inspector.querySelector('[data-execution-runtime-node="read"]')?.textContent).toContain("Read notes.md");
});

it("offers a retry when node detail fails and keeps the structural inspector", async () => {
  const load = vi.fn()
    .mockRejectedValueOnce(new Error("offline"))
    .mockResolvedValueOnce({ execution_id: "execution", node: { ...skeleton.graph!.nodes![0], error_summary: "Recovered summary" } });
  const { container } = renderSkeletonCanvas(load);
  await act(async () => { fireEvent.click(container.querySelector('[data-execution-graph-node-id="lead"]')!); });
  const inspector = screen.getByRole("complementary");
  expect(inspector.querySelector("[data-execution-node-detail-failed]")).not.toBeNull();
  await act(async () => { fireEvent.click(within(inspector).getByRole("button", { name: "Retry" })); });
  expect(load).toHaveBeenCalledTimes(2);
  expect(inspector.textContent).toContain("Recovered summary");
});
