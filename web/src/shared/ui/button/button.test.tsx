// INPUT: 文字、链接与图标 Button 的 type、disabled、名称和键盘行为。
// OUTPUT: 证明动作不会误提交表单，且保持原生 button/link 与可访问名称合同。
// POS: Button DOM 行为测试；实际视觉状态与尺寸在 Gallery 中人工查看。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  UiButton,
  UiIconButton,
  UiLinkButton,
} from "@/shared/ui/button/button";
import { UiSplitButton } from "@/shared/ui/button/split-button";

describe("UiButton", () => {
  it("groups a primary action and menu trigger without merging their commands", async () => {
    const user = userEvent.setup();
    const onMainAction = vi.fn();
    const onMenuAction = vi.fn();

    render(
      <UiSplitButton
        ariaLabel="允许操作"
        mainAction={{ children: "允许本次", onClick: onMainAction }}
        menuAction={{
          "aria-expanded": false,
          "aria-haspopup": "menu",
          "aria-label": "选择允许范围",
          children: <span aria-hidden="true">⌄</span>,
          onClick: onMenuAction,
        }}
      />,
    );

    const mainAction = screen.getByRole("button", { name: "允许本次" });
    const menuAction = screen.getByRole("button", { name: "选择允许范围" });

    expect(mainAction.getAttribute("type")).toBe("button");

    expect(menuAction.getAttribute("aria-haspopup")).toBe("menu");

    await user.tab();
    expect(document.activeElement).toBe(mainAction);
    await user.tab();
    expect(document.activeElement).toBe(menuAction);
    await user.click(mainAction);
    await user.click(menuAction);
    expect(onMainAction).toHaveBeenCalledTimes(1);
    expect(onMenuAction).toHaveBeenCalledTimes(1);
  });

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

  it("keeps busy actions natively disabled and resumes keyboard activation when ready", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    const { rerender } = render(
      <UiButton aria-busy="true" disabled onClick={onClick} tone="primary" variant="solid">保存</UiButton>,
    );
    const button = screen.getByRole("button", { name: "保存" });
    expect(button.getAttribute("aria-busy")).toBe("true");
    await user.click(button);
    expect(onClick).not.toHaveBeenCalled();
    rerender(<UiButton onClick={onClick} tone="primary" variant="solid">保存</UiButton>);
    expect(button.getAttribute("aria-busy")).toBeNull();
    button.focus();
    await user.keyboard("{Enter}");
    expect(onClick).toHaveBeenCalledOnce();
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

  it("can suppress the tooltip while preserving title-derived and explicit accessible names", async () => {
    const user = userEvent.setup();
    render(
      <>
        <UiIconButton title="关闭对话框" tooltip={null}>
          <span aria-hidden="true">×</span>
        </UiIconButton>
        <UiIconButton aria-label="关闭当前面板" title="关闭" tooltip={null}>
          <span aria-hidden="true">×</span>
        </UiIconButton>
        <UiIconButton title="查看详情">
          <span aria-hidden="true">?</span>
        </UiIconButton>
      </>,
    );

    const titleNamedButton = screen.getByRole("button", { name: "关闭对话框" });
    expect(titleNamedButton.getAttribute("title")).toBeNull();
    await user.tab();
    expect(document.activeElement).toBe(titleNamedButton);
    expect(screen.queryByRole("tooltip")).toBeNull();
    await user.tab();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "关闭当前面板" }));
    expect(screen.queryByRole("tooltip")).toBeNull();
    await user.tab();
    expect((await screen.findByRole("tooltip")).textContent).toBe("查看详情");
  });

});
