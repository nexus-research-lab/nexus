// INPUT: Disclosure 的标题、内容、variant/density、受控 open 与原生 toggle 行为。
// OUTPUT: 证明原生语义、键盘入口、共享几何和展开箭头状态。
// POS: Disclosure primitive DOM 合同；业务展开策略由消费者测试。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";

describe("UiDisclosure", () => {
  it("renders a native disclosure with shared panel chrome and toggles itself", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <UiDisclosure
        defaultOpen
        label="连接说明"
        meta="5 步"
        variant="panel"
      >
        <p>创建平台应用</p>
      </UiDisclosure>,
    );

    const details = container.querySelector("details");
    const summary = container.querySelector("summary");
    expect(details?.open).toBe(true);
    expect(details?.className).toContain("surface-radius-md");
    expect(summary?.className).toContain("focus-visible:ring-2");
    expect(summary?.className).toContain("ui-type-supporting");
    expect(screen.getByText("创建平台应用").parentElement?.className)
      .toContain("border-(--divider-subtle-color)");
    expect(container.querySelector("svg")?.className.baseVal)
      .toContain("group-open/disclosure:rotate-180");
    await user.click(screen.getByText("连接说明"));
    expect(details?.open).toBe(false);
  });

  it("forwards controlled open and toggle behavior while keeping compact row geometry", () => {
    const onToggle = vi.fn();
    const { container } = render(
      <UiDisclosure
        density="compact"
        label="运行失败"
        onToggle={onToggle}
        open={false}
        variant="row"
      >
        network timeout
      </UiDisclosure>,
    );

    const details = container.querySelector("details");
    expect(container.querySelector("summary")?.className).toContain("min-h-9");
    fireEvent(details as HTMLDetailsElement, new Event("toggle"));
    expect(onToggle).toHaveBeenCalledOnce();
  });
});
