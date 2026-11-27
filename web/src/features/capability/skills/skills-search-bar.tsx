// INPUT: Skill 目录/社区的搜索、来源及用户命令。
// OUTPUT: 统一目录标签、回车搜索与社区来源筛选；已安装目录按分类分区展示，不再提供分类下拉。
// POS: Skill 工具区纯视图；筛选结构由 UiFilterSelect 唯一拥有。

import { UiFilterSelect } from "@/shared/ui/menu/filter-select";
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { useRef, type KeyboardEvent } from "react";

import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiDirectoryTabs } from "@/shared/ui/navigation/directory-tabs";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
} from "@/features/capability/shared/capability-page-layout";
import type { DiscoveryMode } from "./controller/skill-marketplace-controller";

const DISCOVERY_OPTIONS: ReadonlyArray<{
  labelKey: TranslationKey;
  value: DiscoveryMode;
}> = [
  { labelKey: "capability.skills_tab_catalog", value: "catalog" },
  { labelKey: "capability.skills_tab_external", value: "external" },
];

interface SkillsSearchBarProps {
  catalogQuery: string;
  discoveryMode: DiscoveryMode;
  externalLoading: boolean;
  externalQuery: string;
  externalSourceId: string;
  externalSources: Array<{ disabled?: boolean; label: string; value: string }>;
  onChangeCatalogQuery: (query: string) => void;
  onChangeDiscoveryMode: (mode: DiscoveryMode) => void;
  onChangeExternalQuery: (query: string) => void;
  onChangeExternalSource: (sourceId: string) => void;
  onSubmitExternalSearch: () => void;
}

export function SkillsSearchBar({
  catalogQuery,
  discoveryMode,
  externalLoading,
  externalQuery,
  externalSourceId,
  externalSources,
  onChangeCatalogQuery,
  onChangeDiscoveryMode,
  onChangeExternalQuery,
  onChangeExternalSource,
  onSubmitExternalSearch,
}: SkillsSearchBarProps) {
  const { t } = useI18n();
  const composingRef = useRef(false);

  const searchDisabled = externalQuery.trim().length < 2 || externalLoading;

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (discoveryMode !== "external") return;
    if (event.key !== "Enter") return;
    if (event.defaultPrevented || composingRef.current || isImeKeyboardEvent(event.nativeEvent)) return;
    event.preventDefault();
    if (!searchDisabled) onSubmitExternalSearch();
  };

  return (
    <CapabilityFilterBar className="sm:justify-between">
      <UiDirectoryTabs
        activeValue={discoveryMode}
        ariaLabel={t("capability.skills_tour_modes_title")}
        navAnchor={SKILLS_TOUR_ANCHORS.modes}
        onChange={onChangeDiscoveryMode}
        options={DISCOVERY_OPTIONS.map((option) => ({
          label: t(option.labelKey),
          value: option.value,
        }))}
      />

      <div className="flex min-w-0 flex-1 flex-col gap-2 sm:ml-auto sm:max-w-[520px] sm:flex-row sm:items-center">
        <CapabilityFilterSearchInput
          onChange={(value) => {
            if (discoveryMode === "catalog") {
              onChangeCatalogQuery(value);
              return;
            }
            onChangeExternalQuery(value);
          }}
          onCompositionEnd={() => {
            composingRef.current = false;
          }}
          onCompositionStart={() => {
            composingRef.current = true;
          }}
          onKeyDown={handleKeyDown}
          placeholder={
            discoveryMode === "catalog"
              ? t("capability.skills_search_catalog")
              : t("capability.skills_search_external")
          }
          value={discoveryMode === "catalog" ? catalogQuery : externalQuery}
        />

        {discoveryMode === "external" ? (
          <UiFilterSelect
            ariaLabel={t("capability.skill_source_search_scope")}
            onChange={onChangeExternalSource}
            options={externalSources}
            value={externalSourceId}
          />
        ) : null}
      </div>
    </CapabilityFilterBar>
  );
}
