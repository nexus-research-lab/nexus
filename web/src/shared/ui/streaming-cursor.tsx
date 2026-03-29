"use client";

import { useEffect, useRef } from "react";
import { prepare, layout } from "@chenglou/pretext";

interface StreamingCursorProps {
  text: string;
  font?: string;
  line_height?: number;
  container_width?: number;
  class_name?: string;
}

const DEFAULT_FONT = "400 14px ui-sans-serif, system-ui, sans-serif";
const DEFAULT_LINE_HEIGHT = 28;

let styleInjected = false;
function ensureStyle() {
  if (styleInjected || typeof document === "undefined") return;
  styleInjected = true;
  const style = document.createElement("style");
  // Three-phase blink: fully visible → dim → fully visible
  // width 2.5px, accent color, subtle text-shadow glow
  style.textContent = `
    @keyframes _nexus_cursor_blink {
      0%   { opacity: 1;    transform: scaleY(1); }
      40%  { opacity: 1;    transform: scaleY(1); }
      55%  { opacity: 0.08; transform: scaleY(0.92); }
      70%  { opacity: 1;    transform: scaleY(1); }
      100% { opacity: 1;    transform: scaleY(1); }
    }
    .nexus-cursor {
      display: inline-block;
      width: 2.5px;
      height: 1.15em;
      border-radius: 1.5px;
      margin-left: 2px;
      margin-bottom: -0.1em;
      vertical-align: baseline;
      background: linear-gradient(180deg, #6366f1 0%, #818cf8 100%);
      box-shadow: 0 0 6px 1px rgba(99,102,241,0.55);
      animation: _nexus_cursor_blink 1.1s cubic-bezier(0.4,0,0.6,1) infinite;
      transform-origin: center 60%;
    }
  `;
  document.head.appendChild(style);
}

export function StreamingCursor({
  text,
  font = DEFAULT_FONT,
  line_height = DEFAULT_LINE_HEIGHT,
  container_width,
  class_name,
}: StreamingCursorProps) {
  const ref = useRef<HTMLSpanElement>(null);

  useEffect(() => { ensureStyle(); }, []);

  useEffect(() => {
    if (!container_width || !text) return;
    try {
      const prepared = prepare(text, font);
      void layout(prepared, container_width, line_height);
    } catch { /* fallback */ }
  }, [text, font, container_width, line_height]);

  return (
    <span
      ref={ref}
      className={`nexus-cursor${class_name ? ` ${class_name}` : ""}`}
      aria-hidden
    />
  );
}

// Lightweight variant — no pretext measurement, just the visual
export function InlineStreamingCursor({ class_name }: { class_name?: string }) {
  useEffect(() => { ensureStyle(); }, []);
  return (
    <span
      className={`nexus-cursor${class_name ? ` ${class_name}` : ""}`}
      aria-hidden
    />
  );
}
