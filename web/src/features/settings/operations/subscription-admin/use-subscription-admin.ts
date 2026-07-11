import { useCallback, useEffect, useMemo, useState } from "react";

import {
  createSubscriptionPlanApi,
  getSubscriptionOverviewApi,
  updateSubscriptionPlanApi,
  updateUserSubscriptionApi,
} from "@/lib/api/account/subscription-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  SubscriptionAccount,
  SubscriptionOverview,
  SubscriptionPlan,
} from "@/types/settings/subscription";

import {
  EMPTY_SUBSCRIPTION_SNAPSHOT,
  type AccountDraft,
  type AccountViewModel,
  type FeedbackState,
  type PendingSubscriptionMutation,
  type PlanDraft,
  type PlanViewModel,
  buildPlanPayload,
  buildSubscriptionSnapshot,
  buildSubscriptionSummary,
  createEmptyPlanDraft,
  getSelectablePlans,
} from "./subscription-admin-model";

const EMPTY_ACCOUNTS: SubscriptionAccount[] = [];
const EMPTY_PLANS: SubscriptionPlan[] = [];

interface MutationOptions {
  failure: FeedbackState;
  onSuccess?: () => void;
  pending: PendingSubscriptionMutation;
  request: () => Promise<SubscriptionOverview>;
  success: FeedbackState;
}

function feedbackFromError(
  error: unknown,
  fallback: FeedbackState,
): FeedbackState {
  return {
    ...fallback,
    message: error instanceof Error ? error.message : fallback.message,
  };
}

export function useSubscriptionAdmin() {
  const { t } = useI18n();
  const [snapshot, setSnapshot] = useState(EMPTY_SUBSCRIPTION_SNAPSHOT);
  const [newPlanDraft, setNewPlanDraft] = useState(createEmptyPlanDraft);
  const [loading, setLoading] = useState(true);
  const [pendingMutation, setPendingMutation] =
    useState<PendingSubscriptionMutation | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  const commitOverview = useCallback((overview: SubscriptionOverview) => {
    setSnapshot(buildSubscriptionSnapshot(overview));
  }, []);

  const loadOverview = useCallback(async () => {
    setLoading(true);
    try {
      commitOverview(await getSubscriptionOverviewApi());
      setFeedback((current) => current?.tone === "error" ? null : current);
    } catch (error) {
      setFeedback(feedbackFromError(error, {
        tone: "error",
        title: t("settings.subscription.load_failed_title"),
        message: t("settings.subscription.load_failed_message"),
      }));
    } finally {
      setLoading(false);
    }
  }, [commitOverview, t]);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview]);

  const runMutation = useCallback(async ({
    failure,
    onSuccess,
    pending,
    request,
    success,
  }: MutationOptions) => {
    if (pendingMutation) {
      return;
    }
    setPendingMutation(pending);
    try {
      commitOverview(await request());
      onSuccess?.();
      setFeedback(success);
    } catch (error) {
      setFeedback(feedbackFromError(error, failure));
    } finally {
      setPendingMutation(null);
    }
  }, [commitOverview, pendingMutation]);

  const changeAccountDraft = useCallback((
    ownerUserId: string,
    patch: Partial<AccountDraft>,
  ) => {
    setSnapshot((current) => {
      const draft = current.accountDrafts[ownerUserId];
      if (!draft) {
        return current;
      }
      return {
        ...current,
        accountDrafts: {
          ...current.accountDrafts,
          [ownerUserId]: { ...draft, ...patch },
        },
      };
    });
  }, []);

  const changePlanDraft = useCallback((
    planKey: string,
    patch: Partial<PlanDraft>,
  ) => {
    setSnapshot((current) => {
      const draft = current.planDrafts[planKey];
      if (!draft) {
        return current;
      }
      return {
        ...current,
        planDrafts: {
          ...current.planDrafts,
          [planKey]: { ...draft, ...patch },
        },
      };
    });
  }, []);

  const changeNewPlanDraft = useCallback((patch: Partial<PlanDraft>) => {
    setNewPlanDraft((current) => ({ ...current, ...patch }));
  }, []);

  const saveAccount = useCallback(async (ownerUserId: string) => {
    const draft = snapshot.accountDrafts[ownerUserId];
    if (!draft) {
      return;
    }
    await runMutation({
      pending: { kind: "account", ownerUserId },
      request: () => updateUserSubscriptionApi(ownerUserId, {
        plan_key: draft.planKey,
      }),
      success: {
        tone: "success",
        title: t("settings.subscription.save_success_title"),
        message: t("settings.subscription.save_success_message"),
      },
      failure: {
        tone: "error",
        title: t("settings.subscription.save_failed_title"),
        message: t("settings.subscription.save_failed_message"),
      },
    });
  }, [runMutation, snapshot.accountDrafts, t]);

  const savePlan = useCallback(async (planKey: string) => {
    const draft = snapshot.planDrafts[planKey];
    if (!draft) {
      return;
    }
    const payload = buildPlanPayload(planKey, draft);
    if (!payload) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.plan_save_failed_title"),
        message: t("settings.subscription.plan_limit_invalid"),
      });
      return;
    }
    await runMutation({
      pending: { kind: "plan", planKey },
      request: () => updateSubscriptionPlanApi(planKey, payload),
      success: {
        tone: "success",
        title: t("settings.subscription.plan_save_success_title"),
        message: t("settings.subscription.plan_save_success_message"),
      },
      failure: {
        tone: "error",
        title: t("settings.subscription.plan_save_failed_title"),
        message: t("settings.subscription.plan_save_failed_message"),
      },
    });
  }, [runMutation, snapshot.planDrafts, t]);

  const createPlan = useCallback(async () => {
    const payload = buildPlanPayload(newPlanDraft.planKey, newPlanDraft);
    if (!payload) {
      setFeedback({
        tone: "error",
        title: t("settings.subscription.plan_create_failed_title"),
        message: t("settings.subscription.plan_limit_invalid"),
      });
      return;
    }
    await runMutation({
      pending: { kind: "create-plan" },
      request: () => createSubscriptionPlanApi(payload),
      onSuccess: () => setNewPlanDraft(createEmptyPlanDraft()),
      success: {
        tone: "success",
        title: t("settings.subscription.plan_create_success_title"),
        message: t("settings.subscription.plan_create_success_message"),
      },
      failure: {
        tone: "error",
        title: t("settings.subscription.plan_create_failed_title"),
        message: t("settings.subscription.plan_create_failed_message"),
      },
    });
  }, [newPlanDraft, runMutation, t]);

  const refreshOverview = useCallback(async () => {
    if (loading || pendingMutation) {
      return;
    }
    await loadOverview();
  }, [loadOverview, loading, pendingMutation]);

  const accounts = snapshot.overview?.accounts ?? EMPTY_ACCOUNTS;
  const plans = snapshot.overview?.plans ?? EMPTY_PLANS;
  const summary = useMemo(
    () => buildSubscriptionSummary(accounts, plans),
    [accounts, plans],
  );
  const selectablePlans = useMemo(() => getSelectablePlans(plans), [plans]);
  const mutationPending = pendingMutation !== null;

  const accountView: AccountViewModel = {
    accounts,
    drafts: snapshot.accountDrafts,
    loading,
    mutationPending,
    periodEnd: snapshot.overview?.period_end ?? "",
    periodStart: snapshot.overview?.period_start ?? "",
    plans: selectablePlans,
    savingOwnerUserId:
      pendingMutation?.kind === "account"
        ? pendingMutation.ownerUserId
        : null,
    summary,
  };
  const planView: PlanViewModel = {
    creating: pendingMutation?.kind === "create-plan",
    drafts: snapshot.planDrafts,
    loading,
    mutationPending,
    newPlanDraft,
    plans,
    savingPlanKey:
      pendingMutation?.kind === "plan" ? pendingMutation.planKey : null,
  };

  return {
    accountView,
    planView,
    feedback,
    changeAccountDraft,
    changeNewPlanDraft,
    changePlanDraft,
    createPlan,
    dismissFeedback: () => setFeedback(null),
    refreshOverview,
    saveAccount,
    savePlan,
  };
}
