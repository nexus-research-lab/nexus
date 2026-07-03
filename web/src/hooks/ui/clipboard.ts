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
