// INPUT: Mention 浮层的锚点显示状态、候选项和编辑器键盘事件。
// OUTPUT: 证明隐藏浮层不截获键盘，打开后可选值，关闭后立即释放键盘所有权。
// POS: Mention 键盘生命周期回归测试；不涉及业务目标或文本插入策略。

import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";

import { MentionTargetPopover } from "./mention-target-popover";

it("captures navigation only while visible and releases it after closing", () => {
  const onSelect = vi.fn();
  const onClose = vi.fn();
  const forwardedKeys: string[] = [];
  const items = [
    { id: "maya", label: "Maya", marker: "M" },
    { id: "lin", label: "Lin", marker: "L" },
  ];
  function Harness({ anchorRect }: { anchorRect: DOMRect | null }) {
    return (
      <>
        <input aria-label="编辑器" onKeyDown={(event) => forwardedKeys.push(event.key)} />
        <MentionTargetPopover anchorRect={anchorRect} filter="" items={items} onClose={onClose} onSelect={onSelect} />
      </>
    );
  }
  const view = render(<Harness anchorRect={null} />);
  const editor = screen.getByRole("textbox", { name: "编辑器" });
  const press = (key: string) => {
    const event = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, key });
    fireEvent(editor, event);
    return event;
  };

  for (const key of ["ArrowDown", "Enter", "Escape"]) {
    expect(press(key).defaultPrevented).toBe(false);
  }
  expect(forwardedKeys).toEqual(["ArrowDown", "Enter", "Escape"]);
  expect(onSelect).not.toHaveBeenCalled();
  expect(onClose).not.toHaveBeenCalled();

  view.rerender(<Harness anchorRect={new DOMRect(20, 20, 200, 32)} />);
  const lin = screen.getByText("Lin").closest("button")!;
  lin.scrollIntoView = vi.fn();
  expect(press("ArrowDown").defaultPrevented).toBe(true);
  expect(press("Enter").defaultPrevented).toBe(true);
  expect(onSelect).toHaveBeenCalledWith(items[1]);
  expect(press("Escape").defaultPrevented).toBe(true);
  expect(onClose).toHaveBeenCalledOnce();
  expect(forwardedKeys).toHaveLength(3);

  view.rerender(<Harness anchorRect={null} />);
  expect(press("ArrowDown").defaultPrevented).toBe(false);
  expect(forwardedKeys).toEqual(["ArrowDown", "Enter", "Escape", "ArrowDown"]);
});
