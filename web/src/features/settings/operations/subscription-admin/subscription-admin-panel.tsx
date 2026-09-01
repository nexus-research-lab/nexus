/**
 * INPUT: Subscription 权威快照、独立 mutation 锁与可见反馈。
 * OUTPUT: dismiss 只影响反馈，不得解除未对账 mutation 锁的运营界面。
 * POS: Subscription Admin 页面装配层。
 */
"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import { completeFeedbackBanner } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";

import { SubscriptionAccountView } from "./subscription-account-view";
import type { SubscriptionAdminView } from "./subscription-admin-model";
import { SubscriptionPlanView } from "./subscription-plan-view";
import { useSubscriptionAdmin } from "./use-subscription-admin";

interface SubscriptionAdminPanelProps {
  view: SubscriptionAdminView;
}

export function SubscriptionAdminPanel({ view }: SubscriptionAdminPanelProps) {
  const { t } = useI18n();
  const controller = useSubscriptionAdmin();

  return (
    <>
      <div className="grid gap-4">
        {view === "users" ? (
          <SubscriptionAccountView
            model={controller.accountView}
            onChangeDraft={controller.changeAccountDraft}
            onRefresh={controller.refreshOverview}
            onSave={controller.saveAccount}
          />
        ) : (
          <SubscriptionPlanView
            model={controller.planView}
            onChangeDraft={controller.changePlanDraft}
            onChangeNewDraft={controller.changeNewPlanDraft}
            onCreate={controller.createPlan}
            onSave={controller.savePlan}
          />
        )}
      </div>

      <FeedbackBannerViewport
        item={controller.feedback
          ? completeFeedbackBanner(
            controller.feedback.tone === "success"
              ? {
                  message: controller.feedback.message,
                  onDismiss: controller.dismissFeedback,
                  title: controller.feedback.title,
                  tone: "success",
                }
              : {
                  impact: controller.feedback.impact,
                  nextStep: controller.feedback.nextStep,
                  action: controller.feedback.recoveryAction === "refresh"
                    ? {
                        label: t("settings.subscription.refresh"),
                        onClick: () => {
                          void controller.refreshOverview();
                        },
                      }
                    : undefined,
                  onDismiss: controller.mutationBlocked
                    ? undefined
                    : controller.dismissFeedback,
                  title: controller.feedback.title,
                  tone: controller.feedback.tone,
                },
            {
              impact: t("feedback.unconfirmed_impact"),
            },
          )
          : null}
      />
    </>
  );
}
