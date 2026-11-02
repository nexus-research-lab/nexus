// INPUT: Dialog 的挂载/卸载令牌及其真实模态根。
// OUTPUT: 栈顶模态身份、引用计数滚动锁、共享 Overlay 的当前模态范围与被覆盖父模态的 nested 标记。
// POS: 模态运行时所有者；浮层父子关系和关闭仲裁由 Overlay runtime 负责。

import {
  registerModalOverlayScope,
  unregisterModalOverlayScope,
} from "@/shared/ui/overlay/overlay-dismissal-runtime";

const dialogStack: symbol[] = [];
const dialogRoots = new Map<symbol, HTMLElement>();
let scrollLockCount = 0;
let bodyOverflowBeforeLock = "";

function lockBodyScroll(): void {
  if (scrollLockCount === 0) {
    bodyOverflowBeforeLock = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  }
  scrollLockCount += 1;
}

function unlockBodyScroll(): void {
  scrollLockCount = Math.max(0, scrollLockCount - 1);
  if (scrollLockCount === 0) {
    document.body.style.overflow = bodyOverflowBeforeLock;
    bodyOverflowBeforeLock = "";
  }
}

/**
 * 被上层模态覆盖的父模态打上 `data-nested-dialog-open`，由主题 recipe 轻微虚化，视线只落在栈顶。
 * 栈顶 DOM 若嵌在父根内部（未经 Portal），虚化父级会连带模糊栈顶，因此只标记不包含栈顶的根。
 */
function syncNestedDialogMarks(): void {
  const topToken = dialogStack.at(-1);
  const topRoot = topToken ? dialogRoots.get(topToken) : undefined;
  for (const [token, root] of dialogRoots) {
    const covered = token !== topToken && topRoot !== undefined && !root.contains(topRoot);
    root.toggleAttribute("data-nested-dialog-open", covered);
  }
}

export function registerDialogModal(root?: HTMLElement | null): symbol {
  const token = Symbol("ui-dialog");
  dialogStack.push(token);
  if (root) {
    registerModalOverlayScope(token, root);
    dialogRoots.set(token, root);
  }
  lockBodyScroll();
  syncNestedDialogMarks();
  return token;
}

export function isTopDialogModal(token: symbol): boolean {
  return dialogStack.at(-1) === token;
}

export function unregisterDialogModal(token: symbol): void {
  const index = dialogStack.lastIndexOf(token);
  if (index < 0) {
    return;
  }

  dialogStack.splice(index, 1);
  dialogRoots.get(token)?.removeAttribute("data-nested-dialog-open");
  dialogRoots.delete(token);
  unregisterModalOverlayScope(token);
  unlockBodyScroll();
  syncNestedDialogMarks();
}
