import { useCallback, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import type {
  DiscoveryMode,
  SkillMarketplaceController,
} from "./skill-marketplace-controller";
import { useExternalSkillSearch } from "./use-external-skill-search";
import { useExternalSkillSources } from "./use-external-skill-sources";
import { useSkillCatalog } from "./use-skill-catalog";
import { useSkillMarketplaceFeedback } from "./use-skill-marketplace-feedback";
import { useSkillOperations } from "./use-skill-operations";

export function useSkillMarketplace(): SkillMarketplaceController {
  const { t } = useI18n();
  const [discoveryMode, setDiscoveryMode] = useState<DiscoveryMode>("catalog");
  const { actions: feedbackActions, feedback } = useSkillMarketplaceFeedback();
  const reportCatalogReadFailure = useCallback((message: string) => {
    feedbackActions.report({
      impact: t("capability.skills_catalog_load_failed_impact"),
      message,
      nextStep: t("capability.skills_catalog_load_failed_next_step"),
      pending: false,
      title: t("capability.skills_catalog_load_failed_title"),
      tone: "error",
    });
  }, [feedbackActions, t]);
  const reportExternalReadFailure = useCallback((
    kind: "preview" | "search",
    message: string,
  ) => {
    feedbackActions.report({
      impact: t(kind === "search"
        ? "capability.skills_external_search_failed_impact"
        : "capability.skills_external_preview_failed_impact"),
      message,
      nextStep: t(kind === "search"
        ? "capability.skills_external_search_failed_next_step"
        : "capability.skills_external_preview_failed_next_step"),
      pending: false,
      title: t(kind === "search"
        ? "capability.skills_external_search_failed_title"
        : "capability.skills_external_preview_failed_title"),
      tone: "error",
    });
  }, [feedbackActions, t]);
  const catalog = useSkillCatalog({
    active: discoveryMode === "catalog",
    onError: reportCatalogReadFailure,
  });
  const sources = useExternalSkillSources({
    active: discoveryMode === "external",
    feedback: feedbackActions,
  });
  const external = useExternalSkillSearch({
    active: discoveryMode === "external",
    onError: reportExternalReadFailure,
    sourceRevision: sources.revision,
    sources: sources.items,
    sourcesLoading: sources.loading,
  });
  const operations = useSkillOperations({
    catalogSkills: catalog.skills,
    closeExternalPreview: external.closePreview,
    feedback: feedbackActions,
    refreshCatalog: catalog.refresh,
    updateAvailableCount: catalog.updateAvailableSkills.length,
  });

  return {
    catalog,
    discoveryMode,
    external,
    feedback,
    operations,
    setDiscoveryMode,
    sources,
  };
}
