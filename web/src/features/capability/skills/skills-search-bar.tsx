import { ChevronDown, Globe2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { SegmentedPill } from "@/shared/ui/segmented-pill";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { WorkspaceSearchInput } from "@/shared/ui/workspace/workspace-search-input";

import type { SourceFilter, SkillMarketplaceController } from "@/hooks/use-skill-marketplace";
import { SOURCE_LABELS } from "@/hooks/use-skill-marketplace";

const CONTEXT_MENU_CLASS_NAME =
  "absolute right-0 top-full z-50 mt-2 w-36 overflow-hidden rounded-[16px] border border-[var(--surface-popover-border)] bg-[var(--surface-popover-background)] py-1.5 shadow-[var(--surface-popover-shadow)] backdrop-blur-[18px]";
const CONTEXT_MENU_ITEM_CLASS_NAME =
  "flex w-full items-center rounded-[10px] px-3 py-2 text-[12px] font-medium transition-[background,color] duration-150";

interface SkillsSearchBarProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsSearchBar({ ctrl }: SkillsSearchBarProps) {
  return (
    <div className="mb-5 space-y-3">
      {/* 搜索栏 + 模式切换 */}
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

        {/* 模式切换胶囊 */}
        <div className="flex shrink-0 items-center gap-2">
          <SegmentedPill
            class_name="shrink-0"
            on_change={ctrl.set_discovery_mode}
            options={[
              { label: "库内技能", value: "catalog" },
              { label: "社区技能", value: "external" },
            ]}
            title="技能来源模式"
            value={ctrl.discovery_mode}
          />
          {ctrl.discovery_mode === "external" && (
            <WorkspacePillButton
              onClick={() => void ctrl.handle_external_search()}
              density="compact"
              size="sm"
              variant="outlined"
            >
              <Globe2 className="h-3.5 w-3.5" />
              搜索
            </WorkspacePillButton>
          )}
        </div>
      </div>

      {/* 分类标签 + 过滤器 */}
      <div className="flex items-center justify-between gap-2">
        <div className="soft-scrollbar flex min-w-0 items-center gap-1 overflow-x-auto">
          {ctrl.categories.map((cat) => (
            <WorkspacePillButton
              key={cat.key}
              density="compact"
              onClick={() => ctrl.set_active_category(cat.key)}
              size="sm"
              variant={ctrl.active_category === cat.key ? "tonal" : "text"}
            >
              {cat.label}
            </WorkspacePillButton>
          ))}
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <div className="relative">
            <WorkspacePillButton
              density="compact"
              onClick={() => ctrl.set_source_dropdown_open(!ctrl.source_dropdown_open)}
              size="sm"
              variant="outlined"
            >
              {SOURCE_LABELS[ctrl.source_filter]}
              <ChevronDown className="h-3 w-3" />
            </WorkspacePillButton>
            {ctrl.source_dropdown_open && (
              <>
                <div
                  className="fixed inset-0 z-40"
                  onClick={() => ctrl.set_source_dropdown_open(false)}
                />
                <div className={CONTEXT_MENU_CLASS_NAME}>
                  {(Object.keys(SOURCE_LABELS) as SourceFilter[]).map((key) => (
                    <button
                      key={key}
                      className={cn(
                        CONTEXT_MENU_ITEM_CLASS_NAME,
                        ctrl.source_filter === key
                          ? "bg-[var(--surface-interactive-active-background)] text-slate-950"
                          : "text-slate-600 hover:bg-[var(--surface-interactive-hover-background)]",
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
