"use client";

import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";

import { SubscriptionAccountView } from "./subscription-account-view";
import type { SubscriptionAdminView } from "./subscription-admin-model";
import { SubscriptionPlanView } from "./subscription-plan-view";
import { useSubscriptionAdmin } from "./use-subscription-admin";

interface SubscriptionAdminPanelProps {
  view: SubscriptionAdminView;
}

export function SubscriptionAdminPanel({ view }: SubscriptionAdminPanelProps) {
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
        item={controller.feedback ? {
          message: controller.feedback.message,
          onDismiss: controller.dismissFeedback,
          title: controller.feedback.title,
          tone: controller.feedback.tone,
        } : null}
      />
    </>
  );
}
