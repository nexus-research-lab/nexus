// INPUT: 文字、链接与图标 Button 的 type、disabled、名称和键盘行为。
// OUTPUT: 证明动作不会误提交表单，且保持原生 button/link 与可访问名称合同。
// POS: Button DOM 行为测试；视觉 token 组合由样式合同测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  UiButton,
  UiIconButton,
  UiLinkButton,
} from "@/shared/ui/button/button";

describe("UiButton", () => {
  it("defaults to a non-submitting button inside forms", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());

    render(
      <form onSubmit={onSubmit}>
        <UiButton>普通动作</UiButton>
      </form>,
    );

    const button = screen.getByRole("button", { name: "普通动作" });
    expect(button.getAttribute("type")).toBe("button");
    await user.click(button);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("submits only when the caller opts into submit", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());

    render(
      <form onSubmit={onSubmit}>
        <UiButton type="submit">保存</UiButton>
      </form>,
    );

    await user.click(screen.getByRole("button", { name: "保存" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("keeps disabled actions out of click and keyboard focus flows", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    function Harness() {
      const [count, setCount] = useState(0);
      return (
        <>
          <UiButton disabled onClick={onClick}>停用动作</UiButton>
          <UiButton onClick={() => setCount((value) => value + 1)}>可用动作 {count}</UiButton>
        </>
      );
    }

    render(<Harness />);
    await user.click(screen.getByRole("button", { name: "停用动作" }));
    expect(onClick).not.toHaveBeenCalled();

    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "可用动作 0" }));
    await user.keyboard("{Enter}");
    expect(screen.getByRole("button", { name: "可用动作 1" })).toBeTruthy();
  });

  it("keeps navigation actions as links instead of button-shaped click handlers", () => {
    render(<UiLinkButton href="/docs">查看文档</UiLinkButton>);

    const link = screen.getByRole("link", { name: "查看文档" });
    expect(link.getAttribute("href")).toBe("/docs");
  });

  it("derives an icon action name from the shared tooltip contract", () => {
    render(
      <UiIconButton title="删除附件">
        <span aria-hidden="true">×</span>
      </UiIconButton>,
    );

    const button = screen.getByRole("button", { name: "删除附件" });
    expect(button.getAttribute("type")).toBe("button");
    expect(button.getAttribute("title")).toBeNull();
  });

  it("projects circular icon actions through a semantic shape", () => {
    render(
      <UiIconButton aria-label="返回" shape="round" size="lg">
        <span aria-hidden="true">←</span>
      </UiIconButton>,
    );

    const button = screen.getByRole("button", { name: "返回" });
    expect(button.className).toContain("h-9");
    expect(button.className).toContain("rounded-full");
    expect(button.className).not.toContain("radius-control-lg");
  });

  it("owns the micro action sizes used inside dense toolbars and chips", () => {
    render(
      <>
        <UiButton size="2xs" variant="text">引导</UiButton>
        <UiIconButton aria-label="移除" size="2xs">
          <span aria-hidden="true">×</span>
        </UiIconButton>
      </>,
    );

    const textAction = screen.getByRole("button", { name: "引导" });
    const iconAction = screen.getByRole("button", { name: "移除" });
    expect(textAction.className).toContain("min-h-6");
    expect(textAction.className).toContain("ui-type-caption");
    expect(iconAction.className).toContain("h-5");
    expect(iconAction.className).toContain("w-5");
    expect(iconAction.className).toContain("radius-control-xs");
  });
});
