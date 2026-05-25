/**
 * =====================================================
 * @File   : skill-state-pill.tsx
 * @Date   : 2026-04-16 14:16
 * @Author : leemysw
 * 2026-04-16 14:16   Create
 * =====================================================
 */

import { UiBadge } from "@/shared/ui/badge";

interface SkillStatePillProps {
  children: string;
  tone?: "neutral" | "success" | "warning";
}

export function SkillStatePill({
  children,
  tone = "neutral",
}: SkillStatePillProps) {
  return (
    <UiBadge tone={tone === "neutral" ? "default" : tone}>
      {children}
    </UiBadge>
  );
}
