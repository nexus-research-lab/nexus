// INPUT: 单人/多人、不同状态的 Execution 投影与精确导航动作。
// OUTPUT: 证明活动条保留点击区、可滚动头像与固定图入口，状态精简不改变完整图或导航。
// POS: Execution Process Panel DOM 行为测试；图节点选择规则由 model 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import userEvent from "@testing-library/user-event";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ExecutionView } from "@/types/conversation/execution";

import { ExecutionNodeAvatar } from "./execution-node-avatar";
import { ExecutionProcessPanel } from "./execution-process-panel";

const EXECUTION = {
  created_at: "2026-09-03T00:00:00Z",
  graph: {
    edges: [],
    nodes: [{
      agent_id: "agent-1",
      agent_round_id: "round-1",
      id: "work-1",
      kind: "agent",
      position: 0,
      responsibility_status: "running",
      visibility: "primary",
      work_item_id: "work-1",
    }],
    runtime_edge_total: 0,
    runtime_edges_truncated: false,
    runtime_node_total: 1,
    runtime_nodes_truncated: false,
  },
  id: "execution-1",
  objective: "统一前端活动条",
  progress: {
    accepted: 0,
    blocked: 0,
    cancelled: 0,
    changes_requested: 0,
    failed: 0,
    ready: 0,
    required: 1,
    running: 1,
    submitted: 0,
    total: 1,
    waiting: 0,
  },
  scope_kind: "dm",
  session_key: "session-1",
  status: "active",
  updated_at: "2026-09-03T00:00:00Z",
  version: 1,
  work_items: [{
    acceptance_criteria: ["活动条样式一致"],
    deliverable: "共享视觉入口",
    id: "work-1",
    kind: "produce",
    logical_key: "activity-chip",
    objective: "统一前端活动条",
    owner_agent_id: "agent-1",
    position: 0,
    required: true,
    status: "running",
    subject: "收口排版",
    updated_at: "2026-09-03T00:00:00Z",
  }],
} satisfies ExecutionView;

describe("ExecutionProcessPanel", () => {
  it("keeps shared activity styling while preserving round and graph actions", () => {
    const onNavigateToRound = vi.fn();
    const onOpenGraph = vi.fn();
    const { container } = render(
      <I18nProvider>
        <ExecutionProcessPanel
          directory={{
            "agent-1": { avatar: null, id: "agent-1", name: "Nexus" },
          }}
          execution={EXECUTION}
          onNavigateToRound={onNavigateToRound}
          onOpenGraph={onOpenGraph}
        />
      </I18nProvider>,
    );

    const dock = container.querySelector("[data-execution-agent-activity-dock]");
    expect(dock?.className).toContain("conversation-activity-chip");
    expect(dock?.className).toContain("ui-type-metadata");
    expect(dock?.className).toContain("h-9");
    expect(dock?.className).toContain("px-1");
    expect(dock?.className).toContain("py-px");

    const agentAction = screen.getByRole("button", {
      name: /jump to nexus|跳转到 nexus/i,
    });
    expect(agentAction.className).toContain("h-8");
    expect(agentAction.className).toContain("radius-control-md");
    expect(agentAction.getAttribute("focusinset")).toBeNull();
    fireEvent.click(agentAction);
    expect(onNavigateToRound).toHaveBeenCalledWith("round-1");

    const graphAction = screen.getByRole("button", {
      name: /open full workgraph|打开完整工作图/i,
    });
    expect(graphAction.className).toContain("h-8");
    expect(graphAction.className).not.toContain("focus-visible:ring-inset");
    expect(graphAction.querySelector("svg")?.className.baseVal).toContain("h-4");
    fireEvent.click(graphAction);
    expect(onOpenGraph).toHaveBeenCalledOnce();
  });

  it("keeps all projected agents keyboard reachable with the graph action outside the scroll area", async () => {
    const user = userEvent.setup();
    const onNavigateToRound = vi.fn();
    const onOpenGraph = vi.fn();
    const nodes = Array.from({ length: 7 }, (_, index) => ({
      ...EXECUTION.graph.nodes[0],
      agent_id: `agent-${index + 1}`,
      agent_round_id: `round-${index + 1}`,
      id: `work-${index + 1}`,
      position: index,
      work_item_id: `work-${index + 1}`,
    }));
    const execution: ExecutionView = {
      ...EXECUTION,
      graph: { ...EXECUTION.graph, nodes },
      work_items: nodes.map((node) => ({
        ...EXECUTION.work_items[0],
        id: node.id,
        owner_agent_id: node.agent_id,
        position: node.position,
      })),
    };
    const { container } = render(
      <I18nProvider>
        <ExecutionProcessPanel
          directory={Object.fromEntries(nodes.map((node, index) => [
            node.agent_id,
            { avatar: null, id: node.agent_id, name: `Member ${index + 1}` },
          ]))}
          execution={execution}
          onNavigateToRound={onNavigateToRound}
          onOpenGraph={onOpenGraph}
        />
      </I18nProvider>,
    );

    const list = container.querySelector("[data-execution-agent-activity-list]");
    const agents = list!.querySelectorAll("button");
    const graph = container.querySelector("[data-execution-open-workgraph]");
    // The existing primary-node projection still limits the quick view to five agents.
    expect(agents).toHaveLength(5);
    expect(list?.className).toContain("min-w-0");
    expect(list?.className).toContain("overflow-x-auto");
    expect(list?.contains(graph)).toBe(false);
    expect(graph?.className).toContain("shrink-0");
    expect(container.querySelectorAll("[data-execution-agent-connection]")).toHaveLength(4);
    for (const [index, action] of Array.from(agents).entries()) {
      await user.tab();
      expect(document.activeElement).toBe(action);
      expect(action.className).toContain("focus-visible:ring-inset");
      await user.keyboard("{Enter}");
      expect(onNavigateToRound).toHaveBeenLastCalledWith(`round-${index + 1}`);
    }
    await user.tab();
    expect(document.activeElement).toBe(graph);
    await user.keyboard("{Enter}");
    expect(onOpenGraph).toHaveBeenCalledOnce();
  });

  it.each([
    { hasRound: false, hasNavigation: true },
    { hasRound: true, hasNavigation: false },
  ])("opens the graph when exact round navigation is unavailable: %j", ({ hasRound, hasNavigation }) => {
    const onNavigateToRound = vi.fn();
    const onOpenGraph = vi.fn();
    const { container } = render(
      <I18nProvider>
        <ExecutionProcessPanel
          directory={{ "agent-1": { avatar: null, id: "agent-1", name: "Nexus" } }}
          execution={{
            ...EXECUTION,
            graph: {
              ...EXECUTION.graph,
              nodes: [{ ...EXECUTION.graph.nodes[0], agent_round_id: hasRound ? "round-1" : undefined }],
            },
          }}
          onNavigateToRound={hasNavigation ? onNavigateToRound : undefined}
          onOpenGraph={onOpenGraph}
        />
      </I18nProvider>,
    );

    const action = container.querySelector("[data-execution-agent-activity]")!;
    expect(action.getAttribute("aria-label")).toMatch(/open full workgraph|打开完整工作图/i);
    fireEvent.click(action);
    expect(onOpenGraph).toHaveBeenCalledOnce();
    expect(onNavigateToRound).not.toHaveBeenCalled();
  });

  it("retains the graph entry without an empty avatar area or divider", () => {
    const onOpenGraph = vi.fn();
    const { container } = render(
      <I18nProvider>
        <ExecutionProcessPanel
          directory={{}}
          execution={{ ...EXECUTION, graph: { ...EXECUTION.graph, nodes: [] }, work_items: [] }}
          onOpenGraph={onOpenGraph}
        />
      </I18nProvider>,
    );

    expect(container.querySelector("[data-execution-agent-activity-list]")).toBeNull();
    expect(container.querySelector("[data-execution-agent-activity-divider]")).toBeNull();
    fireEvent.click(screen.getByRole("button"));
    expect(onOpenGraph).toHaveBeenCalledOnce();
  });

  it.each([
    ["running", "bg-(--success)"],
    ["blocked", "bg-(--warning)"],
    ["accepted", "bg-(--icon-muted)"],
  ] as const)("uses a single activity indicator for a %s dock avatar", (status, dotColor) => {
    const { container } = render(
      <ExecutionNodeAvatar
        agent={{ avatar: null, id: "agent-1", name: "Nexus" }}
        current={status === "running"}
        size="dock"
        status={status}
        title="Nexus activity"
        tone="activity"
      />,
    );

    const frame = container.querySelector("[data-execution-node-agent]")!;
    expect(frame.className).toContain("h-7");
    expect(frame.className).toContain("border-transparent");
    expect(frame.className).not.toMatch(/ring-|scale-|border-\(--(?:success|warning)\)/);
    expect(frame.getAttribute("title")).toBeNull();
    expect(screen.getByRole("img", { name: "Nexus" }).className).toContain("h-6.5");
    expect(frame.lastElementChild?.className).toContain(dotColor);
    expect(frame.getAttribute("data-execution-node-current")).toBe(status === "running" ? "true" : null);
  });

  it("preserves the full graph avatar geometry, status frame and selected state", () => {
    const { container } = render(
      <ExecutionNodeAvatar
        agent={{ avatar: null, id: "agent-1", name: "Nexus" }}
        current
        selected
        size="graph"
        status="running"
        title="Nexus node"
      />,
    );

    const frame = container.querySelector("[data-execution-node-agent]")!;
    expect(frame.className).toContain("h-11");
    expect(frame.className).toContain("border-(--primary)");
    expect(frame.className).toContain("scale-105");
    expect(frame.className).toContain("ring-(--primary)");
    expect(frame.getAttribute("title")).toBe("Nexus node");
    expect(screen.getByRole("img", { name: "Nexus" }).className).toContain("h-9.5");
  });
});
