import { Loader2, Puzzle } from "lucide-react";

import type { SkillInfo } from "@/types/skill";

import type { SkillMarketplaceController } from "@/hooks/use-skill-marketplace";

import { SkillsCard } from "./skills-card";

interface SkillsCatalogGridProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsCatalogGrid({ ctrl }: SkillsCatalogGridProps) {
  if (ctrl.loading) {
    return (
      <div className="flex min-h-80 items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
      </div>
    );
  }

  if (!ctrl.grouped_skills.length) {
    return (
      <div className="flex min-h-80 flex-col items-center justify-center gap-3 text-center">
        <div className="flex h-14 w-14 items-center justify-center rounded-full border border-white/40 bg-white/60">
          <Puzzle className="h-6 w-6 text-slate-400" />
        </div>
        <div>
          <p className="text-[16px] font-bold text-slate-800/80">
            {ctrl.installed_only ? "没有已安装的技能" : "没有符合条件的技能"}
          </p>
          <p className="mt-1 text-[13px] text-slate-500/60">
            {ctrl.installed_only
              ? "试试关闭「已安装」筛选，或从社区技能导入"
              : "试试切换分类、来源或搜索条件"}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-7">
      {ctrl.grouped_skills.map(([category_name, items]: [string, SkillInfo[]]) => (
        <section key={category_name}>
          <div className="mb-3 flex items-center gap-2.5">
            <h2 className="text-[15px] font-bold tracking-[-0.02em] text-slate-950/85">
              {category_name}
            </h2>
            <span className="rounded-full bg-white/60 px-2 py-0.5 text-[10px] font-semibold text-slate-500">
              {items.length}
            </span>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
            {items.map((skill: SkillInfo) => (
              <div
                key={skill.name}
                className={ctrl.busy_skill_name === skill.name ? "opacity-60 transition-opacity" : "transition-opacity"}
              >
                <SkillsCard
                  busy={ctrl.busy_skill_name === skill.name}
                  on_delete={() => void ctrl.handle_delete_from_pool(skill)}
                  on_install={() => void ctrl.handle_install_to_pool(skill)}
                  on_select={() => ctrl.set_selected_skill(skill.name)}
                  on_toggle_global_enabled={() => void ctrl.handle_toggle_global_enabled(skill)}
                  on_update={() => void ctrl.handle_update_single(skill.name)}
                  skill={skill}
                />
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
