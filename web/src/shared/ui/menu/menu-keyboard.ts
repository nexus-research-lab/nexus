// INPUT: Menu/Listbox DOM、原生焦点、React 键盘事件与可选 Tab 退出命令。
// OUTPUT: 当前层级首项/当前选项与方向键/Home/End 遍历，Tab 退出使用共享焦点续接。
// POS: Action/Select/业务菜单共用键盘边界；不管理浮层、级联状态或执行业务命令。

import type { KeyboardEvent } from "react";

import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { focusAfterAnchoredOverlayExit } from "@/shared/ui/overlay/overlay-focus-navigation";
import { getAnchoredOverlayAncestorRoots } from "@/shared/ui/overlay/overlay-dismissal-runtime";

type PopupRole = "menu" | "listbox";

function getPopupItems(popup: HTMLElement, role: PopupRole): HTMLElement[] {
  const itemSelector = role === "menu"
    ? ':is([role="menuitem"], [role="menuitemcheckbox"])'
    : '[role="option"]';
  return Array.from(popup.querySelectorAll<HTMLElement>(
    `${itemSelector}:not([aria-disabled="true"]):not(:disabled)`,
  )).filter((item) => item.closest(`[role="${role}"]`) === popup);
}

export function focusFirstMenuItem(menu: HTMLElement | null): void {
  if (menu) (getPopupItems(menu, "menu")[0] ?? menu).focus();
}

export function focusSelectedListboxItem(listbox: HTMLElement | null): void {
  if (!listbox) return;
  const items = getPopupItems(listbox, "listbox");
  (items.find((item) => item.getAttribute("aria-selected") === "true") ?? items[0] ?? listbox).focus();
}

export function handleMenuKeyDown(event: KeyboardEvent<HTMLElement>, onTabExit?: () => void): void {
  handlePopupKeyDown(event, "menu", onTabExit);
}

export function handleListboxKeyDown(event: KeyboardEvent<HTMLElement>, onTabExit?: () => void): void {
  handlePopupKeyDown(event, "listbox", onTabExit);
}

function handlePopupKeyDown(
  event: KeyboardEvent<HTMLElement>,
  role: PopupRole,
  onTabExit?: () => void,
): void {
  if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent)) return;
  const target = event.target;
  // React Portal 会沿组件树冒泡；外部浮层不能借父级菜单执行导航或退出。
  if (!(target instanceof Element) || !event.currentTarget.contains(target)) return;
  if (event.key === "Tab" && onTabExit) {
    // Portal 关闭会移除原始事件目标；显式从归还的锚点续接，避免焦点落到 body。
    event.preventDefault();
    event.stopPropagation();
    const exitingRoots = getAnchoredOverlayAncestorRoots(event.currentTarget);
    onTabExit();
    focusAfterAnchoredOverlayExit(exitingRoots, event.shiftKey);
    return;
  }
  const menu = target.closest<HTMLElement>(`[role="${role}"]`);
  if (!menu || !event.currentTarget.contains(menu)) return;
  if (target.closest('input, textarea, select, [contenteditable="true"]')) return;
  if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
  const items = getPopupItems(menu, role);
  if (!items.length) return;
  event.preventDefault();
  event.stopPropagation();
  const currentIndex = items.indexOf(document.activeElement as HTMLElement);
  const direction = event.key === "ArrowDown" ? 1 : -1;
  const start = currentIndex >= 0 ? currentIndex : direction > 0 ? -1 : 0;
  const index = event.key === "Home" ? 0
    : event.key === "End" ? items.length - 1
      : (start + direction + items.length) % items.length;
  items[index].focus();
}
