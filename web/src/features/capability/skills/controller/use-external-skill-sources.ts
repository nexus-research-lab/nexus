// INPUT: 外部 Skill 来源 API、当前来源快照与用户 mutation。
// OUTPUT: 分离来源写入结果和后续列表刷新的控制状态与可恢复反馈。
// POS: Skill 来源控制器；结果未知时只刷新对账，不重复写入。
import { useCallback, useEffect, useRef, useState } from "react";

import {
  createExternalSkillSourceApi,
  deleteExternalSkillSourceApi,
  listExternalSkillSourcesApi,
  updateExternalSkillSourceApi,
} from "@/lib/api/capability/skill-api";
import { projectMutationFailure } from "@/lib/error-message";
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
  const [loadFailed, setLoadFailed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [mutationLocked, setMutationLocked] = useState(false);
  const mutationLockedRef = useRef(false);
  const recoveryRef = useRef<{ effect: "unknown" | "accepted" | "committed"; name: string } | null>(null);
  const mutationRunningRef = useRef(false);
  const [revision, setRevision] = useState(0);
  const requestRef = useRef(0);
  const shouldLoad = active || managerOpen;
  const updateMutationLock = useCallback((locked: boolean) => {
    mutationLockedRef.current = locked;
    setMutationLocked(locked);
  }, []);

  const refresh = useCallback(async (duringMutation = false): Promise<boolean> => {
    if (mutationRunningRef.current && !duringMutation) return false;
    const requestId = ++requestRef.current;
    setLoading(true);
    try {
      const nextItems = await listExternalSkillSourcesApi();
      if (requestId === requestRef.current) {
        setItems(nextItems);
        setLoadFailed(false);
        if (mutationLockedRef.current) {
          const recovery = recoveryRef.current;
          if (recovery?.effect === "committed") {
            updateMutationLock(false);
            recoveryRef.current = null;
            feedback.clear();
          } else if (recovery) {
            // A list read cannot prove an opaque credential write or a late
            // accepted mutation was not applied. Keep the write lock separate.
            const accepted = recovery.effect === "accepted";
            feedback.report({
              action: {
                label: t(accepted ? "state.retry" : "capability.skill_operation_new_intent_action"),
                onClick: accepted ? () => { void refresh(); } : () => {
                  recoveryRef.current = null;
                  updateMutationLock(false);
                  feedback.clear();
                },
              },
              title: t(accepted ? "capability.skill_source_accepted_title" : "capability.skill_source_unknown_title"),
              impact: t(accepted ? "capability.skill_source_accepted_impact" : "capability.skill_source_unknown_impact", { name: recovery.name }),
              nextStep: t(accepted ? "capability.skill_source_accepted_next_step" : "capability.skill_operation_unknown_next_step"),
              pending: false,
              persistent: true,
              tone: "warning",
            });
          }
        }
      }
      return requestId === requestRef.current;
    } catch {
      if (requestId === requestRef.current) {
        setLoadFailed(true);
        feedback.report({
          action: {
            label: t("state.retry"),
            onClick: () => {
              void refresh();
            },
          },
          impact: t("state.read_failure_impact"),
          message: t("capability.skill_sources_load_failed"),
          nextStep: t("state.retry_next_step"),
          pending: false,
          title: t("capability.skill_sources_load_failed_title"),
          tone: "error",
        });
      }
      return false;
    } finally {
      if (requestId === requestRef.current) {
        setLoading(false);
      }
    }
  }, [feedback, t, updateMutationLock]);

  useEffect(() => {
    if (shouldLoad) {
      void refresh();
    }
  }, [refresh, shouldLoad]);

  const toggle = useCallback(async (
    source: ExternalSkillSourceInfo,
    enabled: boolean,
  ) => {
    if (mutationLockedRef.current || mutationRunningRef.current) return;
    mutationRunningRef.current = true;
    feedback.clear();
    setLoading(true);
    try {
      await updateExternalSkillSourceApi(source.source_id, { enabled });
      recoveryRef.current = { effect: "committed", name: source.name };
      setRevision((value) => value + 1);
      if (await refresh(true)) {
        feedback.success(t(
          enabled
            ? "capability.skill_source_enabled_success"
            : "capability.skill_source_disabled_success",
          { name: source.name },
        ));
      } else {
        updateMutationLock(true);
        reportCommittedRefreshFailure(feedback, refresh, source.name, t);
      }
    } catch (error) {
      const requiresReconciliation = reportSourceMutationFailure(
        feedback,
        refresh,
        source.name,
        error,
        t,
      );
      recoveryRef.current = requiresReconciliation === "not_applied" ? null : { effect: requiresReconciliation, name: source.name };
      updateMutationLock(requiresReconciliation !== "not_applied");
    } finally {
      mutationRunningRef.current = false;
      setLoading(false);
    }
  }, [feedback, refresh, t, updateMutationLock]);

  const save = useCallback(async (
    source: ExternalSkillSourceInfo | null,
    draft: PrivateSkillSourceDraft,
  ): Promise<boolean> => {
    if (mutationLockedRef.current || mutationRunningRef.current) return false;
    mutationRunningRef.current = true;
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
      recoveryRef.current = { effect: "committed", name: draft.name.trim() };
      setRevision((value) => value + 1);
      if (await refresh(true)) {
        feedback.success(t(
          source
            ? "capability.skill_source_updated_success"
            : "capability.skill_source_created_success",
          { name: draft.name.trim() },
        ));
      } else {
        updateMutationLock(true);
        reportCommittedRefreshFailure(feedback, refresh, draft.name.trim(), t);
      }
      return true;
    } catch (error) {
      const requiresReconciliation = reportSourceMutationFailure(
        feedback,
        refresh,
        draft.name.trim(),
        error,
        t,
      );
      recoveryRef.current = requiresReconciliation === "not_applied" ? null : { effect: requiresReconciliation, name: draft.name.trim() };
      updateMutationLock(requiresReconciliation !== "not_applied");
      return false;
    } finally {
      mutationRunningRef.current = false;
      setLoading(false);
    }
  }, [feedback, refresh, t, updateMutationLock]);

  const remove = useCallback(async (source: ExternalSkillSourceInfo) => {
    if (mutationLockedRef.current || mutationRunningRef.current) return;
    mutationRunningRef.current = true;
    feedback.clear();
    setLoading(true);
    try {
      await deleteExternalSkillSourceApi(source.source_id);
      recoveryRef.current = { effect: "committed", name: source.name };
      setRevision((value) => value + 1);
      if (await refresh(true)) {
        feedback.success(t("capability.skill_source_deleted_success", {
          name: source.name,
        }));
      } else {
        updateMutationLock(true);
        reportCommittedRefreshFailure(feedback, refresh, source.name, t);
      }
    } catch (error) {
      const requiresReconciliation = reportSourceMutationFailure(
        feedback,
        refresh,
        source.name,
        error,
        t,
      );
      recoveryRef.current = requiresReconciliation === "not_applied" ? null : { effect: requiresReconciliation, name: source.name };
      updateMutationLock(requiresReconciliation !== "not_applied");
    } finally {
      mutationRunningRef.current = false;
      setLoading(false);
    }
  }, [feedback, refresh, t, updateMutationLock]);

  return {
    closeManager: () => setManagerOpen(false),
    items,
    loadFailed,
    retry: () => { void refresh(); },
    loading,
    mutationBlocked: mutationLocked,
    managerOpen,
    openManager: () => setManagerOpen(true),
    revision,
    remove,
    save,
    toggle,
  };
}

function reportSourceMutationFailure(
  feedback: SkillMarketplaceFeedbackActions,
  refresh: () => Promise<boolean>,
  sourceName: string,
  error: unknown,
  t: ReturnType<typeof useI18n>["t"],
): "accepted" | "committed" | "not_applied" | "unknown" {
  const failure = projectMutationFailure(
    error,
    t("capability.skill_sources_update_failed"),
  );
  const outcome = failure.effect === "accepted"
    || failure.effect === "committed"
    || failure.effect === "not_applied"
    ? failure.effect
    : "unknown";
  const notApplied = outcome === "not_applied";
  const copy = {
    accepted: {
      impact: "capability.skill_source_accepted_impact",
      nextStep: "capability.skill_source_accepted_next_step",
      title: "capability.skill_source_accepted_title",
    },
    committed: {
      impact: "capability.skill_source_committed_impact",
      nextStep: "capability.skill_source_committed_next_step",
      title: "capability.skill_source_committed_title",
    },
    not_applied: {
      impact: "capability.skill_source_not_applied_impact",
      nextStep: "capability.skill_source_not_applied_next_step",
      title: "capability.skill_source_not_applied_title",
    },
    unknown: {
      impact: "capability.skill_source_unknown_impact",
      nextStep: "capability.skill_source_unknown_next_step",
      title: "capability.skill_source_unknown_title",
    },
  } as const;
  const selectedCopy = copy[outcome];
  feedback.report({
    action: notApplied
      ? undefined
      : {
          label: t("state.retry"),
          onClick: () => {
            void refresh();
          },
    },
    impact: t(selectedCopy.impact, { name: sourceName }),
    nextStep: t(selectedCopy.nextStep),
    pending: false,
    persistent: !notApplied,
    title: t(selectedCopy.title),
    tone: notApplied ? "error" : "warning",
  });
  return outcome;
}

function reportCommittedRefreshFailure(
  feedback: SkillMarketplaceFeedbackActions,
  refresh: () => Promise<boolean>,
  sourceName: string,
  t: ReturnType<typeof useI18n>["t"],
) {
  feedback.report({
    action: {
      label: t("state.retry"),
      onClick: () => {
        void refresh();
      },
    },
    impact: t("capability.skill_source_refresh_failed_impact", {
      name: sourceName,
    }),
    message: t("capability.skill_source_refresh_failed_message"),
    nextStep: t("capability.skill_source_refresh_failed_next_step"),
    pending: false,
    persistent: true,
    title: t("capability.skill_source_refresh_failed_title"),
    tone: "warning",
  });
}
