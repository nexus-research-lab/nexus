import { useCallback, useEffect, useMemo, useState } from "react";

import {
  createSubscriptionPlanApi,
  getSubscriptionOverviewApi,
  updateSubscriptionPlanApi,
  updateUserSubscriptionApi,
} from "@/lib/api/account/subscription-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  EMPTY_SUBSCRIPTION_SNAPSHOT,
  type AccountDraft,
  type FeedbackState,
  type PendingSubscriptionMutation,
  type PlanDraft,
  buildPlanPayload,
  buildSubscriptionAdminViewModels,
  buildSubscriptionFeedback,
  buildSubscriptionSnapshot,
  createEmptyPlanDraft,
} from "./subscription-admin-model";
import { SubscriptionTransactionCoordinator } from "./subscription-transaction";

export function useSubscriptionAdmin() {
  const { t } = useI18n();
  const [snapshot, setSnapshot] = useState(EMPTY_SUBSCRIPTION_SNAPSHOT);
  const [newPlanDraft, setNewPlanDraft] = useState(createEmptyPlanDraft);
  const [loading, setLoading] = useState(true);
  const [pendingMutation, setPendingMutation] =
    useState<PendingSubscriptionMutation | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);
  const [transactionCoordinator] = useState(() => (
    new SubscriptionTransactionCoordinator({
      onCommit: (overview) => setSnapshot(buildSubscriptionSnapshot(overview)),
      onFeedback: setFeedback,
      onPendingMutation: setPendingMutation,
    })
  ));

  const loadOverview = useCallback(async () => {
    await transactionCoordinator.load({
      failure: buildSubscriptionFeedback(t, "load-failed"),
      onFinish: () => setLoading(false),
      onStart: () => setLoading(true),
      onSuccess: () => setFeedback((current) => (
        current?.tone === "error" ? null : current
      )),
      request: getSubscriptionOverviewApi,
    });
  }, [t, transactionCoordinator]);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview]);

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
    await transactionCoordinator.runMutation({
      pending: { kind: "account", ownerUserId },
      request: () => updateUserSubscriptionApi(ownerUserId, {
        plan_key: draft.planKey,
      }),
      success: buildSubscriptionFeedback(t, "account-save-succeeded"),
      failure: buildSubscriptionFeedback(t, "account-save-failed"),
    });
  }, [snapshot.accountDrafts, t, transactionCoordinator]);

  const savePlan = useCallback(async (planKey: string) => {
    const draft = snapshot.planDrafts[planKey];
    if (!draft) {
      return;
    }
    const payload = buildPlanPayload(planKey, draft);
    if (!payload) {
      setFeedback(buildSubscriptionFeedback(t, "plan-save-invalid"));
      return;
    }
    await transactionCoordinator.runMutation({
      pending: { kind: "plan", planKey },
      request: () => updateSubscriptionPlanApi(planKey, payload),
      success: buildSubscriptionFeedback(t, "plan-save-succeeded"),
      failure: buildSubscriptionFeedback(t, "plan-save-failed"),
    });
  }, [snapshot.planDrafts, t, transactionCoordinator]);

  const createPlan = useCallback(async () => {
    const payload = buildPlanPayload(newPlanDraft.planKey, newPlanDraft);
    if (!payload) {
      setFeedback(buildSubscriptionFeedback(t, "plan-create-invalid"));
      return;
    }
    await transactionCoordinator.runMutation({
      pending: { kind: "create-plan" },
      request: () => createSubscriptionPlanApi(payload),
      onSuccess: () => setNewPlanDraft(createEmptyPlanDraft()),
      success: buildSubscriptionFeedback(t, "plan-create-succeeded"),
      failure: buildSubscriptionFeedback(t, "plan-create-failed"),
    });
  }, [newPlanDraft, t, transactionCoordinator]);

  const refreshOverview = useCallback(async () => {
    await loadOverview();
  }, [loadOverview]);

  const { accountView, planView } = useMemo(
    () => buildSubscriptionAdminViewModels(
      snapshot,
      newPlanDraft,
      loading,
      pendingMutation,
    ),
    [loading, newPlanDraft, pendingMutation, snapshot],
  );

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
