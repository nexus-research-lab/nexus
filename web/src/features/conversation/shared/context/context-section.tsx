/**
 * ContextSection 通用外壳
 *
 * room-context 详情面板中的统一 section 容器。
 * 提供 border-bottom + 标题行（图标 + 标题 + 可选 trailing）+ 子内容。
 */

import { type ReactNode } from "react";

interface ContextSectionProps {
  icon: ReactNode;
  title: string;
  trailing?: ReactNode;
  children: ReactNode;
}

export function ContextSection({ icon, title, trailing, children }: ContextSectionProps) {
  return (
    <section className="border-b divider-subtle px-4 py-4">
      <div className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-(--text-soft)">
        <div className="flex items-center gap-2">
          {icon}
          {title}
        </div>
        {trailing}
      </div>
      {children}
    </section>
  );
}
