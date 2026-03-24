"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Brain } from "lucide-react";
import { cn } from "@/lib/utils";
import { MarkdownRenderer } from "../markdown-renderer";

interface ThinkingBlockProps {
    thinking: string;
    isStreaming?: boolean;
}

export function ThinkingBlock({ thinking, isStreaming }: ThinkingBlockProps) {
    // 默认展开思考过程，流式状态仅影响展示样式，不影响折叠状态。
    const [isExpanded, setIsExpanded] = useState(true);

    if (!thinking) return null;

  return (
        <div className="workspace-card radius-shell-sm my-2 overflow-hidden transition-all duration-300">
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="flex w-full items-center justify-between px-3 py-2 text-xs text-slate-700/58 transition-colors hover:bg-white/18"
            >
                <div className="flex items-center gap-2">
                    <div className="workspace-chip radius-shell-sm flex h-7 w-7 items-center justify-center">
                        <Brain className={cn("h-3.5 w-3.5", isStreaming ? "animate-pulse text-sky-600" : "text-slate-800/70")} />
                    </div>
                    <span className="font-medium uppercase tracking-[0.14em]">
                        {isStreaming ? "整理思路中" : "协作思路"}
                    </span>
                </div>
                {isExpanded ? (
                    <ChevronDown className="w-3.5 h-3.5" />
                ) : (
                    <ChevronRight className="w-3.5 h-3.5" />
                )}
            </button>

            {isExpanded && (
                <div className="border-t workspace-divider px-4 py-3 font-mono text-xs text-slate-700/78">
                    <MarkdownRenderer content={thinking} isStreaming={isStreaming} />
                </div>
            )}
        </div>
    );
}
