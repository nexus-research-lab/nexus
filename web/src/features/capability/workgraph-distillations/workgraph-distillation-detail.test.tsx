// INPUT: 一个命名 WorkGraph 与详情页动作回调。
// OUTPUT: 证明详情使用共享 Button、Panel、Typography、语义画布形状并保持动作行为。
// POS: WorkGraph 能力详情 DOM 合同；资源、删除和编辑事务由目录/controller 门禁负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";

import { WorkGraphDistillationDetail } from "./workgraph-distillation-detail";

const mocks = vi.hoisted(() => ({
  t: (key: string) => key,
}));

vi.mock("@/shared/i18n/i18n-context", () => ({
  useI18n: () => ({ locale: "zh", t: mocks.t }),
}));

vi.mock("@/features/conversation/shared/execution/workgraph-workflow-canvas-preview", () => ({
  WorkGraphWorkflowCanvasPreview: ({ className }: { className?: string }) => (
    <div className={className} data-testid="workflow-canvas" />
  ),
}));

const WORKFLOW: WorkGraphWorkflow = {
  id: "workflow-1",
  slash_name: "release-check",
  title: "Release check",
  description: "Prepare and verify a release.",
  source_execution_id: "execution-1",
  source_session_key: "session-1",
  objective: "Ship a verified release.",
  nodes: [],
  version: 1,
  created_at: "2026-09-03T00:00:00Z",
  updated_at: "2026-09-03T00:00:00Z",
};

describe("WorkGraphDistillationDetail", () => {
  it("renders semantic detail chrome and dispatches shared actions", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();
    const onCopy = vi.fn();
    const onEdit = vi.fn();
    render(
      <WorkGraphDistillationDetail
        item={WORKFLOW}
        onBack={onBack}
        onCopy={onCopy}
        onEdit={onEdit}
      />,
    );

    expect(screen.getByRole("heading", { name: "/release-check" }).className).toContain("ui-type-object-title");
    expect(screen.getByText(WORKFLOW.description ?? "").className).toContain("ui-type-supporting");
    expect(screen.getByText(WORKFLOW.objective).className).toContain("ui-type-body");
    expect(screen.getByTestId("workflow-canvas").className).toContain("surface-radius-md");

    await user.click(screen.getByRole("button", { name: "common.back" }));
    await user.click(screen.getByRole("button", { name: "capability.workgraph_edit" }));
    await user.click(screen.getByRole("button", { name: "capability.workgraph_copy" }));

    expect(onBack).toHaveBeenCalledOnce();
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onCopy).toHaveBeenCalledOnce();
  });

  it("does not offer editing for built-in templates", () => {
    render(
      <WorkGraphDistillationDetail
        item={{ ...WORKFLOW, built_in: true }}
        onBack={vi.fn()}
        onCopy={vi.fn()}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.queryByRole("button", {
      name: "capability.workgraph_edit",
    })).toBeNull();
  });
});
