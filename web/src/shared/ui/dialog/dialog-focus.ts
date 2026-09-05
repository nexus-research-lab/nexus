// INPUT: 模态根、可见可操作 DOM 与当前浏览器焦点。
// OUTPUT: 可聚焦元素目录、根内焦点位置及不滚动页面的聚焦动作。
// POS: Dialog 焦点几何适配；不决定关闭顺序或键盘规则。
const FOCUSABLE_SELECTOR = [
  "a[href]",
  "button:not([disabled])",
  "textarea:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function isVisibleFocusTarget(element: HTMLElement): boolean {
  if (element.hasAttribute("disabled") || element.getAttribute("aria-hidden") === "true") {
    return false;
  }

  const style = window.getComputedStyle(element);
  return style.visibility !== "hidden"
    && style.display !== "none"
    && element.getClientRects().length > 0;
}

export function getDialogFocusableElements(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    isVisibleFocusTarget,
  );
}

export function getDialogFocusState(
  root: HTMLElement,
  focusable: readonly HTMLElement[],
): { activeIndex: number; focusInside: boolean } {
  const activeElement = document.activeElement;
  if (!(activeElement instanceof HTMLElement)) {
    return { activeIndex: -1, focusInside: false };
  }
  return {
    activeIndex: focusable.indexOf(activeElement),
    focusInside: root.contains(activeElement),
  };
}

export function focusDialogElement(element: HTMLElement | null | undefined): void {
  element?.focus({ preventScroll: true });
}
