// INPUT: Menu DOM、原生焦点与 React 键盘事件，以及可选的 Tab 退出命令。
// OUTPUT: 当前菜单内动作和勾选项的首项焦点、方向键/Home/End 遍历，以及关闭后从锚点续接的 Tab 焦点。
// POS: Action/业务菜单共用键盘边界；不管理浮层、级联状态或执行业务命令。

import type { KeyboardEvent } from "react";

import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { getTabbableElements } from "@/shared/lib/browser/focus-navigation";
import { getAnchoredOverlayAncestorRoots } from "@/shared/ui/overlay/overlay-dismissal-runtime";

function getMenuItems(menu: HTMLElement): HTMLElement[] {
  return Array.from(menu.querySelectorAll<HTMLElement>(
    ':is([role="menuitem"], [role="menuitemcheckbox"]):not([aria-disabled="true"]):not(:disabled)',
  )).filter((item) => item.closest('[role="menu"]') === menu);
}

export function focusFirstMenuItem(menu: HTMLElement | null): void {
  if (menu) (getMenuItems(menu)[0] ?? menu).focus();
}

export function handleMenuKeyDown(
  event: KeyboardEvent<HTMLElement>,
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
    focusAfterMenuExit(exitingRoots, event.shiftKey);
    return;
  }
  const menu = target.closest<HTMLElement>('[role="menu"]');
  if (!menu || !event.currentTarget.contains(menu)) return;
  if (target.closest('input, textarea, select, [contenteditable="true"]')) return;
  if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
  const items = getMenuItems(menu);
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

function focusAfterMenuExit(menuRoots: readonly HTMLElement[], backwards: boolean): void {
  const anchor = document.activeElement;
  if (!(anchor instanceof HTMLElement)) return;
  const exitingRoots = menuRoots.filter((root) => !root.contains(anchor));
  if (!exitingRoots.length) return;
  const modal = anchor.closest<HTMLElement>("[data-modal-root='true']");
  const targets = getTabbableElements(modal ?? document.body)
    .filter((element) => !exitingRoots.some((root) => root.contains(element)));
  const index = targets.indexOf(anchor);
  let adjacent: HTMLElement | undefined;
  if (index >= 0) {
    adjacent = targets[index + (backwards ? -1 : 1)];
  } else if (backwards) {
    adjacent = targets.findLast((element) => Boolean(anchor.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_PRECEDING));
  } else {
    adjacent = targets.find((element) => Boolean(anchor.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_FOLLOWING));
  }
  if (!adjacent && modal) adjacent = backwards ? targets.at(-1) : targets[0];
  (adjacent ?? anchor).focus();
}
