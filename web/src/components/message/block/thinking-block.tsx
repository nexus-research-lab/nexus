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
        <div className="neo-inset radius-shell-sm my-2 overflow-hidden transition-all duration-300">
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="w-full flex items-center justify-between px-3 py-2 text-xs text-muted-foreground hover:bg-muted/50 transition-colors"
            >
                <div className="flex items-center gap-2">
                    <div className="neo-pill radius-shell-sm flex h-7 w-7 items-center justify-center">
                        <Brain className={cn("w-3.5 h-3.5", isStreaming ? "animate-pulse text-accent" : "")} />
                    </div>
                    <span className="font-medium uppercase tracking-wider">
                        {isStreaming ? "Thinking..." : "Thought Process"}
                    </span>
                </div>
                {isExpanded ? (
                    <ChevronDown className="w-3.5 h-3.5" />
                ) : (
                    <ChevronRight className="w-3.5 h-3.5" />
                )}
            </button>

            {isExpanded && (
                <div className="border-t border-muted/50 px-4 py-3 text-xs text-muted-foreground/80 font-mono">
                    <MarkdownRenderer content={thinking} isStreaming={isStreaming} />
                </div>
            )}
        </div>
    );
}
