/**
 * =====================================================
 * @File   : dialog-keyboard.ts
 * @Date   : 2026-04-16 14:00
 * @Author : leemysw
 * 2026-04-16 14:00   Create
 * =====================================================
 */

export function close_on_escape(event: KeyboardEvent, on_close: () => void) {
  if (event.key !== "Escape") {
    return;
  }
  event.preventDefault();
  on_close();
}
