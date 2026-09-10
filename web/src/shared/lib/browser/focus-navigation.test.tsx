// INPUT: 原生控件、fieldset、隐藏/inert 区域、tabindex 与同名 radio。
// OUTPUT: 证明菜单与 Dialog 共用可见、可用且按 Tab 顺序排列的焦点目录。
// POS: DOM 边界回归；不模拟浏览器的窗口或可见性绘制。

import { render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

import { getTabbableElements } from "./focus-navigation";

afterEach(() => vi.restoreAllMocks());

it("uses native disabled/Tab semantics and keeps radio groups and hidden regions out of extra stops", () => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([new DOMRect(0, 0, 100, 32)] as unknown as DOMRectList);
  const { container } = render(<>
    <button type="button">Normal</button>
    <button type="button">Second</button>
    <button type="button">First</button>
    <button type="button" tabIndex={-1}>Programmatic</button>
    <fieldset disabled><button type="button">Disabled by fieldset</button></fieldset>
    <div inert><button type="button">Inert child</button></div>
    <div hidden><button type="button">Hidden child</button></div>
    <button type="button" style={{ visibility: "hidden" }}>Invisible</button>
    <input aria-label="Radio one" type="radio" name="choice" />
    <input aria-label="Radio two" type="radio" name="choice" defaultChecked />
  </>);
  // 检查外部 DOM 的已有正 tabindex；产品 JSX 不创建正 tabindex。
  screen.getByRole("button", { name: "Second" }).tabIndex = 2;
  screen.getByRole("button", { name: "First" }).tabIndex = 1;
  expect(getTabbableElements(container)).toEqual([
    screen.getByRole("button", { name: "First" }),
    screen.getByRole("button", { name: "Second" }),
    screen.getByRole("button", { name: "Normal" }),
    screen.getByRole("radio", { name: "Radio two" }),
  ]);
});
