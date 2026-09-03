// INPUT: Agent 自动保存的 saving/error 状态与用户展开动作。
// OUTPUT: 证明状态复用 Spinner、IconButton、Typography 和语义浮层并正确开合。
// POS: Contacts 详情保存状态 DOM 合同；不触发真实保存。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { AgentOptionsPersistenceStatus } from "./agent-options-persistence-status";

describe("AgentOptionsPersistenceStatus", () => {
  it("uses the shared reduced-motion spinner while saving", () => {
    const { container } = render(
      <AgentOptionsPersistenceStatus
        state={{ message: "正在保存", phase: "saving" }}
      />,
    );

    const status = container.querySelector("[aria-live='polite']");
    const spinner = container.querySelector("svg.animate-spin");
    expect(status?.className).toContain("ui-type-caption");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });

  it("opens the tokenized mobile error popover from the shared icon action", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <AgentOptionsPersistenceStatus
        state={{ message: "保存失败，请重试", phase: "error" }}
      />,
    );

    const action = screen.getByRole("button", { name: "保存失败，请重试" });
    expect(action.className).toContain("radius-control-md");
    await user.click(action);

    const popover = container.querySelector("[data-agent-save-error-popover]");
    expect(popover?.className).toContain("ui-layer-popover");
    expect(popover?.className).toContain("surface-popover");
    expect(popover?.className).toContain("ui-type-caption");
    await user.click(action);
    expect(container.querySelector("[data-agent-save-error-popover]")).toBeNull();
  });
});
