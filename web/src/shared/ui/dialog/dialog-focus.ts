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
