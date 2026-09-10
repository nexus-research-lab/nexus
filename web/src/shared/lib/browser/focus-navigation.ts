// INPUT: DOM 范围、原生可操作控件及其可见性/Tab 顺序。
// OUTPUT: 跳过隐藏、禁用、inert 和负 tabindex 的有序焦点目录。
// POS: Dialog 焦点循环和菜单 Tab 退出的中立 DOM 边界；不拥有关闭、模态栈或业务动作。

const FOCUSABLE_SELECTOR = "a[href], button, textarea, input, select, [tabindex]";

export function getTabbableElements(root: HTMLElement): HTMLElement[] {
  const elements = Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter((element) => {
    if (element.tabIndex < 0 || element.matches(":disabled")
      || element.closest('[hidden], [inert], [aria-hidden="true"]')) return false;
    const style = window.getComputedStyle(element);
    return style.visibility !== "hidden" && style.display !== "none" && element.getClientRects().length > 0;
  });
  return elements.filter((element) => {
    if (!(element instanceof HTMLInputElement) || element.type !== "radio" || !element.name) return true;
    const group = elements.filter((candidate): candidate is HTMLInputElement => (
      candidate instanceof HTMLInputElement && candidate.type === "radio"
      && candidate.name === element.name && candidate.form === element.form
    ));
    return element === (group.find((radio) => radio.checked) ?? group[0]);
  }).sort((a, b) => (a.tabIndex || Infinity) - (b.tabIndex || Infinity));
}
