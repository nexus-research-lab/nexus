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
    // If it's streaming, default to expanded so the user can see it typing.
    const [isExpanded, setIsExpanded] = useState(isStreaming ?? true);

    if (!thinking) return null;

    return (
        <div className="my-2 border rounded-lg bg-muted/30 border-muted/50 overflow-hidden transition-all duration-300">
            <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="w-full flex items-center justify-between px-3 py-2 text-xs text-muted-foreground hover:bg-muted/50 transition-colors"
            >
                <div className="flex items-center gap-2">
                    <Brain className={cn("w-3.5 h-3.5", isStreaming ? "animate-pulse text-accent" : "")} />
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
                <div className="px-4 py-3 border-t border-muted/50 text-xs text-muted-foreground/80 font-mono">
                    <MarkdownRenderer content={thinking} isStreaming={isStreaming} />
                </div>
            )}
        </div>
    );
}
