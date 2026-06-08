export async function write_text_to_clipboard(text: string): Promise<boolean> {
  if (text.length === 0) {
    return false;
  }
  if (can_use_async_clipboard()) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      return write_text_with_legacy_clipboard(text);
    }
  }
  return write_text_with_legacy_clipboard(text);
}

function can_use_async_clipboard(): boolean {
  return (
    typeof window !== "undefined"
    && window.isSecureContext
    && typeof navigator !== "undefined"
    && typeof navigator.clipboard?.writeText === "function"
  );
}

function write_text_with_legacy_clipboard(text: string): boolean {
  if (
    typeof document === "undefined"
    || typeof document.execCommand !== "function"
    || document.body == null
  ) {
    return false;
  }

  const active_element = document.activeElement;
  const text_area = document.createElement("textarea");
  text_area.value = text;
  text_area.setAttribute("aria-hidden", "true");
  text_area.setAttribute("readonly", "true");
  text_area.style.position = "fixed";
  text_area.style.top = "0";
  text_area.style.left = "0";
  text_area.style.width = "1px";
  text_area.style.height = "1px";
  text_area.style.opacity = "0";
  text_area.style.pointerEvents = "none";
  text_area.style.zIndex = "-1";

  document.body.appendChild(text_area);
  text_area.focus({ preventScroll: true });
  text_area.select();
  text_area.setSelectionRange(0, text.length);

  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    text_area.remove();
    restore_focus(active_element);
  }
}

function restore_focus(element: Element | null): void {
  if (element instanceof HTMLElement) {
    try {
      element.focus({ preventScroll: true });
    } catch {
      element.focus();
    }
  }
}
