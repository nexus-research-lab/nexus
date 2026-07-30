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
  const accessibleLanguage = language?.trim() || "text";

  return (
    <div
      className={cn(
        "content-code-shell group/copy relative",
        className,
      )}
      aria-label={`${accessibleLanguage} code`}
      role="group"
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- 代码壳需要键盘聚焦以复现操作层的 focus-within 行为。
      tabIndex={0}
    >
      {rightSlot ? (
        <div className="content-code-copy-layer">
          <div className="content-code-copy-actions">
            {rightSlot}
          </div>
        </div>
      ) : null}
      {language ? <div className="content-code-label">{language}</div> : null}
      <div className={contentClassName}>
        {children}
      </div>
    </div>
  );
}
