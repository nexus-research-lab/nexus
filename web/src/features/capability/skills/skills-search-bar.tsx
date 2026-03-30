import { Globe2, Search } from "lucide-react";

import { cn } from "@/lib/utils";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";

import type { SkillMarketplaceController } from "@/hooks/use-skill-marketplace";

interface SkillsSearchBarProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsSearchBar({ ctrl }: SkillsSearchBarProps) {
  return (
    <div className="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center">
      {/* 搜索框 */}
      <label className="home-glass-input inline-flex flex-1 items-center gap-2 rounded-full px-4 py-2.5 text-sm text-slate-700/62">
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

      {/* 模式切换 */}
      <div className="flex items-center gap-1.5">
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
  );
}
