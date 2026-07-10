import { useCallback, useEffect, useRef, useState } from "react";

import {
  listExternalSkillSourcesApi,
  updateExternalSkillSourceApi,
} from "@/lib/api/skill-api";
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
        feedback.error(error instanceof Error ? error.message : "技能来源加载失败");
      }
      return false;
    } finally {
      if (requestId === requestRef.current) {
        setLoading(false);
      }
    }
  }, [feedback]);

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
        feedback.success(`${source.name} 已${enabled ? "启用" : "停用"}`);
      }
    } catch (error) {
      feedback.error(error instanceof Error ? error.message : "技能来源更新失败");
    } finally {
      setLoading(false);
    }
  }, [feedback, refresh]);

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
