// INPUT: Skill 卡片内容、整卡选择命令与独立领域动作。
// OUTPUT: 证明目录卡复用共享 Button/Typography，并保持覆盖选择与独立动作边界。
// POS: Skill 共享目录卡 DOM 合同；业务徽标和命令状态由 catalog/external 负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiIconButton } from "@/shared/ui/button/button";

import { SkillDirectoryCard } from "./skill-directory-card";

describe("SkillDirectoryCard", () => {
  it("keeps the shared card action separate from a domain action", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const onSelect = vi.fn();
    render(
      <SkillDirectoryCard
        action={(
          <UiIconButton
            aria-label="删除技能"
            className="pointer-events-auto"
            onClick={onAction}
            size="sm"
          >
            <span aria-hidden="true">×</span>
          </UiIconButton>
        )}
        description="Collect evidence and produce a concise report."
        meta={<span>Built-in · v1.2.0</span>}
        onSelect={onSelect}
        seed="research-skill"
        title="Research Skill"
      />,
    );

    const selectAction = screen.getByRole("button", { name: "Research Skill" });
    expect(selectAction.className).toContain("surface-radius-md");
    expect(screen.getByRole("heading", { name: "Research Skill" }).className)
      .toContain("ui-type-section-title");
    expect(screen.getByText("Collect evidence and produce a concise report.").className)
      .toContain("ui-type-metadata");
    expect(screen.getByText("Built-in · v1.2.0").parentElement?.className)
      .toContain("ui-type-caption");

    await user.click(selectAction);
    expect(onSelect).toHaveBeenCalledOnce();

    await user.click(screen.getByRole("button", { name: "删除技能" }));
    expect(onAction).toHaveBeenCalledOnce();
    expect(onSelect).toHaveBeenCalledOnce();
  });
});
