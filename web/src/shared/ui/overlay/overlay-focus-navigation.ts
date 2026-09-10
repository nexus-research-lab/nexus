// INPUT: 关闭中的浮层根、已经归还的锚点焦点与前后方向。
// OUTPUT: 跳过即将移除的 Portal，从锚点续接同一页面或模态中的原生 Tab 顺序。
// POS: Menu 与非模态详情浮层共用的焦点退出规则；调用方负责关闭及归还真实锚点。

import { getTabbableElements } from "@/shared/lib/browser/focus-navigation";

export function focusAfterAnchoredOverlayExit(overlayRoots: readonly HTMLElement[], backwards: boolean): void {
  const anchor = document.activeElement;
  if (!(anchor instanceof HTMLElement)) return;
  const exitingRoots = overlayRoots.filter((root) => !root.contains(anchor));
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
