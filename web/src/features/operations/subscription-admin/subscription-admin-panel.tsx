"use client";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import {
  SETTINGS_ITEM_DESCRIPTION_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
} from "@/features/settings/settings-panel-ui";

import { SubscriptionAccountView } from "./subscription-account-view";
import type { SubscriptionAdminView } from "./subscription-admin-model";
import { SubscriptionPlanView } from "./subscription-plan-view";
import { useSubscriptionAdmin } from "./use-subscription-admin";

interface SubscriptionAdminPanelProps {
  view: SubscriptionAdminView;
}

const VIEW_COPY: Record<
  SubscriptionAdminView,
  { description: TranslationKey; title: TranslationKey }
> = {
  users: {
    title: "settings.subscription.users_title",
    description: "settings.subscription.users_description",
  },
  plans: {
    title: "settings.subscription.plan_management_title",
    description: "settings.subscription.plan_management_description",
  },
};

export function SubscriptionAdminPanel({ view }: SubscriptionAdminPanelProps) {
  const { t } = useI18n();
  const controller = useSubscriptionAdmin();
  const copy = VIEW_COPY[view];

  return (
    <>
      <div className={cn(
        "mx-auto grid w-full gap-4 px-4 py-4 sm:px-6",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}>
        <section className="grid gap-1 px-1">
          <p className={SETTINGS_SECTION_TITLE_CLASS_NAME}>{t(copy.title)}</p>
          <p className={SETTINGS_ITEM_DESCRIPTION_CLASS_NAME}>
            {t(copy.description)}
          </p>
        </section>

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

      <FeedbackBannerStack
        items={controller.feedback ? [{
          key: "subscription-feedback",
          message: controller.feedback.message,
          onDismiss: controller.dismissFeedback,
          title: controller.feedback.title,
          tone: controller.feedback.tone,
        }] : []}
      />
    </>
  );
}
