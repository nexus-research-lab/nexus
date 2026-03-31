import { ChevronDown, Filter, Globe2, Search } from "lucide-react";

import { cn } from "@/lib/utils";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";

import type { SourceFilter, SkillMarketplaceController } from "@/hooks/use-skill-marketplace";
import { SOURCE_LABELS } from "@/hooks/use-skill-marketplace";

interface SkillsSearchBarProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsSearchBar({ ctrl }: SkillsSearchBarProps) {
  return (
    <div className="mb-5 space-y-3">
      {/* 搜索栏 + 模式切换 */}
      <div className="flex items-stretch gap-2">
        {/* 搜索框 */}
        <label className="home-glass-input inline-flex flex-1 items-center gap-2 rounded-full px-4 py-3 text-sm text-slate-700/62">
          <Search className="h-4 w-4 shrink-0" />
          <input
            className="min-w-0 flex-1 bg-transparent text-sm text-slate-950/86 outline-none placeholder:text-slate-500"
            onChange={(e) => {
              ctrl.set_search_query(e.target.value);
              if (ctrl.discovery_mode === "external") ctrl.set_external_query(e.target.value);
            }}
            placeholder={
              ctrl.discovery_mode === "catalog"
                ? "搜索技能名称、标签或场景..."
                : "搜索社区共享技能..."
            }
            value={ctrl.discovery_mode === "catalog" ? ctrl.search_query : ctrl.external_query}
          />
        </label>

        {/* 模式切换胶囊 */}
        <div className="flex items-center gap-1">
          <button
            className={cn(
              "rounded-full px-3.5 py-2 text-[12px] font-semibold transition-all",
              ctrl.discovery_mode === "catalog"
                ? "workspace-chip text-slate-950"
                : "text-slate-500 hover:bg-white/50",
            )}
            onClick={() => ctrl.set_discovery_mode("catalog")}
            type="button"
          >
            库内技能
          </button>
          <button
            className={cn(
              "rounded-full px-3.5 py-2 text-[12px] font-semibold transition-all",
              ctrl.discovery_mode === "external"
                ? "workspace-chip text-slate-950"
                : "text-slate-500 hover:bg-white/50",
            )}
            onClick={() => ctrl.set_discovery_mode("external")}
            type="button"
          >
            社区技能
          </button>
          {ctrl.discovery_mode === "external" && (
            <WorkspacePillButton
              onClick={() => void ctrl.handle_external_search()}
              size="sm"
            >
              <Globe2 className="h-3.5 w-3.5" />
              搜索
            </WorkspacePillButton>
          )}
        </div>
      </div>

      {/* 分类标签 + 过滤器 — 只在库内模式显示 */}
      {/* 分类标签 + 过滤器 */}
      <div className="flex items-center justify-between gap-2">
        <div className="soft-scrollbar flex min-w-0 items-center gap-1 overflow-x-auto">
          {ctrl.categories.map((cat) => (
            <button
              key={cat.key}
              className={cn(
                "shrink-0 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all",
                ctrl.active_category === cat.key
                  ? "workspace-chip text-slate-950 shadow-[0_10px_18px_rgba(111,126,162,0.08)]"
                  : "text-slate-500 hover:bg-white/50 hover:text-slate-900",
              )}
              onClick={() => ctrl.set_active_category(cat.key)}
              type="button"
            >
              {cat.label}
            </button>
          ))}
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <button
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all",
              ctrl.installed_only
                ? "border border-emerald-200/60 bg-emerald-50 text-emerald-700"
                : "text-slate-600 hover:bg-white/50",
            )}
            onClick={() => ctrl.set_installed_only(!ctrl.installed_only)}
            type="button"
          >
            <Filter className="h-3 w-3" />
            已安装
          </button>

          <div className="relative">
            <button
              className="inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-[12px] font-semibold text-slate-600 transition-all hover:bg-white/50"
              onClick={() => ctrl.set_source_dropdown_open(!ctrl.source_dropdown_open)}
              type="button"
            >
              {SOURCE_LABELS[ctrl.source_filter]}
              <ChevronDown className="h-3 w-3" />
            </button>
            {ctrl.source_dropdown_open && (
              <>
                <div
                  className="fixed inset-0 z-40"
                  onClick={() => ctrl.set_source_dropdown_open(false)}
                />
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

