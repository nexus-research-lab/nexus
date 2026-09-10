// INPUT: 调用者明确请求复制的文本与浏览器 Clipboard 能力。
// OUTPUT: 成功/失败结果；异步 API 不可用时回退原生复制并恢复原焦点。
// POS: 共享浏览器剪贴板适配，不拥有提示文案、计时器或业务数据。
export async function writeTextToClipboard(text: string): Promise<boolean> {
  if (text.length === 0) {
    return false;
  }
  if (canUseAsyncClipboard()) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      return writeTextWithLegacyClipboard(text);
    }
  }
  return writeTextWithLegacyClipboard(text);
}

function canUseAsyncClipboard(): boolean {
  return (
    typeof window !== "undefined"
    && window.isSecureContext
    && typeof navigator !== "undefined"
    && typeof navigator.clipboard?.writeText === "function"
  );
}

function writeTextWithLegacyClipboard(text: string): boolean {
  if (
    typeof document === "undefined"
    || typeof document.execCommand !== "function"
    || document.body == null
  ) {
    return false;
  }

  const activeElement = document.activeElement;
  const textArea = document.createElement("textarea");
  textArea.value = text;
  textArea.setAttribute("aria-hidden", "true");
  textArea.setAttribute("readonly", "true");
  textArea.style.position = "fixed";
  textArea.style.top = "0";
  textArea.style.left = "0";
  textArea.style.width = "1px";
  textArea.style.height = "1px";
  textArea.style.opacity = "0";
  textArea.style.pointerEvents = "none";
  textArea.style.zIndex = "-1";

  document.body.appendChild(textArea);
  textArea.focus({ preventScroll: true });
  textArea.select();
  textArea.setSelectionRange(0, text.length);

  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    textArea.remove();
    restoreFocus(activeElement);
  }
}

function restoreFocus(element: Element | null): void {
  if (element instanceof HTMLElement) {
    try {
      element.focus({ preventScroll: true });
    } catch {
      element.focus();
    }
  }
}
