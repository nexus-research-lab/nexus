/**
 * =====================================================
 * @File   : code-shell.tsx
 * @Date   : 2026-04-05 15:08
 * @Author : leemysw
 * 2026-04-05 15:08   Create
 * =====================================================
 */

"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface CodeShellProps {
  language: string;
  right_slot?: ReactNode;
  content_class_name?: string;
  class_name?: string;
  children: ReactNode;
}

/** 中文注释：代码块壳层只在消息区复用，直接收进组件层，避免全局样式继续承担细节实现。 */
export function CodeShell({
  language,
  right_slot,
  content_class_name,
  class_name,
  children,
}: CodeShellProps) {
  return (
    <div
      className={cn(
        "relative my-4 overflow-hidden rounded-[22px] border",
        class_name,
      )}
      style={{
        background: "var(--surface-panel-subtle-background)",
        borderColor: "var(--surface-panel-subtle-border)",
      }}
    >
      <div
        className="flex items-center justify-between border-b px-4 py-2"
        style={{
          background: "var(--surface-interactive-active-background)",
          borderColor: "var(--divider-subtle-color)",
        }}
      >
        <div className="flex items-center gap-2">
          <div className="flex gap-1.5">
            <div className="h-2.5 w-2.5 rounded-full border border-red-500/50 bg-red-500/18" />
            <div className="h-2.5 w-2.5 rounded-full border border-yellow-500/50 bg-yellow-500/18" />
            <div className="h-2.5 w-2.5 rounded-full border border-green-500/50 bg-green-500/18" />
          </div>
          <span
            className="message-cjk-code-font ml-2 text-xs"
            style={{ color: "var(--text-muted)" }}
          >
            {language || "text"}
          </span>
        </div>
        {right_slot}
      </div>

      <div className={content_class_name}>
        {children}
      </div>
    </div>
  );
}
