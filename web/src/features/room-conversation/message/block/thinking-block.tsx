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
      <div className="mb-2 flex items-center gap-2 text-[11px] font-medium text-slate-500">
        <Brain className={is_streaming ? "h-3.5 w-3.5 animate-pulse text-sky-500" : "h-3.5 w-3.5 text-slate-400"} />
        <span>{is_streaming ? "正在整理思路" : "协作思路"}</span>
      </div>
      <div className="text-[13px] leading-6 text-slate-700">
        <MarkdownRenderer content={thinking} is_streaming={is_streaming} />
      </div>
    </div>
  );
}
