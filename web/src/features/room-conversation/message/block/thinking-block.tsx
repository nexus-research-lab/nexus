"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Brain } from "lucide-react";
import { cn } from "@/lib/utils";
import { MarkdownRenderer } from "../markdown-renderer";

interface ThinkingBlockProps {
  thinking: string;
  is_streaming?: boolean;
}

export function ThinkingBlock({ thinking, is_streaming }: ThinkingBlockProps) {
    // 流式阶段默认展开，完成后默认收起，避免长思路长期占住首层。
    const [isExpanded, setIsExpanded] = useState(Boolean(is_streaming));

    if (!thinking) return null;

  return (
        <div className="my-2 overflow-hidden rounded-[18px] border border-white/24 bg-white/12 transition-all duration-300">
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="flex w-full items-center justify-between px-3 py-2 text-xs text-slate-700/58 transition-colors hover:bg-white/12"
            >
                <div className="flex items-center gap-2">
                    <div className="flex h-7 w-7 items-center justify-center rounded-full border border-white/32 bg-white/22">
                        <Brain className={cn("h-3.5 w-3.5", is_streaming ? "animate-pulse text-sky-600" : "text-slate-800/70")} />
                    </div>
                    <span className="font-medium uppercase tracking-[0.14em]">
                        {is_streaming ? "正在整理" : "协作思路"}
                    </span>
                </div>
                {isExpanded ? (
                    <ChevronDown className="w-3.5 h-3.5" />
                ) : (
                    <ChevronRight className="w-3.5 h-3.5" />
                )}
            </button>

            {isExpanded && (
                <div className="border-t border-white/18 px-4 py-3 text-[13px] leading-6 text-slate-700/82">
                    <MarkdownRenderer content={thinking} is_streaming={is_streaming} />
                </div>
            )}
        </div>
    );
}
