import { useCallback, useEffect, useRef, useState } from "react";

import {
  listExternalSkillSourcesApi,
  updateExternalSkillSourceApi,
} from "@/lib/api/capability/skill-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ExternalSkillSourceInfo } from "@/types/capability/skill";

import type {
  ExternalSkillSourcesController,
  SkillMarketplaceFeedbackActions,
} from "./skill-marketplace-controller";

interface UseExternalSkillSourcesOptions {
  active: boolean;
  feedback: SkillMarketplaceFeedbackActions;
}

export function useExternalSkillSources({
  active,
  feedback,
}: UseExternalSkillSourcesOptions): ExternalSkillSourcesController {
  const { t } = useI18n();
  const [items, setItems] = useState<ExternalSkillSourceInfo[]>([]);
  const [managerOpen, setManagerOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [revision, setRevision] = useState(0);
  const requestRef = useRef(0);
  const shouldLoad = active || managerOpen;

  const refresh = useCallback(async (): Promise<boolean> => {
    const requestId = ++requestRef.current;
    setLoading(true);
    try {
      const nextItems = await listExternalSkillSourcesApi();
      if (requestId === requestRef.current) {
        setItems(nextItems);
      }
      return requestId === requestRef.current;
    } catch (error) {
      if (requestId === requestRef.current) {
        feedback.error(getErrorMessage(
          error,
          t("capability.skill_sources_load_failed"),
        ));
      }
      return false;
    } finally {
      if (requestId === requestRef.current) {
        setLoading(false);
      }
    }
  }, [feedback, t]);

  useEffect(() => {
    if (shouldLoad) {
      void refresh();
    }
  }, [refresh, shouldLoad]);

  const toggle = useCallback(async (
    source: ExternalSkillSourceInfo,
    enabled: boolean,
  ) => {
    feedback.clear();
    setLoading(true);
    try {
      await updateExternalSkillSourceApi(source.source_id, { enabled });
      setRevision((value) => value + 1);
      if (await refresh()) {
        feedback.success(t(
          enabled
            ? "capability.skill_source_enabled_success"
            : "capability.skill_source_disabled_success",
          { name: source.name },
        ));
      }
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skill_sources_update_failed"),
      ));
    } finally {
      setLoading(false);
    }
  }, [feedback, refresh, t]);

  return {
    closeManager: () => setManagerOpen(false),
    items,
    loading,
    managerOpen,
    openManager: () => setManagerOpen(true),
    revision,
    toggle,
  };
}
