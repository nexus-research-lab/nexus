/**
 * INPUT: Exact Channel login session, shared mutation gate, and committed-channel refresh.
 * OUTPUT: Stale-safe polling, verification reconciliation, and idempotent login recovery.
 * POS: Channel login lifecycle owner; it never replays configuration writes or exposes raw errors.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  getCurrentChannelLoginApi,
  getChannelLoginApi,
  startChannelLoginApi,
  submitChannelLoginVerifyCodeApi,
  type ChannelLoginView,
  type ImChannelType,
} from "@/lib/api/capability/channel-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";

import {
  buildChannelOperationIssue,
  channelOperationNeedsReconciliation,
  type ChannelOperationIssue,
} from "../../channel-operation-recovery";
import type { ChannelPendingAction } from "../channel-connection-model";
import type { RunChannelCommand } from "../use-channel-command";
import { isChannelLoginRunning } from "./channel-login-model";

type LoginReadIssue = "completion_refresh" | "poll" | null;

interface LoginMutationRecovery {
  check: "failed" | "not_checked" | "unproven";
  issue: ChannelOperationIssue;
  kind: "start" | "verify";
  loginId: string;
}

interface UseChannelLoginOptions {
  channelType: ImChannelType;
  enabled: boolean;
  onCompleted: () => Promise<void>;
  pendingAction: ChannelPendingAction | null;
  runCommand: RunChannelCommand;
}

export function useChannelLoginController({
  channelType,
  enabled,
  onCompleted,
  pendingAction,
  runCommand,
}: UseChannelLoginOptions) {
  const { t } = useI18n();
  const startPendingRef = useRef(false);
  const [view, setView] = useState<ChannelLoginView | null>(null);
  const [readIssue, setReadIssue] = useState<LoginReadIssue>(null);
  const [pollGeneration, setPollGeneration] = useState(0);
  const [startPending, setStartPending] = useState(false);
  const [recovery, setRecovery] = useState<LoginMutationRecovery | null>(null);
  const running = isChannelLoginRunning(view);

  const refreshCompletedChannel = useCallback(async (): Promise<boolean> => {
    try {
      await onCompleted();
      setReadIssue(null);
      return true;
    } catch {
      setReadIssue("completion_refresh");
      return false;
    }
  }, [onCompleted]);

  useEffect(() => {
    if (!enabled || !view?.login_id || !running) {
      return;
    }

    let disposed = false;
    let timer = 0;
    const poll = async () => {
      try {
        const nextView = await getChannelLoginApi(channelType, view.login_id);
        if (disposed) {
          return;
        }
        setView(nextView);
        setReadIssue(null);
        if (nextView.status === "succeeded") {
          await refreshCompletedChannel();
          return;
        }
        if (nextView.status === "running") {
          timer = window.setTimeout(poll, 1500);
        }
      } catch {
        if (!disposed) {
          setReadIssue("poll");
        }
      }
    };

    timer = window.setTimeout(poll, 1500);
    return () => {
      disposed = true;
      window.clearTimeout(timer);
    };
  }, [
    channelType,
    enabled,
    pollGeneration,
    refreshCompletedChannel,
    running,
    view?.login_id,
  ]);

  const startLogin = useCallback(async (): Promise<boolean> => {
    if (!enabled || startPendingRef.current) {
      return false;
    }
    startPendingRef.current = true;
    setStartPending(true);
    try {
      const nextView = await startChannelLoginApi(channelType);
      setView(nextView);
      setRecovery(null);
      setReadIssue(null);
      return true;
    } catch (error) {
      setRecovery({
        check: "not_checked",
        issue: buildChannelOperationIssue(error, "login_start", t),
        kind: "start",
        loginId: "",
      });
      return false;
    } finally {
      startPendingRef.current = false;
      setStartPending(false);
    }
  }, [channelType, enabled, t]);

  const reconcileVerifyCode = useCallback(async () => {
    if (!recovery || recovery.kind !== "verify" || !recovery.loginId) {
      return;
    }
    try {
      const nextView = await getChannelLoginApi(channelType, recovery.loginId);
      setView(nextView);
      if (nextView.status !== "verify_code_required") {
        setRecovery(null);
        setReadIssue(null);
        return;
      }
      setRecovery((current) => current?.kind === "verify"
        ? { ...current, check: "unproven" }
        : current);
    } catch {
      setRecovery((current) => current?.kind === "verify"
        ? { ...current, check: "failed" }
        : current);
    }
  }, [channelType, recovery]);

  const reconcileLoginStart = useCallback(async () => {
    if (!recovery || recovery.kind !== "start") {
      return;
    }
    try {
      // Recover only the unique active, unbound Web login. This GET never
      // starts or registers another QR flow.
      const currentLogin = await getCurrentChannelLoginApi(channelType);
      setView(currentLogin);
      setRecovery(null);
      setReadIssue(null);
    } catch (error) {
      const reconciliationIsUnproven = error instanceof ApiRequestError
        && (error.status === 404 || error.status === 409);
      if (reconciliationIsUnproven) {
        const refreshed = await refreshCompletedChannel();
        setRecovery((current) => current?.kind === "start"
          ? { ...current, check: refreshed ? "unproven" : "failed" }
          : current);
        return;
      }
      setRecovery((current) => current?.kind === "start"
        ? { ...current, check: "failed" }
        : current);
    }
  }, [channelType, recovery, refreshCompletedChannel]);

  const submitVerifyCode = useCallback(async (value: string) => {
    if (!enabled || !view?.login_id) {
      return false;
    }
    if (recovery && !(
      recovery.kind === "verify"
      && recovery.issue.effect === "not_applied"
    )) {
      return false;
    }
    const loginId = view.login_id;
    if (recovery?.kind === "verify") {
      setRecovery(null);
    }
    const result = await runCommand({ kind: "verify-code" }, async () => {
      try {
        setView(await submitChannelLoginVerifyCodeApi(
          channelType,
          loginId,
          value,
        ));
        setRecovery(null);
        return true;
      } catch (error) {
        setRecovery({
          check: "not_checked",
          issue: buildChannelOperationIssue(error, "verify_code", t),
          kind: "verify",
          loginId,
        });
        return false;
      }
    });
    return result ?? false;
  }, [channelType, enabled, recovery, runCommand, t, view?.login_id]);

  const recoveryNotice = useMemo<FeedbackBannerProps | null>(() => {
    if (recovery) {
      const needsReconciliation = channelOperationNeedsReconciliation(
        recovery.issue,
      );
      const checkingFailed = recovery.check === "failed";
      const unproven = recovery.check === "unproven";
      const startCanRetry = recovery.kind === "start"
        && recovery.issue.effect === "not_applied";
      const message = checkingFailed
        ? t("capability.channel_reconcile_failed_message")
        : unproven
          ? t("capability.channel_reconcile_unproven_message")
          : recovery.issue.message;
      const impact = unproven
        ? t("capability.channel_reconcile_unproven_impact")
        : recovery.issue.impact;
      const nextStep = checkingFailed
        ? t("capability.channel_reconcile_failed_next_step")
        : unproven
          ? t("capability.channel_reconcile_unproven_next_step")
          : startCanRetry
            ? t("capability.channel_login_start_recovery_next_step")
            : recovery.issue.nextStep;
      return {
        action: {
          label: startCanRetry
            ? t("capability.channel_login_resume_action")
            : unproven
              ? t("capability.channel_start_new_intent_action")
              : t("capability.channel_reconcile_action"),
          onClick: () => {
            if (startCanRetry) {
              void startLogin();
            } else if (unproven) {
              setRecovery(null);
            } else if (recovery.kind === "start") {
              void reconcileLoginStart();
            } else {
              void reconcileVerifyCode();
            }
          },
        },
        impact,
        message,
        nextStep,
        onDismiss: needsReconciliation ? undefined : () => setRecovery(null),
        title: recovery.issue.title,
        tone: recovery.issue.tone,
      };
    }
    if (readIssue === "completion_refresh") {
      return {
        action: {
          label: t("capability.channel_reconcile_action"),
          onClick: () => void refreshCompletedChannel(),
        },
        impact: t("capability.channel_login_refresh_failed_impact"),
        message: t("capability.channel_login_refresh_failed_message"),
        nextStep: t("capability.channel_login_refresh_failed_next_step"),
        title: t("capability.channel_login_refresh_failed_title"),
        tone: "warning",
      };
    }
    if (readIssue === "poll") {
      return {
        action: {
          label: t("capability.channel_reconcile_action"),
          onClick: () => {
            setReadIssue(null);
            setPollGeneration((current) => current + 1);
          },
        },
        impact: t("capability.channel_login_poll_failed_impact"),
        message: t("capability.channel_login_poll_failed_message"),
        nextStep: t("capability.channel_login_poll_failed_next_step"),
        title: t("capability.channel_login_poll_failed_title"),
        tone: "error",
      };
    }
    return null;
  }, [
    readIssue,
    reconcileLoginStart,
    reconcileVerifyCode,
    recovery,
    refreshCompletedChannel,
    startLogin,
    t,
  ]);

  const mutationBlocked = recovery !== null
    && (
      recovery.kind === "start"
      || recovery.issue.effect !== "not_applied"
    );

  return {
    loading: pendingAction?.kind === "save"
      || pendingAction?.kind === "verify-code"
      || startPending,
    mutationBlocked,
    recoveryNotice,
    running,
    startLogin,
    submitVerifyCode,
    view,
  };
}
