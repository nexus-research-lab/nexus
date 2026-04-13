import { Globe2 } from "lucide-react";
import { cn } from "@/lib/utils";

import { WorkspaceSearchInput } from "@/shared/ui/workspace/workspace-search-input";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/workspace-surface-header";

import type { SkillMarketplaceController } from "@/hooks/use-skill-marketplace";

interface SkillsSearchBarProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsSearchBar({ ctrl }: SkillsSearchBarProps) {
  return (
    <div className="mb-5 space-y-3">
      <div className="flex items-center gap-2.5">
        <WorkspaceSearchInput
          class_name="flex-1"
          on_change={(value) => {
            ctrl.set_search_query(value);
            if (ctrl.discovery_mode === "external") ctrl.set_external_query(value);
          }}
          placeholder={
            ctrl.discovery_mode === "catalog"
              ? "搜索技能名称、标签或场景..."
              : "搜索社区共享技能..."
          }
          value={ctrl.discovery_mode === "catalog" ? ctrl.search_query : ctrl.external_query}
        />
        <WorkspaceSurfaceToolbarAction
          onClick={() =>
            void (
              ctrl.discovery_mode === "external"
                ? ctrl.handle_external_search()
                : ctrl.handle_catalog_search()
            )}
          tone="primary"
        >
          <Globe2 className="h-3.5 w-3.5" />
          搜索
        </WorkspaceSurfaceToolbarAction>
      </div>

      {ctrl.discovery_mode === "catalog" ? (
        <div className="soft-scrollbar scrollbar-hide -mx-0.5 flex min-w-0 flex-1 items-center gap-4 overflow-x-auto px-0.5">
          {ctrl.categories.map((category) => {
            const is_active = ctrl.active_category === category.key;
            return (
              <button
                key={category.key}
                className={cn(
                  "inline-flex h-8 shrink-0 items-center border-b-2 border-transparent px-0 py-1 text-[11px] font-semibold transition-[color,border-color] duration-[var(--motion-duration-fast)]",
                  is_active
                    ? "border-[var(--surface-interactive-active-border)] text-[color:var(--text-strong)]"
                    : "text-[color:var(--text-default)] hover:text-[color:var(--text-strong)]",
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
