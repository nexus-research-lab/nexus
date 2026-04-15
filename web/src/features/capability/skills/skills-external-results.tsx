import { Download, Loader2, Puzzle } from "lucide-react";
import { cn } from "@/lib/utils";

import {
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceCatalogTextAction,
  WorkspaceCatalogTitle,
  WorkspaceIconFrame,
} from "@/shared/ui/workspace/workspace-catalog-card";
import type { ExternalSkillSearchItem } from "@/types/skill";

import { formatInstalls } from "@/hooks/use-skill-marketplace";
import type { SkillMarketplaceController } from "@/hooks/use-skill-marketplace";

interface SkillsExternalResultsProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsExternalResults({ ctrl }: SkillsExternalResultsProps) {
  if (ctrl.external_loading) {
    return (
      <div className="flex items-center gap-2 py-12 justify-center text-sm text-(--text-soft)">
        <Loader2 className="h-4 w-4 animate-spin" />
        正在搜索社区技能...
      </div>
    );
  }

  if (ctrl.external_query && !ctrl.external_results.length) {
    return (
      <div className="rounded-[18px] border border-dashed border-(--divider-subtle-color) px-5 py-8 text-center text-sm text-(--text-soft)">
        暂无匹配结果，试试更具体的关键词
      </div>
    );
  }

  if (!ctrl.external_results.length) return null;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-[12px] text-(--text-soft)">
        <span>
          找到 <span className="font-bold text-(--text-strong)">{ctrl.external_results.length}</span> 个结果
        </span>
        <span>优先展示安装量更高的技能</span>
      </div>
      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        {ctrl.external_results.map((item: ExternalSkillSearchItem) => (
          <ExternalResultCard
            key={`${item.package_spec}@${item.skill_slug}`}
            busy_external_key={ctrl.busy_external_key}
            imported_external_sources={ctrl.imported_external_sources}
            item={item}
            on_import={() => void ctrl.handle_import_external(item)}
            on_preview={() => ctrl.handle_preview_external(item)}
          />
        ))}
      </div>
    </div>
  );
}

/* ── 单张外部结果卡片 ─────────────────────────── */

interface ExternalResultCardProps {
  item: ExternalSkillSearchItem;
  busy_external_key: string | null;
  imported_external_sources: Map<string, Set<string>>;
  on_preview: () => void;
  on_import: () => void;
}

function ExternalSkillStatePill({
  children,
  tone = "neutral",
}: {
  children: string;
  tone?: "neutral" | "success" | "warning";
}) {
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

function ExternalResultCard({
  item,
  busy_external_key,
  imported_external_sources,
  on_preview,
  on_import,
}: ExternalResultCardProps) {
  const imported_sources = imported_external_sources.get(item.skill_slug);
  const already_imported = imported_sources?.has(item.package_spec) ?? false;
  const has_name_conflict = !!imported_sources && !already_imported;
  const external_key = `${item.package_spec}@@${item.skill_slug}`;
  const is_busy = busy_external_key === external_key;
  const state_label = already_imported ? "已导入" : has_name_conflict ? "同名冲突" : "可导入";
  const state_tone = already_imported ? "success" : has_name_conflict ? "warning" : "neutral";

  return (
    <WorkspaceCatalogCard
      class_name="h-full"
      interactive
      onClick={on_preview}
      size="compact"
    >
      <WorkspaceCatalogHeader class_name="items-center gap-3.5">
        <WorkspaceIconFrame
          class_name="shrink-0 text-sky-600"
          size="sm"
          tone="primary"
        >
          <Puzzle className="h-4 w-4" />
        </WorkspaceIconFrame>

        <div className="min-w-0 flex-1">
          <WorkspaceCatalogTitle class_name="tracking-tight" size="sm" truncate>
            {item.title || item.skill_slug}
          </WorkspaceCatalogTitle>
          <p className="mt-1 flex items-center gap-2 truncate text-[11px] text-(--text-soft)">
            <span>{item.package_spec}</span>
            <span>·</span>
            <span>{formatInstalls(item.installs)} installs</span>
          </p>
        </div>
      </WorkspaceCatalogHeader>

      <WorkspaceCatalogBody grow>
        <WorkspaceCatalogDescription class_name="text-[12px] leading-[1.6]" lines={2}>
          {item.readme_markdown || item.description}
        </WorkspaceCatalogDescription>
      </WorkspaceCatalogBody>

      <WorkspaceCatalogFooter justify="end" onClick={(e) => e.stopPropagation()}>
        <div className="flex shrink-0 items-center gap-1.5">
          <ExternalSkillStatePill tone={state_tone}>
            {state_label}
          </ExternalSkillStatePill>
          {!already_imported && !has_name_conflict ? (
            <WorkspaceCatalogTextAction
              disabled={is_busy || has_name_conflict}
              onClick={on_import}
              tone="primary"
              class_name="px-1"
            >
              {is_busy ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <Download className="h-3 w-3" />
              )}
              导入
            </WorkspaceCatalogTextAction>
          ) : null}
        </div>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
