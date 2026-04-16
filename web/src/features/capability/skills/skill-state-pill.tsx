/**
 * =====================================================
 * @File   : skill-state-pill.tsx
 * @Date   : 2026-04-16 14:16
 * @Author : leemysw
 * 2026-04-16 14:16   Create
 * =====================================================
 */

import { cn } from "@/lib/utils";

interface SkillStatePillProps {
  children: string;
  tone?: "neutral" | "success" | "warning";
}

export function SkillStatePill({
  children,
  tone = "neutral",
}: SkillStatePillProps) {
  const tone_class_name =
    tone === "warning"
      ? "border-amber-200/80 bg-amber-50/88 text-amber-700"
      : tone === "success"
        ? "border-emerald-200/80 bg-emerald-50/90 text-emerald-700"
        : "border-(--surface-panel-subtle-border) bg-(--surface-panel-subtle-background) text-(--text-soft)";

  return (
    <span
      className={cn(
        "inline-flex h-6 items-center rounded-full border px-2.5 text-[11px] font-medium leading-none tracking-[0.01em]",
        tone_class_name,
      )}
    >
      {children}
    </span>
  );
}
