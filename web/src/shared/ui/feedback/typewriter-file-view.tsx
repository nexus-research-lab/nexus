"use client";

import { useEffect, useRef, useState } from "react";
import { prepare, layout } from "@chenglou/pretext";

// Font matching the textarea: font-mono text-sm leading-6
const MONO_FONT = "400 14px ui-monospace, SFMono-Regular, Menlo, monospace";
const LINE_HEIGHT = 24; // leading-6 = 1.5rem = 24px

interface TypewriterFileViewProps {
  /** The full content being written (grows over time) */
  content: string;
  /** Width of the view container in px; used to measure line wraps */
  container_width?: number;
  class_name?: string;
}

/**
 * Replaces the plain textarea when an agent is actively writing a file.
 *
 * Uses pretext to measure how many lines the current content fills, then
 * displays a live line-count badge and a blinking write-cursor at the end.
 * The text itself renders in a read-only pre element to match the textarea style.
 */
export function TypewriterFileView({
  content,
  container_width,
  class_name,
}: TypewriterFileViewProps) {
  const [lineCount, setLineCount] = useState(1);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (!container_width || container_width <= 0) return;
    try {
      const prepared = prepare(content, MONO_FONT);
      const result = layout(prepared, container_width, LINE_HEIGHT);
      setLineCount(Math.max(1, Math.round(result.height / LINE_HEIGHT)));
    } catch {
      // Fallback: count raw newlines
      setLineCount(content.split("\n").length);
    }
  }, [content, container_width]);

  // Scroll to bottom as content grows
  useEffect(() => {
    const el = preRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [content]);

  return (
    <div className={`relative flex h-full min-h-0 flex-col overflow-hidden font-mono text-sm leading-6 ${class_name ?? ""}`}>
      {/* Line count badge */}
      <div className="absolute right-4 top-3 z-10 flex items-center gap-1.5 rounded-full bg-primary/10 px-2.5 py-0.5 text-[10px] font-semibold text-primary/80">
        <span
          className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary"
          style={{ animationDuration: "0.8s" }}
        />
        {lineCount} {lineCount === 1 ? "line" : "lines"}
      </div>

      <pre
        ref={preRef}
        className="soft-scrollbar surface-card h-full w-full overflow-auto whitespace-pre-wrap break-all rounded-[28px] p-5 text-(--text-strong)"
        style={{ wordBreak: "break-word" }}
      >
        {content}
        <WriteCursor />
      </pre>
    </div>
  );
}

// A block-level write cursor: thicker than the streaming cursor,
// using a bright amber/green accent to signal "agent writing"
let writeCursorStyleInjected = false;
function ensureWriteCursorStyle() {
  if (writeCursorStyleInjected || typeof document === "undefined") return;
  writeCursorStyleInjected = true;
  const style = document.createElement("style");
  style.textContent = `
    @keyframes _nexus_write_cursor {
      0%   { opacity: 1; }
      48%  { opacity: 1; }
      52%  { opacity: 0; }
      100% { opacity: 0; }
    }
    .nexus-write-cursor {
      display: inline-block;
      width: 8px;
      height: 1em;
      margin-left: 1px;
      margin-bottom: -0.12em;
      border-radius: 2px;
      background: linear-gradient(180deg, var(--success) 0%, color-mix(in srgb, var(--success) 78%, black) 100%);
      box-shadow: 0 0 8px 2px color-mix(in srgb, var(--success) 50%, transparent);
      animation: _nexus_write_cursor 0.8s step-end infinite;
      vertical-align: baseline;
    }
  `;
  document.head.appendChild(style);
}

function WriteCursor() {
  useEffect(() => { ensureWriteCursorStyle(); }, []);
  return <span className="nexus-write-cursor" aria-hidden />;
}
