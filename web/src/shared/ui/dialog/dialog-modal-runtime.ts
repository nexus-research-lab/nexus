// INPUT: Dialog 的挂载/卸载令牌及其真实模态根。
// OUTPUT: 栈顶模态身份、引用计数滚动锁与共享 Overlay 的当前模态范围。
// POS: 模态运行时所有者；浮层父子关系和关闭仲裁由 Overlay runtime 负责。

import {
  registerModalOverlayScope,
  unregisterModalOverlayScope,
} from "@/shared/ui/overlay/overlay-dismissal-runtime";

const dialogStack: symbol[] = [];
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

export function registerDialogModal(root?: HTMLElement | null): symbol {
  const token = Symbol("ui-dialog");
  dialogStack.push(token);
  if (root) {
    registerModalOverlayScope(token, root);
  }
  lockBodyScroll();
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
  unregisterModalOverlayScope(token);
  unlockBodyScroll();
}
