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
  language?: string;
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
        "relative overflow-hidden rounded-[10px] border",
        class_name,
      )}
      style={{
        background: "color-mix(in srgb, var(--surface-panel-background) 90%, transparent)",
        borderColor: "color-mix(in srgb, var(--surface-panel-subtle-border) 80%, transparent)",
      }}
    >
      {language || right_slot ? (
        <div
          className="flex items-center justify-between gap-2 border-b px-2.5"
          style={{ borderColor: "var(--divider-subtle-color)" }}
        >
          <span
            className="message-cjk-code-font truncate text-[10px] lowpercase tracking-[0.12em]"
            style={{ color: "var(--text-muted)" }}
          >
            {language || "text"}
          </span>
          {right_slot ? (
            <div className="shrink-0">
              {right_slot}
            </div>
          ) : null}
        </div>
      ) : null}
      <div className={content_class_name}>
        {children}
      </div>
    </div>
  );
}
