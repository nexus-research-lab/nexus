// INPUT: GlassSwitch 的点击、键盘、禁用与可访问属性。
// OUTPUT: 证明一个真实 switch DOM 同时拥有交互、状态和禁用语义。
// POS: Shared Switch 行为测试；业务确认和持久化由消费者测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";

describe("GlassSwitch", () => {
  it("preserves all supplied description references on the native switch", () => {
    render(<>
      <p id="switch-risk">仅为可信页面开启</p>
      <p id="switch-description">允许访问完整调试功能</p>
      <GlassSwitch aria-label="完整浏览器访问" aria-describedby="switch-risk switch-description"
        checked={false} onChange={vi.fn()} />
    </>);
    const control = screen.getByRole("switch", { name: "完整浏览器访问" });
    expect(control.tagName).toBe("BUTTON");
    const descriptions = control.getAttribute("aria-describedby")!.split(" ")
      .map((id) => document.getElementById(id)?.textContent);
    expect(descriptions).toEqual(["仅为可信页面开启", "允许访问完整调试功能"]);
  });

  it("activates one native switch through pointer and keyboard", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const { rerender } = render(
      <GlassSwitch
        aria-label="自动回复"
        checked={false}
        onChange={onChange}
        title="切换自动回复"
      />,
    );

    const control = screen.getByRole("switch", { name: "自动回复" });
    expect(control.getAttribute("aria-checked")).toBe("false");
    expect(control.getAttribute("title")).toBe("切换自动回复");
    await user.click(control);
    expect(onChange).toHaveBeenLastCalledWith(true);

    rerender(
      <GlassSwitch aria-label="自动回复" checked onChange={onChange} />,
    );
    control.focus();
    await user.keyboard("{Enter}");
    expect(onChange).toHaveBeenLastCalledWith(false);
  });

  it("projects disabled state to the native button and blocks activation", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <GlassSwitch
        aria-label="自动回复"
        checked
        disabled
        onChange={onChange}
      />,
    );

    const control = screen.getByRole("switch", { name: "自动回复" }) as HTMLButtonElement;
    expect(control.disabled).toBe(true);
    await user.click(control);
    expect(onChange).not.toHaveBeenCalled();
  });
});
