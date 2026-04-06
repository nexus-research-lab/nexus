import { Download, Loader2 } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import {
  WorkspaceCatalogBadge,
  WorkspaceCatalogCard,
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
      <div className="flex items-center gap-2 py-12 justify-center text-sm text-slate-500">
        <Loader2 className="h-4 w-4 animate-spin" />
        正在搜索社区技能...
      </div>
    );
  }

  if (ctrl.external_query && !ctrl.external_results.length) {
    return (
      <div className="surface-inset radius-shell-md px-5 py-8 text-center text-sm text-slate-500">
        暂无匹配结果，试试更具体的关键词
      </div>
    );
  }

  if (!ctrl.external_results.length) return null;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-[12px] text-slate-500">
        <span>
          找到 <span className="font-bold text-slate-800">{ctrl.external_results.length}</span> 个结果
        </span>
        <span>优先展示安装量更高的技能</span>
      </div>
      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        {ctrl.external_results.map((item: ExternalSkillSearchItem) => (
          <ExternalResultCard
            key={`${item.package_spec}@${item.skill_slug}`}
            busy_skill_name={ctrl.busy_skill_name}
            imported_skill_names={ctrl.imported_skill_names}
            item={item}
            on_import={() => void ctrl.handle_import_external(item)}
            on_preview={() => ctrl.set_preview_external_item(item)}
          />
        ))}
      </div>
    </div>
  );
}

/* ── 单张外部结果卡片 ─────────────────────────── */

interface ExternalResultCardProps {
  item: ExternalSkillSearchItem;
  busy_skill_name: string | null;
  imported_skill_names: Set<string>;
  on_preview: () => void;
  on_import: () => void;
}

function ExternalResultCard({
  item,
  busy_skill_name,
  imported_skill_names,
  on_preview,
  on_import,
}: ExternalResultCardProps) {
  const already_imported = imported_skill_names.has(item.skill_slug);
  const is_busy = busy_skill_name === item.skill_slug;

  return (
    <WorkspaceCatalogCard
      class_name="cursor-pointer rounded-[20px] px-5 py-4"
      onClick={on_preview}
    >
      {/* 标题行 */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="truncate text-[14px] font-bold tracking-tight text-slate-900">
            {item.title || item.skill_slug}
          </p>
          <p className="mt-0.5 flex items-center gap-2 truncate text-[11px] text-slate-400">
            <span>{item.package_spec}</span>
            <span>·</span>
            <span>{formatInstalls(item.installs)} installs</span>
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
          {already_imported ? (
            <WorkspaceCatalogBadge tone="success">
              已导入
            </WorkspaceCatalogBadge>
          ) : (
            <WorkspacePillButton
              disabled={is_busy}
              density="compact"
              onClick={on_import}
              size="sm"
              variant="primary"
            >
              {is_busy ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <Download className="h-3 w-3" />
              )}
              导入
            </WorkspacePillButton>
          )}
        </div>
      </div>

      {/* 描述 */}
      <p className="mt-2.5 line-clamp-2 flex-1 text-[12px] leading-[1.6] text-slate-600/70">
        {item.readme_markdown || item.description}
      </p>
    </WorkspaceCatalogCard>
  );
}
