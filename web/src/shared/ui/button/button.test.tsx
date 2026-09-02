// INPUT: UiButton 的默认/显式 type、disabled 状态和键盘焦点行为。
// OUTPUT: 证明基础动作不会误提交表单，且保持原生 button 交互合同。
// POS: Button DOM 行为测试；视觉 token 组合由样式合同测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { UiButton } from "@/shared/ui/button/button";

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
});
