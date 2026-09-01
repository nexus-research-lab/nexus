/**
 * INPUT: owner-scoped Echo PUT/对账 GET、Preferences 权威快照与领域 failure effect。
 * OUTPUT: CAS 保存、结果未知对账、关闭后收口修复和完整恢复提示。
 * POS: 主动跟进设置的唯一前端事务边界；未知写入不自动重放且不被其他资源刷新解锁。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import {
  getEchoApi,
  updateEchoApi,
  type EchoSettings,
} from "@/lib/api/settings/echo-api";
import { getErrorMessage, projectMutationFailure } from "@/lib/error-message";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
} from "@/shared/auth/auth-owner-generation";
import { useI18n } from "@/shared/i18n/i18n-context";

import type {
  EchoSettingsFeedback,
  EchoSettingsRecoveryControls,
  PendingEchoChange,
} from "./model/echo-settings-reliability-model";
import { validEchoSettings } from "./model/echo-settings-reliability-model";

type RecoveryPurpose = "finish-disabling" | "normal" | "reapply" | "use-latest";

export function useEchoSettings({
  aggregate,
  aggregateLoading,
  blocked,
  onAggregateSnapshot,
}: {
  aggregate: EchoSettings | null;
  aggregateLoading: boolean;
  blocked: boolean;
  onAggregateSnapshot: (expectedVersion: number, snapshot: EchoSettings) => void;
}) {
  const { t } = useI18n();
  const [enabled, setEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [checking, setChecking] = useState(false);
  const [repairing, setRepairing] = useState(false);
  const [writable, setWritable] = useState(false);
  const [comparisonReady, setComparisonReady] = useState(false);
  const [cleanupRepairReady, setCleanupRepairReady] = useState(false);
  const [checkReady, setCheckReady] = useState(false);
  const [feedback, setFeedback] = useState<EchoSettingsFeedback | null>(null);
  const authoritativeRef = useRef<EchoSettings | null>(null);
  const pendingRef = useRef<PendingEchoChange | null>(null);
  const savingRef = useRef(false);
  const checkingRef = useRef(false);
  const saveRequestRef = useRef(0);
  const checkRequestRef = useRef(0);

  const publishAuthoritative = useCallback((settings: EchoSettings) => {
    if (!validEchoSettings(settings)) {
      throw new Error("Echo response is missing a valid version");
    }
    authoritativeRef.current = settings;
    setEnabled(settings.enabled);
    return settings;
  }, []);

  useEffect(() => {
    if (aggregateLoading) {
      saveRequestRef.current += 1;
      checkRequestRef.current += 1;
      savingRef.current = false;
      checkingRef.current = false;
      authoritativeRef.current = null;
      pendingRef.current = null;
      setEnabled(false);
      setSaving(false);
      setChecking(false);
      setRepairing(false);
      setWritable(false);
      setComparisonReady(false);
      setCleanupRepairReady(false);
      setCheckReady(false);
      setFeedback(null);
      return;
    }
    if (
      blocked
      || savingRef.current
      || checkingRef.current
      || pendingRef.current
      || !aggregate
      || !validEchoSettings(aggregate)
    ) {
      return;
    }
    publishAuthoritative(aggregate);
    setWritable(true);
  }, [aggregate, aggregateLoading, blocked, publishAuthoritative]);

  const submitAtVersion = useCallback(async (
    desired: boolean,
    base: EchoSettings,
    purpose: RecoveryPurpose,
  ) => {
    if (savingRef.current || !validEchoSettings(base)) {
      return null;
    }
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const requestId = saveRequestRef.current + 1;
    const previousPending = pendingRef.current;
    saveRequestRef.current = requestId;
    savingRef.current = true;
    setEnabled(desired);
    setSaving(true);
    setRepairing(purpose === "finish-disabling");
    setWritable(false);
    setComparisonReady(false);
    setCleanupRepairReady(false);
    setCheckReady(false);
    setFeedback(null);
    try {
      const settings = await updateEchoApi(
        { enabled: desired },
        { expectedVersion: base.version },
      );
      if (
        saveRequestRef.current !== requestId
        || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
      ) {
        return null;
      }
      const saved = publishAuthoritative(settings);
      pendingRef.current = null;
      setWritable(true);
      onAggregateSnapshot(base.version, saved);
      setFeedback(successFeedback(purpose, t));
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
        t("settings.general.echo_save_failed"),
      );
      const needsReconciliation = failure.effect !== "not_applied"
        || failure.code === "echo.version_conflict"
        || purpose !== "normal";
      if (needsReconciliation) {
        pendingRef.current = {
          base,
          desired,
          latest: null,
          cleanupRepairRequired:
            failure.code === "echo.cleanup_incomplete"
            || previousPending?.cleanupRepairRequired === true,
        };
        setEnabled(desired);
        setWritable(false);
        setFeedback(failure.code === "echo.cleanup_incomplete"
          ? echoCleanupIncomplete(failure.message, t)
          : failure.code === "echo.version_conflict"
            ? echoConflict(failure.message, t)
            : failure.effect === "committed"
              ? echoCommittedNeedsCheck(failure.message, t)
              : failure.effect === "not_applied"
                ? echoRecoveryNotApplied(failure.message, t)
                : echoUnknown(failure.message, t));
        setCheckReady(true);
      } else {
        pendingRef.current = null;
        setEnabled(base.enabled);
        setWritable(true);
        setFeedback(echoNotApplied(failure.message, t));
        setCheckReady(false);
      }
      return null;
    } finally {
      if (saveRequestRef.current === requestId) {
        savingRef.current = false;
        if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          setSaving(false);
          setRepairing(false);
        }
      }
    }
  }, [onAggregateSnapshot, publishAuthoritative, t]);

  const checkLatest = useCallback(() => {
    if (checkingRef.current) {
      return;
    }
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const requestId = checkRequestRef.current + 1;
    checkRequestRef.current = requestId;
    checkingRef.current = true;
    setChecking(true);
    setWritable(false);
    setComparisonReady(false);
    setCleanupRepairReady(false);
    setCheckReady(false);
    void getEchoApi()
      .then((settings) => {
        if (
          checkRequestRef.current !== requestId
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        if (!validEchoSettings(settings)) {
          throw new Error("Echo response is missing a valid version");
        }
        const pending = pendingRef.current;
        const expectedVersion = pending?.base.version
          ?? authoritativeRef.current?.version
          ?? settings.version;
        onAggregateSnapshot(expectedVersion, settings);
        if (!pending) {
          publishAuthoritative(settings);
          setWritable(true);
          setFeedback(null);
          return;
        }
        pending.latest = settings;
        if (pending.cleanupRepairRequired) {
          if (!settings.enabled) {
            setEnabled(false);
            setCleanupRepairReady(true);
            setCheckReady(false);
            setFeedback(echoCleanupRepairRequired(t));
            return;
          }
          publishAuthoritative(settings);
          pendingRef.current = null;
          setWritable(true);
          setCheckReady(false);
          setFeedback(echoCleanupSuperseded(t));
          return;
        }
        // A lost response cannot prove that the post-save cancellation stage ran.
        // Repeating only the idempotent disable cleanup is safer than declaring
        // the whole flow complete or replaying an unrelated user action.
        if (!pending.desired && !settings.enabled) {
          setEnabled(false);
          setCleanupRepairReady(true);
          setCheckReady(false);
          setFeedback(echoCleanupRepairRequired(t));
          return;
        }
        if (settings.enabled === pending.desired) {
          publishAuthoritative(settings);
          pendingRef.current = null;
          setWritable(true);
          setCheckReady(false);
          setFeedback(echoCommitted(t));
          return;
        }
        setEnabled(pending.desired);
        setComparisonReady(true);
        setCheckReady(false);
        setFeedback(echoDifference(t));
      })
      .catch((error: unknown) => {
        if (
          checkRequestRef.current !== requestId
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        setFeedback(echoCheckFailure(
          getErrorMessage(error, t("settings.general.echo_load_failed")),
          pendingRef.current !== null,
          t,
        ));
        setCheckReady(true);
        setWritable(pendingRef.current === null && authoritativeRef.current !== null);
      })
      .finally(() => {
        if (checkRequestRef.current === requestId) {
          checkingRef.current = false;
          if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
            setChecking(false);
          }
        }
      });
  }, [onAggregateSnapshot, publishAuthoritative, t]);

  const useLatest = useCallback(() => {
    const pending = pendingRef.current;
    if (!pending?.latest) {
      return;
    }
    void submitAtVersion(pending.latest.enabled, pending.latest, "use-latest");
  }, [submitAtVersion]);

  const reapplyChange = useCallback(() => {
    const pending = pendingRef.current;
    if (!pending?.latest) {
      return;
    }
    void submitAtVersion(pending.desired, pending.latest, "reapply");
  }, [submitAtVersion]);

  const finishDisabling = useCallback(() => {
    const pending = pendingRef.current;
    if (!pending?.latest || pending.latest.enabled) {
      return;
    }
    void submitAtVersion(false, pending.latest, "finish-disabling");
  }, [submitAtVersion]);

  const handleEnabledChange = useCallback((desired: boolean) => {
    const base = authoritativeRef.current;
    if (blocked || !writable || !base || desired === enabled) {
      return;
    }
    void submitAtVersion(
      desired,
      base,
      "normal",
    );
  }, [blocked, enabled, submitAtVersion, writable]);

  const recovery: EchoSettingsRecoveryControls = {
    canCheckLatest: checkReady,
    canCompare: comparisonReady,
    canFinishDisabling: cleanupRepairReady,
    checking,
    checkLatest,
    finishDisabling,
    reapplyChange,
    repairing,
    useLatest,
  };

  return {
    disabled: blocked || aggregateLoading || saving || !writable,
    enabled,
    feedback,
    handleEnabledChange,
    hasUnresolvedMutation: pendingRef.current !== null,
    loading: aggregateLoading,
    recovery,
    saving,
  };
}

type Translate = ReturnType<typeof useI18n>["t"];
type EchoFeedbackPrefix =
  | "echo_check_failure_pending"
  | "echo_cleanup_completed"
  | "echo_cleanup_incomplete"
  | "echo_cleanup_repair"
  | "echo_cleanup_superseded"
  | "echo_committed"
  | "echo_committed_needs_check"
  | "echo_conflict"
  | "echo_difference"
  | "echo_latest_selected"
  | "echo_load_failure"
  | "echo_not_applied"
  | "echo_reapplied"
  | "echo_recovery_not_applied"
  | "echo_unknown";

function feedback(
  prefix: EchoFeedbackPrefix,
  t: Translate,
  tone: EchoSettingsFeedback["tone"],
  message?: string,
): EchoSettingsFeedback {
  return {
    title: t(`settings.general.${prefix}_title`),
    message: message ?? t(`settings.general.${prefix}_message`),
    impact: t(`settings.general.${prefix}_impact`),
    nextStep: t(`settings.general.${prefix}_next_step`),
    tone,
  };
}

const echoConflict = (message: string, t: Translate) =>
  feedback("echo_conflict", t, "warning", message);
const echoUnknown = (message: string, t: Translate) =>
  feedback("echo_unknown", t, "warning", message);
const echoNotApplied = (message: string, t: Translate) =>
  feedback("echo_not_applied", t, "error", message);
const echoCleanupIncomplete = (message: string, t: Translate) =>
  feedback("echo_cleanup_incomplete", t, "warning", message);
const echoCleanupRepairRequired = (t: Translate) =>
  feedback("echo_cleanup_repair", t, "warning");
const echoCleanupSuperseded = (t: Translate) =>
  feedback("echo_cleanup_superseded", t, "success");
const echoCommitted = (t: Translate) =>
  feedback("echo_committed", t, "success");
const echoCommittedNeedsCheck = (message: string, t: Translate) =>
  feedback("echo_committed_needs_check", t, "warning", message);
const echoRecoveryNotApplied = (message: string, t: Translate) =>
  feedback("echo_recovery_not_applied", t, "error", message);
const echoDifference = (t: Translate) =>
  feedback("echo_difference", t, "warning");
const echoCheckFailure = (message: string, hasPending: boolean, t: Translate) =>
  feedback(hasPending ? "echo_check_failure_pending" : "echo_load_failure", t, "error", message);

function successFeedback(
  purpose: RecoveryPurpose,
  t: Translate,
): EchoSettingsFeedback | null {
  switch (purpose) {
    case "finish-disabling":
      return feedback("echo_cleanup_completed", t, "success");
    case "reapply":
      return feedback("echo_reapplied", t, "success");
    case "use-latest":
      return feedback("echo_latest_selected", t, "success");
    default:
      return null;
  }
}
