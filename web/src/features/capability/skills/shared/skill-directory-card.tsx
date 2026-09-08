/**
 * INPUT: Skill 身份、标题、一行摘要、必要元数据与领域动作。
 * OUTPUT: 已安装、更新和社区结果共用的两行摘要目录卡片。
 * POS: Skill 目录视觉合同；摘要用于选择，完整正文留给详情。
 */
"use client";

import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { WorkspaceCatalogCard } from "@/shared/ui/workspace/catalog/workspace-catalog-card";

interface SkillDirectoryCardProps {
  action?: ReactNode;
  badges?: ReactNode;
  busy?: boolean;
  description?: string;
  meta?: ReactNode;
  onSelect: () => void;
  seed: string;
  title: string;
}

/** 中文注释：能力页所有 Skill 结果共用同一信息层级和点击区域。 */
export function SkillDirectoryCard({
  action,
  badges,
  busy = false,
  description,
  meta,
  onSelect,
  seed,
  title,
}: SkillDirectoryCardProps) {
  return (
    <WorkspaceCatalogCard
      aria-busy={busy || undefined}
      className={cn(
        "group h-full",
        !meta && "min-h-[116px]",
      )}
      muted={busy}
      primaryAction={{ label: title, onClick: onSelect }}
      size="compact"
    >
      <div className="pointer-events-none relative z-10 grid w-full min-w-0 grid-cols-[40px_minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2">
        <UiSeededAvatar seed={seed} />
        <div className="flex min-h-10 min-w-0 items-center">
          <div className="flex w-full min-w-0 flex-wrap items-center gap-1.5">
            <h3 className={cn(
              "min-w-0 flex-1 truncate",
              getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
            )}>
              {title}
            </h3>
            {badges}
          </div>
        </div>
        {action ? (
          <div className="pointer-events-none flex min-h-10 shrink-0 items-center gap-1.5">
            {action}
          </div>
        ) : null}

        {description ? (
          <p className={cn(
            "col-span-3 min-h-9 line-clamp-2",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {description}
          </p>
        ) : null}
        {meta ? (
          <div className={cn(
            "col-span-3 flex min-w-0 items-center gap-1.5 overflow-hidden",
            getUiTypographyClassName({ role: "caption", tone: "soft" }),
          )}>
            {meta}
          </div>
        ) : null}
      </div>
    </WorkspaceCatalogCard>
  );
}
