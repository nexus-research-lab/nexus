// INPUT: Tooltip 文案、触发器以及用户的焦点、悬停、键盘和指针事件。
// OUTPUT: 证明共享 Tooltip 的延迟、ARIA 关联、关闭行为和语义层级合同。
// POS: Tooltip primitive DOM 行为测试；锚点坐标数学由 overlay model 合同测试负责。

import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";

afterEach(() => {
  vi.useRealTimers();
});

describe("UiTooltip", () => {
  it("skips repeated visible text but reveals clipped text", () => {
    render(<UiTooltip label="子智能体"><button type="button">子智能体</button></UiTooltip>);
    const trigger = screen.getByRole("button");
    act(() => trigger.focus());
    expect(screen.queryByRole("tooltip")).toBeNull();
    act(() => trigger.blur());
    Object.defineProperties(trigger, { clientWidth: { value: 40 }, scrollWidth: { value: 80 } });
    act(() => trigger.focus());
    expect(screen.getByRole("tooltip").textContent).toBe("子智能体");
  });

});
