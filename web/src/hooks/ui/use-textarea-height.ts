import { RefObject, useEffect, useRef } from "react";
import { prepare, layout } from "@chenglou/pretext";

// ─── useTextareaHeight ────────────────────────────────────────────────────────
//
// Replaces the scrollHeight-reflow pattern:
//   textarea.style.height = "auto";
//   textarea.style.height = `${textarea.scrollHeight}px`;
//
// Reading scrollHeight forces the browser to flush pending styles and perform
// a full layout (synchronous reflow). At 60 fps this is fine for a single
// element, but it blocks the main thread and can cause visible jank when the
// page is already doing other work (e.g. streaming tokens).
//
// pretext layout() does the same line-break arithmetic in pure JS — no DOM
// access, no reflow. We measure the container width once (cheap ResizeObserver)
// and recompute height on every value change without touching the DOM until
// we have the final pixel value.
//
// Usage:
//   const textareaRef = useRef<HTMLTextAreaElement>(null);
//   useTextareaHeight(textareaRef, value, { minHeight: 24, maxHeight: 128 });
//
// The hook writes `style.height` on the ref directly (same as the old pattern)
// so no React state / re-render is needed.

interface UseTextareaHeightOptions {
  /** Minimum height in px (default 24) */
  minHeight?: number;
  /** Maximum height in px, element scrolls beyond this (default 128) */
  maxHeight?: number;
  /** Line height in px matching the textarea's CSS (default 24) */
  lineHeight?: number;
  /** Extra vertical padding inside the textarea in px (default 0) */
  paddingY?: number;
}

export function useTextareaHeight(
  ref: RefObject<HTMLTextAreaElement | null>,
  value: string,
  {
    minHeight = 24,
    maxHeight = 128,
    lineHeight = 24,
    paddingY = 0,
  }: UseTextareaHeightOptions = {},
): void {
  // Cache container width across renders — only update on resize
  const widthRef = useRef(0);
  const fontRef = useRef("");

  // Measure container width + font once after mount, then watch for resizes
  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const sample = () => {
      // contentRect excludes padding/border — matches what text actually fills
      const rect = el.getBoundingClientRect();
      // Approximate inner width: subtract horizontal padding
      const style = window.getComputedStyle(el);
      const paddingLeft = parseFloat(style.paddingLeft) || 0;
      const paddingRight = parseFloat(style.paddingRight) || 0;
      widthRef.current = Math.max(1, rect.width - paddingLeft - paddingRight);
      fontRef.current = style.font || `400 15px ui-sans-serif, system-ui, sans-serif`;
    };

    sample();

    const observer = new ResizeObserver(sample);
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref]);

  // Recompute height without reflow on every value change
  useEffect(() => {
    const el = ref.current;
    if (!el || widthRef.current <= 0) return;

    let contentHeight: number;
    try {
      // pretext measures the full text including \n hard breaks
      const prepared = prepare(value || " ", fontRef.current);
      const result = layout(prepared, widthRef.current, lineHeight);
      contentHeight = result.height + paddingY;
    } catch {
      // Fallback: count newlines × lineHeight (rough but reflow-free)
      const lines = (value.match(/\n/g) ?? []).length + 1;
      contentHeight = lines * lineHeight + paddingY;
    }

    const clamped = Math.min(Math.max(contentHeight, minHeight), maxHeight);
    el.style.height = `${clamped}px`;
    el.style.overflowY = "auto";
    if (
      contentHeight > maxHeight &&
      document.activeElement === el &&
      el.selectionStart === value.length &&
      el.selectionEnd === value.length
    ) {
      el.scrollTop = el.scrollHeight;
    }
  }, [value, lineHeight, minHeight, maxHeight, paddingY, ref]);
}
