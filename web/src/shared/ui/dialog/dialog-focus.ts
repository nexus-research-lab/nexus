// INPUT: 模态根、共享 DOM 焦点目录与当前浏览器焦点。
// OUTPUT: 根内焦点位置及不滚动页面的聚焦动作。
// POS: Dialog 焦点适配；可操作元素发现归 shared/lib/browser/focus-navigation。

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
