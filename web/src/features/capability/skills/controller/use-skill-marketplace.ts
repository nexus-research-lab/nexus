import { useState } from "react";

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
  const [discoveryMode, setDiscoveryMode] = useState<DiscoveryMode>("catalog");
  const { actions: feedbackActions, feedback } = useSkillMarketplaceFeedback();
  const catalog = useSkillCatalog({
    active: discoveryMode === "catalog",
    onError: feedbackActions.error,
  });
  const sources = useExternalSkillSources({
    active: discoveryMode === "external",
    feedback: feedbackActions,
  });
  const external = useExternalSkillSearch({
    active: discoveryMode === "external",
    onError: feedbackActions.error,
    sourceRevision: sources.revision,
  });
  const operations = useSkillOperations({
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
