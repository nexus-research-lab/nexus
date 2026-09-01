/**
 * INPUT: owner-scoped Preferences GET/PATCH、服务端 version 与本页偏好草稿。
 * OUTPUT: 完整权威快照、首次读取门禁、If-Match CAS、未知结果对账和可显式重应用的偏好状态。
 * POS: General/Runtime 共用的 Preferences 交互事务边界；草稿未确认前不发布为全局 runtime 默认值。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
} from "@/shared/auth/auth-owner-generation";
import { setUserPreferences } from "@/config/runtime-options";
import {
  getUserPreferencesApi,
  updateUserPreferencesApi,
} from "@/lib/api/settings/preferences-api";
import { projectMutationFailure } from "@/lib/error-message";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { UserPreferences } from "@/types/settings/preferences";

import {
  type PreferenceFeedback,
  type PreferenceRecoveryControls,
  buildPreferencesUpdatePayload,
  equivalentPreferences,
  normalizePreferences,
  rebasePreferenceDraft,
} from "./model/settings-preferences-model";

type PreferenceMutation = (current: UserPreferences) => UserPreferences;

interface PendingPreferenceDraft {
  base: UserPreferences;
  draft: UserPreferences;
  latest: UserPreferences | null;
  projectionRepairRequired: boolean;
}

export function useUserPreferences() {
  const { t } = useI18n();
  const { status: authStatus } = useAuth();
  const authOwnerReloadKey = preferencesAuthOwnerReloadKey(authStatus);
  const [preferences, setPreferences] = useState<UserPreferences>(() =>
    normalizePreferences(null),
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [checking, setChecking] = useState(false);
  const [repairing, setRepairing] = useState(false);
  const [writable, setWritable] = useState(false);
  const [comparisonReady, setComparisonReady] = useState(false);
  const [projectionRepairReady, setProjectionRepairReady] = useState(false);
  const [feedback, setFeedback] = useState<PreferenceFeedback | null>(null);
  const preferencesRef = useRef(preferences);
  const translateRef = useRef(t);
  const lastSavedRef = useRef<UserPreferences | null>(null);
  const pendingRef = useRef<PendingPreferenceDraft | null>(null);
  const savingRef = useRef(false);
  const saveRequestRef = useRef(0);
  const checkRequestRef = useRef(0);
  const checkingRef = useRef(false);
  translateRef.current = t;

  const showDraft = useCallback((next: UserPreferences) => {
    const normalized = normalizePreferences(next);
    preferencesRef.current = normalized;
    setPreferences(normalized);
    return normalized;
  }, []);

  const publishAuthoritative = useCallback((next: UserPreferences) => {
    const normalized = showDraft(next);
    lastSavedRef.current = normalized;
    setUserPreferences(normalized);
    return normalized;
  }, [showDraft]);

  useEffect(() => {
    let cancelled = false;
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    saveRequestRef.current += 1;
    checkRequestRef.current += 1;
    savingRef.current = false;
    checkingRef.current = false;
    lastSavedRef.current = null;
    pendingRef.current = null;
    showDraft(normalizePreferences(null));
    setLoading(true);
    setSaving(false);
    setChecking(false);
    setRepairing(false);
    setWritable(false);
    setComparisonReady(false);
    setProjectionRepairReady(false);
    setFeedback(null);
    void getUserPreferencesApi()
      .then((result) => {
        if (
          cancelled
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        const normalized = requirePreferencesVersion(result);
        publishAuthoritative(normalized);
        pendingRef.current = null;
        setComparisonReady(false);
        setProjectionRepairReady(false);
        setFeedback(null);
        setWritable(true);
      })
      .catch(() => {
        if (
          !cancelled
          && isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          const translate = translateRef.current;
          setFeedback(loadFailureFeedback(translate));
        }
      })
      .finally(() => {
        if (
          !cancelled
          && isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [authOwnerReloadKey, publishAuthoritative, showDraft]);

  const persistAtVersion = useCallback(async (
    draft: UserPreferences,
    base: UserPreferences,
  ) => {
    if (savingRef.current || !validPreferencesVersion(base.version)) {
      return null;
    }
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const requestId = saveRequestRef.current + 1;
    saveRequestRef.current = requestId;
    savingRef.current = true;
    const optimistic = showDraft({ ...draft, version: base.version });
    setFeedback(null);
    setSaving(true);

    try {
      const result = await updateUserPreferencesApi(
        buildPreferencesUpdatePayload(optimistic),
        { expectedVersion: base.version },
      );
      if (
        saveRequestRef.current !== requestId
        || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
      ) {
        return null;
      }
      const saved = publishAuthoritative(requirePreferencesVersion(result));
      pendingRef.current = null;
      setComparisonReady(false);
      setProjectionRepairReady(false);
      setWritable(true);
      return saved;
    } catch (error) {
      if (
        saveRequestRef.current !== requestId
        || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
      ) {
        return null;
      }
      const failure = projectMutationFailure(
        error,
        t("settings.general.preferences_save_failed"),
      );
      const needsReconciliation = failure.effect !== "not_applied"
        || failure.code === "preferences.version_conflict";
      if (needsReconciliation) {
        pendingRef.current = {
          base,
          draft: optimistic,
          latest: null,
          projectionRepairRequired:
            failure.code === "preferences.projection_result_unknown",
        };
        setComparisonReady(false);
        setProjectionRepairReady(false);
        setWritable(false);
        setFeedback(failure.code === "preferences.version_conflict"
          ? conflictFeedback(t)
          : unknownResultFeedback(t));
      } else {
        // 已确认未写入时仍保留本页草稿；lastSaved/global runtime
        // 继续指向服务端基线，避免未确认值影响其他页面。
        showDraft(optimistic);
        setProjectionRepairReady(false);
        setWritable(true);
        setFeedback(notAppliedFeedback(t));
      }
      throw normalizeError(error, t("settings.general.preferences_save_failed"));
    } finally {
      if (saveRequestRef.current === requestId) {
        savingRef.current = false;
        if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          setSaving(false);
        }
      }
    }
  }, [publishAuthoritative, showDraft, t]);

  const persistPreferences = useCallback(async (next: UserPreferences) => {
    if (!writable || pendingRef.current) {
      return null;
    }
    const base = lastSavedRef.current;
    if (!base || !validPreferencesVersion(base.version)) {
      setWritable(false);
      setFeedback(loadFailureFeedback(t));
      return null;
    }
    return persistAtVersion(next, base);
  }, [persistAtVersion, t, writable]);

  const checkLatest = useCallback(() => {
    if (checkingRef.current) {
      return;
    }
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const requestId = checkRequestRef.current + 1;
    checkRequestRef.current = requestId;
    checkingRef.current = true;
    setProjectionRepairReady(false);
    setWritable(false);
    setChecking(true);
    void getUserPreferencesApi()
      .then((result) => {
        if (
          checkRequestRef.current !== requestId
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        const latest = requirePreferencesVersion(result);
        const pending = pendingRef.current;
        if (!pending) {
          publishAuthoritative(latest);
          setComparisonReady(false);
          setProjectionRepairReady(false);
          setFeedback(null);
          setWritable(true);
          return;
        }
        pending.latest = normalizePreferences(latest);
        if (pending.projectionRepairRequired) {
          setComparisonReady(false);
          setProjectionRepairReady(true);
          setWritable(false);
          setFeedback(projectionRepairRequiredFeedback(t));
          return;
        }
        if (equivalentPreferences(pending.latest, pending.draft)) {
          publishAuthoritative(pending.latest);
          pendingRef.current = null;
          setComparisonReady(false);
          setProjectionRepairReady(false);
          setWritable(true);
          setFeedback(reconciledCommittedFeedback(t));
          return;
        }
        setComparisonReady(true);
        setProjectionRepairReady(false);
        setWritable(false);
        setFeedback(equivalentPreferences(pending.latest, pending.base)
          ? reconciledNotAppliedFeedback(t)
          : reconciledDifferenceFeedback(t));
      })
      .catch(() => {
        if (
          checkRequestRef.current !== requestId
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        const hasDraft = pendingRef.current !== null;
        setFeedback(checkFailureFeedback(hasDraft, t));
        setWritable(!hasDraft && lastSavedRef.current !== null);
      })
      .finally(() => {
        if (checkRequestRef.current === requestId) {
          checkingRef.current = false;
          if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
            setChecking(false);
          }
        }
      });
  }, [publishAuthoritative, t]);

  const repairProjectionSnapshot = useCallback(async () => {
    const pending = pendingRef.current;
    if (!pending?.latest || savingRef.current) {
      return null;
    }
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const requestId = saveRequestRef.current + 1;
    saveRequestRef.current = requestId;
    savingRef.current = true;
    setSaving(true);
    setRepairing(true);
    setProjectionRepairReady(false);
    setFeedback(null);
    const latest = pending.latest;
    try {
      const result = await updateUserPreferencesApi(
        buildPreferencesUpdatePayload(latest),
        { expectedVersion: latest.version },
      );
      if (
        saveRequestRef.current !== requestId
        || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
      ) {
        return null;
      }
      const saved = publishAuthoritative(requirePreferencesVersion(result));
      const rebasedDraft = rebasePreferenceDraft(
        pending.base,
        pending.draft,
        saved,
      );
      if (equivalentPreferences(saved, rebasedDraft)) {
        pendingRef.current = null;
        setComparisonReady(false);
        setProjectionRepairReady(false);
        setWritable(true);
        setFeedback(projectionRepairCompletedFeedback(t));
        return saved;
      }
      pendingRef.current = {
        base: saved,
        draft: rebasedDraft,
        latest: saved,
        projectionRepairRequired: false,
      };
      showDraft(rebasedDraft);
      setComparisonReady(true);
      setWritable(false);
      setFeedback(projectionRepairCompletedWithDraftFeedback(t));
      return saved;
    } catch (error) {
      if (
        saveRequestRef.current !== requestId
        || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
      ) {
        return null;
      }
      const failure = projectMutationFailure(
        error,
        t("settings.general.preferences_save_failed"),
      );
      const currentPending = pendingRef.current;
      if (currentPending) {
        currentPending.latest = null;
        currentPending.projectionRepairRequired =
          failure.code === "preferences.projection_result_unknown"
          || currentPending.projectionRepairRequired;
      }
      setComparisonReady(false);
      setProjectionRepairReady(false);
      setWritable(false);
      setFeedback(failure.code === "preferences.version_conflict"
        ? conflictFeedback(t)
        : unknownResultFeedback(t));
      throw normalizeError(error, t("settings.general.preferences_save_failed"));
    } finally {
      if (saveRequestRef.current === requestId) {
        savingRef.current = false;
        if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          setRepairing(false);
          setSaving(false);
        }
      }
    }
  }, [publishAuthoritative, showDraft, t]);

  const repairProjection = useCallback(() => {
    void repairProjectionSnapshot().catch(() => {});
  }, [repairProjectionSnapshot]);

  const reapplyDraft = useCallback(() => {
    const pending = pendingRef.current;
    if (!pending?.latest || savingRef.current) {
      return;
    }
    const rebased = rebasePreferenceDraft(
      pending.base,
      pending.draft,
      pending.latest,
    );
    pendingRef.current = null;
    setComparisonReady(false);
    setProjectionRepairReady(false);
    void persistAtVersion(rebased, pending.latest).catch(() => {});
  }, [persistAtVersion]);

  const updatePreferences = useCallback((mutate: PreferenceMutation) => {
    void persistPreferences(mutate(preferencesRef.current)).catch(() => {});
  }, [persistPreferences]);

  const getCurrentPreferences = useCallback(
    () => preferencesRef.current,
    [],
  );

  const acceptExternalAggregateSnapshot = useCallback((
    expectedVersion: number,
    snapshot: { enabled: boolean; version: number },
  ): boolean => {
    if (
      savingRef.current
      || pendingRef.current
      || !validPreferencesVersion(expectedVersion)
      || !validPreferencesVersion(snapshot.version)
    ) {
      return false;
    }
    const current = lastSavedRef.current;
    if (!current) {
      return false;
    }
    if (current.version === snapshot.version) {
      publishAuthoritative({ ...current, echo_enabled: snapshot.enabled });
      return true;
    }
    if (current.version !== expectedVersion || snapshot.version <= expectedVersion) {
      return false;
    }
    publishAuthoritative({
      ...current,
      echo_enabled: snapshot.enabled,
      version: snapshot.version,
    });
    return true;
  }, [publishAuthoritative]);

  const recovery: PreferenceRecoveryControls = {
    canCompare: comparisonReady,
    canRepairProjection: projectionRepairReady,
    checking,
    checkLatest,
    reapplyDraft,
    repairProjection,
    repairing,
  };

  return {
    acceptExternalAggregateSnapshot,
    feedback,
    getCurrentPreferences,
    hasUnresolvedMutation: pendingRef.current !== null,
    loading,
    persistPreferences,
    preferences,
    recovery,
    saving,
    setFeedback,
    updatePreferences,
    writable,
  };
}

function validPreferencesVersion(value: number | undefined): value is number {
  return Number.isSafeInteger(value) && (value ?? 0) > 0;
}

function requirePreferencesVersion(value: UserPreferences): UserPreferences {
  if (!validPreferencesVersion(value.version)) {
    throw new Error("Preferences response is missing a valid version");
  }
  return normalizePreferences(value);
}

function normalizeError(error: unknown, fallback: string): Error {
  return error instanceof Error ? error : new Error(fallback);
}

type Translate = ReturnType<typeof useI18n>["t"];

function preferencesAuthOwnerReloadKey(
  status: ReturnType<typeof useAuth>["status"],
): string {
  if (!status) {
    return "pending";
  }
  if (!status.authenticated) {
    return "signed-out";
  }
  return [
    "signed-in",
    status.user_id?.trim() ?? "",
    status.username?.trim() ?? "",
  ].join("\u001f");
}

function loadFailureFeedback(t: Translate): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_load_impact"),
    title: t("settings.general.preferences_load_title"),
    tone: "error",
  };
}

function conflictFeedback(t: Translate): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_conflict_impact"),
    title: t("settings.general.preferences_conflict_title"),
    tone: "warning",
  };
}

function unknownResultFeedback(t: Translate): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_unknown_impact"),
    title: t("settings.general.preferences_unknown_title"),
    tone: "warning",
  };
}

function notAppliedFeedback(t: Translate): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_not_applied_impact"),
    title: t("settings.general.preferences_not_applied_title"),
    tone: "error",
  };
}

function checkFailureFeedback(
  hasDraft: boolean,
  t: Translate,
): PreferenceFeedback {
  return {
    impact: hasDraft
      ? t("settings.general.preferences_check_failed_draft_impact")
      : t("settings.general.preferences_load_impact"),
    title: t("settings.general.preferences_check_failed_title"),
    tone: "error",
  };
}

function reconciledCommittedFeedback(t: Translate): PreferenceFeedback {
  return {
    message: t("settings.general.preferences_committed_message"),
    title: t("settings.general.preferences_committed_title"),
    tone: "success",
  };
}

function reconciledNotAppliedFeedback(t: Translate): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_reconciled_not_applied_impact"),
    title: t("settings.general.preferences_reconciled_not_applied_title"),
    tone: "warning",
  };
}

function reconciledDifferenceFeedback(t: Translate): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_reconciled_difference_impact"),
    title: t("settings.general.preferences_reconciled_difference_title"),
    tone: "warning",
  };
}

function projectionRepairRequiredFeedback(t: Translate): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_projection_repair_impact"),
    title: t("settings.general.preferences_projection_repair_title"),
    tone: "warning",
  };
}

function projectionRepairCompletedFeedback(t: Translate): PreferenceFeedback {
  return {
    message: t("settings.general.preferences_projection_repaired_message"),
    title: t("settings.general.preferences_projection_repaired_title"),
    tone: "success",
  };
}

function projectionRepairCompletedWithDraftFeedback(
  t: Translate,
): PreferenceFeedback {
  return {
    impact: t("settings.general.preferences_projection_repaired_draft_impact"),
    title: t("settings.general.preferences_projection_repaired_title"),
    tone: "warning",
  };
}
