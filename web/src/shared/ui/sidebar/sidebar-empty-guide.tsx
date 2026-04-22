/**
 * =====================================================
 * @File   : sidebar-empty-guide.tsx
 * @Date   : 2026-04-15
 * @Author : leemysw
 * 侧边栏空状态引导 — 轻量卡片结构，紧凑贴合 sidebar 密度。
 * =====================================================
 */

import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface SidebarEmptyGuideProps {
  icon: LucideIcon;
  title: string;
  description: string;
  action_label?: string;
  on_action?: () => void;
  class_name?: string;
}

export function SidebarEmptyGuide({
  icon: Icon,
  title,
  description,
  action_label,
  on_action,
  class_name,
}: SidebarEmptyGuideProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-1 rounded-[12px] border border-(--divider-subtle-color) px-2.5 py-2",
        class_name,
      )}
    >
      <div className="flex items-center gap-1.5 text-(--text-muted)">
        <Icon className="h-3.5 w-3.5 shrink-0" />
        <span className="text-[11px] font-semibold">{title}</span>
      </div>
      <p className="text-[11px] leading-relaxed text-(--text-soft)">
        {description}
      </p>
      {action_label && on_action ? (
        <button
          className="mt-0.5 inline-flex w-fit items-center gap-1 rounded-[8px] bg-(--surface-interactive-hover-background) px-2 py-[3px] text-[11px] font-semibold text-(--primary) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-active-background)"
          onClick={on_action}
          type="button"
        >
          {action_label}
        </button>
      ) : null}
    </div>
  );
}
