// INPUT: Select/Action Menu 的触发器、选项、disabled 项与用户键盘/点击事件。
// OUTPUT: 证明 Portal 菜单的 ARIA、选择、输入法/已处理事件边界、遍历、关闭和焦点归还合同。
// POS: Menu pattern DOM 行为测试；定位数学和业务菜单内容分别由模型/feature 测试负责。

import { render as renderReact, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

function render(ui: ReactNode) { return renderReact(ui, { wrapper: I18nProvider }); }

// jsdom 不计算布局；为焦点目录提供可见控件的矩形，不模拟浏览器视觉验收。
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe("UiSelectMenu listbox navigation", () => {
  const options = [
    { label: "Alpha", value: "alpha" },
    { label: "Disabled", value: "disabled", disabled: true },
    { label: "Gamma with a complete model name", value: "gamma" },
  ];

  it("allows exit from an all-disabled list and closes when its catalog becomes empty", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const view = render(<UiSelectMenu ariaLabel="Model" options={options.map((option) => ({ ...option, disabled: true }))} value="alpha" onChange={onChange} />);
    const trigger = screen.getByRole("button", { name: "Model" });
    await user.click(trigger);
    expect(document.activeElement).toBe(screen.getByRole("listbox"));
    await user.keyboard("{ArrowDown}{Enter}{Escape}");
    expect(onChange).not.toHaveBeenCalled();
    expect(document.activeElement).toBe(trigger);
    await user.click(trigger);
    view.rerender(<UiSelectMenu ariaLabel="Model" options={[]} value="alpha" onChange={onChange} />);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect((trigger as HTMLButtonElement).disabled).toBe(true);
    view.rerender(<UiSelectMenu ariaLabel="Model" options={options} value="alpha" onChange={onChange} />);
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
  });
});
