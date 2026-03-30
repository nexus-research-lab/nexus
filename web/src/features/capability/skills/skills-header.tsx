import { ChevronDown, Download, Filter, FolderUp, Puzzle, RefreshCw } from "lucide-react";

import { cn } from "@/lib/utils";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";

import type { SourceFilter, SkillMarketplaceController } from "@/hooks/use-skill-marketplace";
import { SOURCE_LABELS } from "@/hooks/use-skill-marketplace";

interface SkillsHeaderProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsHeader({ ctrl }: SkillsHeaderProps) {
  return (
    <div className="z-10 border-b workspace-divider bg-white/60">
      {/* 第一行：标题 + 操作按钮 */}
      <div className="flex items-center justify-between gap-4 px-5 py-3 xl:px-6">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-white/40 bg-white/60 shadow-sm">
            <Puzzle className="h-4 w-4 text-slate-700/72" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="text-[17px] font-black tracking-[-0.04em] text-slate-950/88">
                Skills
              </span>
              <span className="rounded-full border border-white/40 bg-white/56 px-2 py-0.5 text-[10px] font-semibold text-slate-600">
                已安装 {ctrl.installed_count}
              </span>
            </div>
            <p className="text-[12px] text-slate-700/52">浏览和维护全局技能资源池</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <WorkspacePillButton onClick={() => ctrl.file_input_ref.current?.click()} size="sm">
            <FolderUp className="h-3.5 w-3.5" />
            导入本地
          </WorkspacePillButton>
          <WorkspacePillButton onClick={() => ctrl.set_git_prompt_open(true)} size="sm">
            <Download className="h-3.5 w-3.5" />
            Git 安装
          </WorkspacePillButton>
          <WorkspacePillButton onClick={() => void ctrl.handle_update_installed()} size="sm">
            <RefreshCw className="h-3.5 w-3.5" />
            更新技能库
          </WorkspacePillButton>
        </div>
      </div>

      {/* 第二行：分类标签 + 筛选 */}
      <div className="flex items-center justify-between gap-4 px-5 pb-2.5 xl:px-6">
        <div className="flex items-center gap-1 overflow-x-auto">
          {ctrl.categories.map((cat) => (
            <button
              key={cat.key}
              className={cn(
                "shrink-0 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all",
                ctrl.active_category === cat.key
                  ? "workspace-chip text-slate-950 shadow-[0_10px_18px_rgba(111,126,162,0.08)]"
                  : "text-slate-600 hover:bg-slate-100 hover:text-slate-950",
              )}
              onClick={() => ctrl.set_active_category(cat.key)}
              type="button"
            >
              {cat.label}
            </button>
          ))}
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {/* 已安装筛选 */}
          <button
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all",
              ctrl.installed_only
                ? "bg-emerald-50 text-emerald-700 border border-emerald-200/60"
                : "text-slate-600 hover:bg-slate-100",
            )}
            onClick={() => ctrl.set_installed_only(!ctrl.installed_only)}
            type="button"
          >
            <Filter className="h-3 w-3" />
            已安装
            {ctrl.installed_only && (
              <span className="rounded-full bg-emerald-100 px-1.5 text-[10px]">
                {ctrl.installed_count}
              </span>
            )}
          </button>

          {/* 来源选择 */}
          <div className="relative">
            <button
              className="inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[12px] font-semibold text-slate-600 transition-all hover:bg-slate-100"
              onClick={() => ctrl.set_source_dropdown_open(!ctrl.source_dropdown_open)}
              type="button"
            >
              {SOURCE_LABELS[ctrl.source_filter]}
              <ChevronDown className="h-3 w-3" />
            </button>
            {ctrl.source_dropdown_open && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => ctrl.set_source_dropdown_open(false)} />
                <div className="absolute right-0 top-full z-50 mt-1 w-32 overflow-hidden rounded-[14px] border border-white/50 bg-white/95 py-1 shadow-lg backdrop-blur-sm">
                  {(Object.keys(SOURCE_LABELS) as SourceFilter[]).map((key) => (
                    <button
                      key={key}
                      className={cn(
                        "flex w-full items-center px-3 py-2 text-[12px] font-medium transition-colors",
                        ctrl.source_filter === key
                          ? "bg-slate-100 text-slate-950"
                          : "text-slate-600 hover:bg-slate-50",
                      )}
                      onClick={() => {
                        ctrl.set_source_filter(key);
                        ctrl.set_source_dropdown_open(false);
                      }}
                      type="button"
                    >
                      {SOURCE_LABELS[key]}
                    </button>
                  ))}
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
