// INPUT: 代码语言、非交互标签与可选操作槽。
// OUTPUT: 本地化代码组与统一操作层，键盘直接进入实际按钮。
// POS: 静态/流式代码共用外壳，不管理复制或代码渲染。
"use client";

import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

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
  rightSlot,
  contentClassName,
  className,
  children,
}: CodeShellProps) {
  const { t } = useI18n();
  const accessibleLanguage = language?.trim() || "text";

  return (
    <div
      className={cn(
        "content-code-shell group/copy relative",
        className,
      )}
      aria-label={t("markdown.code.group", { language: accessibleLanguage })}
      role="group"
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
