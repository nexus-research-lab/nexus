// INPUT: 含一个运行中一级 Agent 节点的 Execution 投影与导航动作。
// OUTPUT: 证明 Composer WorkGraph 活动条复用共享排版/图标按钮并保持导航语义。
// POS: Execution Process Panel DOM 行为测试；图节点选择规则由 model 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ExecutionView } from "@/types/conversation/execution";

import { ExecutionProcessPanel } from "./execution-process-panel";

const EXECUTION: ExecutionView = {
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
};

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
    expect(dock?.className).toContain("min-h-10");
    expect(dock?.className).toContain("px-1");
    expect(dock?.className).toContain("py-1");

    const agentAction = screen.getByRole("button", {
      name: /jump to nexus|跳转到 nexus/i,
    });
    expect(agentAction.className).toContain("h-8");
    expect(agentAction.className).toContain("radius-control-md");
    fireEvent.click(agentAction);
    expect(onNavigateToRound).toHaveBeenCalledWith("round-1");

    const graphAction = screen.getByRole("button", {
      name: /open full workgraph|打开完整工作图/i,
    });
    expect(graphAction.className).toContain("h-8");
    expect(graphAction.querySelector("svg")?.className.baseVal).toContain("h-[18px]");
    fireEvent.click(graphAction);
    expect(onOpenGraph).toHaveBeenCalledOnce();
  });
});
