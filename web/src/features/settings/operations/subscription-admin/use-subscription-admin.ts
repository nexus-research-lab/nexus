// INPUT: Subscription API、当前 overview 草稿与 FailureCore。
// OUTPUT: 保留最后快照、阻止未知结果重复写入的运营控制器。
// POS: Subscription Admin 业务编排边界；反馈与 mutation 锁分离，只有权威 overview 才解锁。
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
  buildSubscriptionMutationFailure,
  buildSubscriptionReadFailure,
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
  const [unresolvedMutation, setUnresolvedMutation] =
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
      failure: (error) => buildSubscriptionReadFailure(t, error),
      onFinish: () => setLoading(false),
      onStart: () => setLoading(true),
      onSuccess: () => {
        setUnresolvedMutation(null);
        setFeedback(null);
      },
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
    if (unresolvedMutation) {
      return;
    }
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
      onSuccess: () => setUnresolvedMutation(null),
      failure: (error) => {
        const projection = buildSubscriptionMutationFailure(
          t,
          "account-save",
          error,
        );
        setUnresolvedMutation(
          projection.effect === "not_applied"
            ? null
            : { kind: "account", ownerUserId },
        );
        return projection.feedback;
      },
    });
  }, [snapshot.accountDrafts, t, transactionCoordinator, unresolvedMutation]);

  const savePlan = useCallback(async (planKey: string) => {
    if (unresolvedMutation) {
      return;
    }
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
      onSuccess: () => setUnresolvedMutation(null),
      failure: (error) => {
        const projection = buildSubscriptionMutationFailure(
          t,
          "plan-save",
          error,
        );
        setUnresolvedMutation(
          projection.effect === "not_applied" ? null : { kind: "plan", planKey },
        );
        return projection.feedback;
      },
    });
  }, [snapshot.planDrafts, t, transactionCoordinator, unresolvedMutation]);

  const createPlan = useCallback(async () => {
    if (unresolvedMutation) {
      return;
    }
    const payload = buildPlanPayload(newPlanDraft.planKey, newPlanDraft);
    if (!payload) {
      setFeedback(buildSubscriptionFeedback(t, "plan-create-invalid"));
      return;
    }
    await transactionCoordinator.runMutation({
      pending: { kind: "create-plan" },
      request: () => createSubscriptionPlanApi(payload),
      onSuccess: () => {
        setNewPlanDraft(createEmptyPlanDraft());
        setUnresolvedMutation(null);
      },
      success: buildSubscriptionFeedback(t, "plan-create-succeeded"),
      failure: (error) => {
        const projection = buildSubscriptionMutationFailure(
          t,
          "plan-create",
          error,
        );
        setUnresolvedMutation(
          projection.effect === "not_applied" ? null : { kind: "create-plan" },
        );
        return projection.feedback;
      },
    });
  }, [newPlanDraft, t, transactionCoordinator, unresolvedMutation]);

  const refreshOverview = useCallback(async () => {
    await loadOverview();
  }, [loadOverview]);

  const { accountView, planView } = useMemo(
    () => buildSubscriptionAdminViewModels(
      snapshot,
      newPlanDraft,
      loading,
      pendingMutation,
      Boolean(unresolvedMutation),
    ),
    [loading, newPlanDraft, pendingMutation, snapshot, unresolvedMutation],
  );

  return {
    accountView,
    planView,
    feedback,
    mutationBlocked: Boolean(unresolvedMutation),
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
