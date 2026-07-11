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

export function registerDialogModal(): symbol {
  const token = Symbol("ui-dialog");
  dialogStack.push(token);
  lockBodyScroll();
  return token;
}

export function isTopDialogModal(token: symbol): boolean {
  return dialogStack.at(-1) === token;
}

export function unregisterDialogModal(token: symbol): void {
  const index = dialogStack.lastIndexOf(token);
  if (index >= 0) {
    dialogStack.splice(index, 1);
  }
  unlockBodyScroll();
}
