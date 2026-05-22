import { UiUnderlineTabs } from "@/shared/ui/tabs";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/controls/workspace-search-input";
import type { SkillMarketplaceController } from "./skills-view-model";
import { SKILLS_TOUR_ANCHORS } from "./skills-tour";

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
        <UiUnderlineTabs
          active_value={ctrl.active_category}
          aria_label="技能分类"
          class_name="flex-1 gap-5"
          item_class_name="text-[12px]"
          nav_anchor={SKILLS_TOUR_ANCHORS.categories}
          on_change={ctrl.set_active_category}
          options={ctrl.categories.map((category) => ({
            label: category.label,
            value: category.key,
          }))}
        />
      ) : null}
    </div>
  );
}
