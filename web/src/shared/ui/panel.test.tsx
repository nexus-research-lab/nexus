// INPUT: Panel 的默认值、显式视觉语义、原生属性和用户事件。
// OUTPUT: 证明内容表面只保留真实 variant，默认无阴影且不吞掉 section 属性。
// POS: Panel primitive DOM 合同；页面分栏与业务状态由消费者负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UiPanel } from "@/shared/ui/panel";

describe("UiPanel", () => {
  it("uses the neutral card contract by default and forwards section behavior", () => {
    const onClick = vi.fn();
    render(
      <UiPanel aria-label="项目摘要" onClick={onClick}>
        内容
      </UiPanel>,
    );

    const panel = screen.getByRole("region", { name: "项目摘要" });
    expect(panel.tagName).toBe("SECTION");
    expect(panel.className).toContain("shadow-none");
    expect(panel.className).toContain("surface-radius-md");
    expect(panel.className).toContain("px-4");
    fireEvent.click(panel);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("keeps dashed and plain as distinct, finite variants", () => {
    const { rerender } = render(
      <UiPanel aria-label="待添加" padding="none" radius="sm" variant="dashed">
        空状态
      </UiPanel>,
    );

    const panel = screen.getByRole("region", { name: "待添加" });
    expect(panel.className).toContain("border-dashed");
    expect(panel.className).toContain("surface-radius-sm");
    expect(panel.className).not.toContain("px-");

    rerender(
      <UiPanel aria-label="待添加" variant="plain">
        空状态
      </UiPanel>,
    );
    expect(panel.className).not.toContain("border-dashed");
    expect(panel.className).not.toContain("shadow-");
  });
});
