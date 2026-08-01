"use client";

import type { ReactNode } from "react";

import { UiSeededAvatar } from "@/shared/ui/display/seeded-avatar";
import { WorkspaceCatalogCard } from "@/shared/ui/workspace/catalog/workspace-catalog-card";

interface SkillDirectoryCardProps {
  action?: ReactNode;
  badges?: ReactNode;
  busy?: boolean;
  description: string;
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
      className="group relative h-full overflow-hidden hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)"
      muted={busy}
      size="compact"
    >
      <button
        aria-label={title}
        className="absolute inset-0 z-0 rounded-[inherit] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
        onClick={onSelect}
        type="button"
      />

      <div className="pointer-events-none relative z-10 grid w-full min-w-0 grid-cols-[40px_minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2">
        <UiSeededAvatar seed={seed} />
        <div className="min-w-0 pt-0.5">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <h3 className="min-w-0 flex-1 truncate text-[14px] font-semibold text-(--text-strong)">
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

        <p className="col-span-3 min-h-9 line-clamp-2 text-compact leading-[1.125rem] text-(--text-muted)">
          {description}
        </p>
        {meta ? (
          <div className="col-span-3 flex min-w-0 items-center gap-1.5 overflow-hidden text-2xs leading-4 text-(--text-soft)">
            {meta}
          </div>
        ) : null}
      </div>
    </WorkspaceCatalogCard>
  );
}
