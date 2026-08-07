import { useCallback, useEffect, useRef, useState } from "react";

import {
  createExternalSkillSourceApi,
  deleteExternalSkillSourceApi,
  listExternalSkillSourcesApi,
  updateExternalSkillSourceApi,
} from "@/lib/api/capability/skill-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { ExternalSkillSourceInfo } from "@/types/capability/skill";

import type {
  ExternalSkillSourcesController,
  PrivateSkillSourceDraft,
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

  const save = useCallback(async (
    source: ExternalSkillSourceInfo | null,
    draft: PrivateSkillSourceDraft,
  ): Promise<boolean> => {
    feedback.clear();
    setLoading(true);
    try {
      if (source) {
        await updateExternalSkillSourceApi(source.source_id, {
          auth_type: draft.authType,
          name: draft.name.trim(),
          token: draft.token.trim() || undefined,
        });
      } else {
        await createExternalSkillSourceApi({
          auth_type: draft.authType,
          name: draft.name.trim(),
          token: draft.token.trim() || undefined,
          url: draft.url.trim(),
        });
      }
      setRevision((value) => value + 1);
      await refresh();
      feedback.success(t(
        source
          ? "capability.skill_source_updated_success"
          : "capability.skill_source_created_success",
        { name: draft.name.trim() },
      ));
      return true;
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skill_sources_update_failed"),
      ));
      return false;
    } finally {
      setLoading(false);
    }
  }, [feedback, refresh, t]);

  const remove = useCallback(async (source: ExternalSkillSourceInfo) => {
    feedback.clear();
    setLoading(true);
    try {
      await deleteExternalSkillSourceApi(source.source_id);
      setRevision((value) => value + 1);
      await refresh();
      feedback.success(t("capability.skill_source_deleted_success", {
        name: source.name,
      }));
    } catch (error) {
      feedback.error(getErrorMessage(
        error,
        t("capability.skill_source_delete_failed"),
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
    remove,
    save,
    toggle,
  };
}
