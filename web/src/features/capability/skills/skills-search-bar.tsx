import { cn } from "@/lib/utils";

import { WorkspaceSearchInput } from "@/shared/ui/workspace/controls/workspace-search-input";

import type { SkillMarketplaceController } from "@/hooks/capability/use-skill-marketplace";

interface SkillsSearchBarProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsSearchBar({ ctrl }: SkillsSearchBarProps) {
  return (
    <div className="mb-5 flex flex-wrap items-center gap-x-5 gap-y-3">
      <div className="w-full max-w-[34rem] shrink-0">
        <WorkspaceSearchInput
          class_name="h-11 w-full px-3.5 py-2"
          input_class_name="text-[15px]"
          on_change={(value) => {
            if (ctrl.discovery_mode === "catalog") {
              ctrl.set_search_query(value);
              return;
            }
            ctrl.set_external_query(value);
          }}
          placeholder={
            ctrl.discovery_mode === "catalog"
              ? "搜索技能名称、标签或场景..."
              : "搜索社区共享技能..."
          }
          value={ctrl.discovery_mode === "catalog" ? ctrl.search_query : ctrl.external_query}
        />
      </div>

      {ctrl.discovery_mode === "catalog" ? (
        <div className="soft-scrollbar scrollbar-hide flex min-w-0 flex-1 items-center gap-5 overflow-x-auto">
          {ctrl.categories.map((category) => {
            const is_active = ctrl.active_category === category.key;
            return (
              <button
                key={category.key}
                className={cn(
                  "inline-flex h-9 shrink-0 items-center border-b-2 border-transparent px-0 py-1 text-[12px] font-semibold transition-[color,border-color] duration-(--motion-duration-fast)",
                  is_active
                    ? "border-(--surface-interactive-active-border) text-(--text-strong)"
                    : "text-(--text-default) hover:text-(--text-strong)",
                )}
                onClick={() => ctrl.set_active_category(category.key)}
                type="button"
              >
                {category.label}
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
