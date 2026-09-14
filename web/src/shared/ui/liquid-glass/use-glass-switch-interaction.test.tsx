// INPUT: Switch 主/次按钮和丢失指针捕获事件。
// OUTPUT: 装饰按压状态准确结束，捕获生命周期不提交业务切换。
// POS: GlassSwitch 交互 Hook 回归，原生键盘/点击由组件测试覆盖。
import { act, renderHook } from "@testing-library/react";
import type { PointerEvent } from "react";
import { expect, it, vi } from "vitest";
import { useGlassSwitchInteraction } from "./use-glass-switch-interaction";

it("ignores secondary presses and clears primary presses when capture is lost", () => {
  const onChange = vi.fn();
  const capture = vi.fn();
  const { result } = renderHook(() => useGlassSwitchInteraction({ checked: false, disabled: false, onChange }));
  const event = (button: number) => ({
    button,
    pointerId: 1,
    currentTarget: { setPointerCapture: capture },
  }) as unknown as PointerEvent<HTMLButtonElement>;

  act(() => { result.current.buttonHandlers.onPointerDown(event(2)); });
  expect(result.current.isPressed).toBe(false);
  expect(capture).not.toHaveBeenCalled();
  act(() => { result.current.buttonHandlers.onPointerDown(event(0)); });
  expect(result.current.isPressed).toBe(true);
  expect(capture).toHaveBeenCalledWith(1);
  act(() => { result.current.buttonHandlers.onLostPointerCapture(); });
  expect(result.current.isPressed).toBe(false);
  expect(onChange).not.toHaveBeenCalled();
});
