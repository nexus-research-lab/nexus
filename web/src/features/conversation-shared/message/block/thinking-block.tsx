"use client";

import { Brain } from "lucide-react";
import { MarkdownRenderer } from "../markdown-renderer";

interface ThinkingBlockProps {
  thinking: string;
  is_streaming?: boolean;
}

export function ThinkingBlock({ thinking, is_streaming }: ThinkingBlockProps) {
  if (!thinking) return null;

  return (
    <div className="border-l border-slate-200/90 pl-4">
      <div className="flex items-center gap-2 text-[11px] font-medium text-slate-500">
        <Brain className={is_streaming ? "h-3 w-3 animate-pulse text-sky-500" : "h-3 w-3 text-slate-400"} />
        <span>{is_streaming ? "Thinking……" : "Thought"}</span>
      </div>
      <div className="text-[11px] leading-4 text-slate-700">
        <MarkdownRenderer content={thinking} is_streaming={is_streaming} />
      </div>
    </div>
  );
}
