// INPUT: 带来源、当前进度与可展开详情的 Todo 投影。
// OUTPUT: 证明共享活动条排版、图标动作、展开内容与焦点返回合同。
// POS: Workspace Task Strip DOM 行为测试；Todo 归一化由 model 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { TodoItem } from "@/types/conversation/todo";

import { WorkspaceTaskPanel } from "./workspace-task-strip";

const TODOS: TodoItem[] = [
  {
    active_form: "已完成需求解析",
    content: "解析需求",
    status: "completed",
  },
  {
    active_form: "正在统一排版",
    content: "统一活动条",
    status: "in_progress",
  },
];

describe("WorkspaceTaskPanel", () => {
  it("keeps compact typography and icon actions under shared UI owners", () => {
    const { container } = render(
      <I18nProvider>
        <WorkspaceTaskPanel
          source={{ agentId: "agent-nexus", avatar: null, name: "Nexus" }}
          todos={TODOS}
        />
      </I18nProvider>,
    );

    const trigger = container.querySelector<HTMLButtonElement>("[data-workspace-task-trigger]");
    const visual = container.querySelector("[data-workspace-task-visual]");
    expect(trigger).not.toBeNull();
    expect(visual?.className).toContain("conversation-activity-chip");
    expect(visual?.className).toContain("ui-type-metadata");
    expect(screen.getByText("Nexus").className).toContain("ui-type-caption");

    fireEvent.click(trigger!);

    const progress = container.querySelector("[data-workspace-task-progress-label]");
    expect(progress?.className).toContain("ui-type-metadata");
    expect(progress?.className).toContain("ui-type-tone-soft");
    expect(screen.getByText("统一活动条").className).toContain("ui-type-metadata");

    const detailActions = screen.getAllByRole("button", {
      name: /expand task details|展开任务详情/i,
    });
    expect(detailActions[0].className).toContain("radius-control-xs");
    fireEvent.click(detailActions[0]);
    expect(screen.getByText("已完成需求解析").className).toContain("ui-type-caption");

    const collapse = screen.getAllByRole("button", {
      name: /collapse tasks panel|收起任务面板/i,
    }).find((action) => action !== trigger);
    expect(collapse).toBeDefined();
    expect(collapse!.className).toContain("radius-control-xs");
    fireEvent.click(collapse!);
    expect(document.activeElement).toBe(trigger);
    expect(container.querySelector("[data-workspace-task-progress-label]")).toBeNull();
  });
});
