import { Download, Import, Loader2 } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
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
      <div className="rounded-[18px] border border-dashed border-white/40 bg-white/40 px-5 py-8 text-center text-sm text-slate-500">
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

  return (
    <div className="workspace-card rounded-[20px] px-5 py-4 transition-all hover:bg-white/40">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate text-[14px] font-bold text-slate-950/90">
              {item.title || item.skill_slug}
            </p>
            <span className="rounded-full bg-sky-50 px-2 py-0.5 text-[10px] font-semibold text-sky-600">
              {formatInstalls(item.installs)} installs
            </span>
            {already_imported && (
              <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-600">
                已导入
              </span>
            )}
          </div>
          <p className="mt-0.5 truncate text-[11px] text-slate-500">
            {item.package_spec}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <WorkspacePillButton onClick={on_preview} size="sm">
            预览
          </WorkspacePillButton>
          <WorkspacePillButton
            disabled={busy_skill_name === item.skill_slug}
            onClick={on_import}
            size="sm"
            variant="strong"
          >
            {busy_skill_name === item.skill_slug ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Download className="h-3.5 w-3.5" />
            )}
            导入
          </WorkspacePillButton>
        </div>
      </div>
      <p className="mt-2 line-clamp-2 text-[12px] leading-5 text-slate-600/70">
        {item.readme_markdown || item.description}
      </p>
      <div className="mt-2.5 flex items-center gap-1.5 text-[10px] text-slate-400">
        <Import className="h-3 w-3" />
        导入后由 Nexus 管理安装与更新
      </div>
    </div>
  );
}
