/**
 * =====================================================
 * @File   : dialog-keyboard.ts
 * @Date   : 2026-04-16 14:00
 * @Author : leemysw
 * 2026-04-16 14:00   Create
 * =====================================================
 */

export function closeOnEscape(event: KeyboardEvent, onClose: () => void) {
  if (event.key !== "Escape") {
    return;
  }
  event.preventDefault();
  onClose();
}
