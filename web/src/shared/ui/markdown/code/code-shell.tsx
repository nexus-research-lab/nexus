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

import { cn } from "@/shared/ui/class-name";

interface CodeShellProps {
  language?: string;
  rightSlot?: ReactNode;
  contentClassName?: string;
  className?: string;
  children: ReactNode;
}

/** 中文注释：代码块壳层只在消息区复用，直接收进组件层，避免全局样式继续承担细节实现。 */
export function CodeShell({
  language,
  rightSlot: rightSlot,
  contentClassName: contentClassName,
  className: className,
  children,
}: CodeShellProps) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-[10px] border",
        className,
      )}
      style={{
        background: "color-mix(in srgb, var(--surface-panel-background) 90%, transparent)",
        borderColor: "color-mix(in srgb, var(--surface-panel-subtle-border) 80%, transparent)",
      }}
    >
      {language || rightSlot ? (
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
          {rightSlot ? (
            <div className="shrink-0">
              {rightSlot}
            </div>
          ) : null}
        </div>
      ) : null}
      <div className={contentClassName}>
        {children}
      </div>
    </div>
  );
}
