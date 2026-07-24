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
  headerVisible?: boolean;
  contentClassName?: string;
  className?: string;
  children: ReactNode;
}

/** 中文注释：代码块壳层只在消息区复用，直接收进组件层，避免全局样式继续承担细节实现。 */
export function CodeShell({
  language,
  rightSlot: rightSlot,
  headerVisible = false,
  contentClassName: contentClassName,
  className: className,
  children,
}: CodeShellProps) {
  return (
    <div
      className={cn(
        "content-code-shell relative",
        className,
      )}
      data-code-header-visible={headerVisible ? "true" : undefined}
    >
      {language || rightSlot ? (
        <div
          className="content-code-header"
        >
          {language ? (
            <span
              className="content-code-label message-code-font"
            >
              {language}
            </span>
          ) : null}
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
