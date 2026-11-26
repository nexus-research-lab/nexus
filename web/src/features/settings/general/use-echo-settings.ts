/**
 * INPUT: owner-scoped Echo PUT/对账 GET、Preferences 权威快照与领域 failure effect。
 * OUTPUT: CAS 保存、结果未知对账和单一安全恢复动作；关闭只改开关，已安排的 Echo 到点即静默，无需前端收口。
 * POS: 主动跟进设置的唯一前端事务边界；未知写入不自动重放且不被其他资源刷新解锁。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import {
  getEchoApi,
  updateEchoApi,
  type EchoSettings,
} from "@/lib/api/settings/echo-api";
import { projectMutationFailure } from "@/lib/error-message";
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

type RecoveryPurpose = "normal" | "reapply";

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
  const [writable, setWritable] = useState(false);
  const [comparisonReady, setComparisonReady] = useState(false);
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
      setWritable(false);
      setComparisonReady(false);
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
    saveRequestRef.current = requestId;
    savingRef.current = true;
    setEnabled(desired);
    setSaving(true);
    setWritable(false);
    setComparisonReady(false);
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
        pendingRef.current = { base, desired, latest: null };
        setEnabled(desired);
        setWritable(false);
        setFeedback(failure.code === "echo.version_conflict"
          ? echoConflict(t)
          : failure.effect === "committed"
            ? echoCommittedNeedsCheck(t)
            : failure.effect === "not_applied"
              ? echoRecoveryNotApplied(t)
              : echoUnknown(t));
        setCheckReady(true);
      } else {
        pendingRef.current = null;
        setEnabled(base.enabled);
        setWritable(true);
        setFeedback(echoNotApplied(t));
        setCheckReady(false);
      }
      return null;
    } finally {
      if (saveRequestRef.current === requestId) {
        savingRef.current = false;
        if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          setSaving(false);
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
      .catch(() => {
        if (
          checkRequestRef.current !== requestId
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        setFeedback(echoCheckFailure(pendingRef.current !== null, t));
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

  const reapplyChange = useCallback(() => {
    const pending = pendingRef.current;
    if (!pending?.latest) {
      return;
    }
    void submitAtVersion(pending.desired, pending.latest, "reapply");
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
    checking,
    checkLatest,
    reapplyChange,
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
  | "echo_committed"
  | "echo_committed_needs_check"
  | "echo_conflict"
  | "echo_difference"
  | "echo_load_failure"
  | "echo_not_applied"
  | "echo_reapplied"
  | "echo_recovery_not_applied"
  | "echo_unknown";

function feedback(
  prefix: EchoFeedbackPrefix,
  t: Translate,
  tone: EchoSettingsFeedback["tone"],
): EchoSettingsFeedback {
  if (tone === "success") {
    return {
      title: t(`settings.general.${prefix}_title`),
      message: t(`settings.general.${prefix}_message`),
      tone,
    };
  }
  return {
    title: t(`settings.general.${prefix}_title`),
    impact: t(`settings.general.${prefix}_impact`),
    tone,
  };
}

const echoConflict = (t: Translate) =>
  feedback("echo_conflict", t, "warning");
const echoUnknown = (t: Translate) =>
  feedback("echo_unknown", t, "warning");
const echoNotApplied = (t: Translate) =>
  feedback("echo_not_applied", t, "error");
const echoCommitted = (t: Translate) =>
  feedback("echo_committed", t, "success");
const echoCommittedNeedsCheck = (t: Translate) =>
  feedback("echo_committed_needs_check", t, "warning");
const echoRecoveryNotApplied = (t: Translate) =>
  feedback("echo_recovery_not_applied", t, "error");
const echoDifference = (t: Translate) =>
  feedback("echo_difference", t, "warning");
const echoCheckFailure = (hasPending: boolean, t: Translate) =>
  feedback(hasPending ? "echo_check_failure_pending" : "echo_load_failure", t, "error");

function successFeedback(
  purpose: RecoveryPurpose,
  t: Translate,
): EchoSettingsFeedback | null {
  return purpose === "reapply" ? feedback("echo_reapplied", t, "success") : null;
}
