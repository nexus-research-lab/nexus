// INPUT: 设置目录 Pattern 的活动态、点击动作和分组标题。
// OUTPUT: 证明目录投影当前页语义，且禁用入口不执行动作。
// POS: Settings 共享视图行为测试；不覆盖路由或 Provider 业务状态。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  SettingsNavigationButton,
  SettingsNavigationGroupLabel,
} from "@/features/settings/shared/settings-panel-ui";

describe("Settings navigation UI", () => {
  it("projects the active page and dispatches its action", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(
      <nav aria-label="设置">
        <SettingsNavigationGroupLabel>界面</SettingsNavigationGroupLabel>
        <SettingsNavigationButton active onClick={onClick}>
          外观
        </SettingsNavigationButton>
      </nav>,
    );

    const button = screen.getByRole("button", { name: "外观" });
    expect(button.getAttribute("type")).toBe("button");
    expect(button.getAttribute("aria-current")).toBe("page");
    screen.getByText("界面");

    await user.click(button);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("keeps inactive and disabled entries out of the current-page state", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(
      <SettingsNavigationButton disabled onClick={onClick} size="lg">
        尚未支持
      </SettingsNavigationButton>,
    );

    const button = screen.getByRole("button", { name: "尚未支持" });
    expect(button.hasAttribute("aria-current")).toBe(false);
    await user.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });
});
