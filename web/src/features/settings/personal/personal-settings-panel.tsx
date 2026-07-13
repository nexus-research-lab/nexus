"use client";

import { Loader2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";

import { PersonalPasswordSection } from "./personal-password-section";
import { PersonalProfileSection } from "./personal-profile-section";
import { PersonalTokenUsageSection } from "./personal-token-usage-section";
import { usePersonalSettingsController } from "./use-personal-settings-controller";

export function PersonalSettingsPanel() {
  const controller = usePersonalSettingsController();

  return (
    <>
      <div className={cn(
        "mx-auto flex w-full flex-col gap-3 px-1 py-3",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}>
        {controller.profile.isLoading ? (
          <section className="flex min-h-[220px] items-center justify-center rounded-[12px] border border-(--divider-subtle-color) bg-transparent text-(--text-soft)">
            <Loader2 className="h-5 w-5 animate-spin" />
          </section>
        ) : (
          <>
            <PersonalProfileSection
              avatar={controller.avatar.value}
              canUpdateAvatar={controller.avatar.canUpdate}
              isSavingAvatar={controller.avatar.isSaving}
              onAvatarChange={(avatar) => {
                void controller.avatar.save(avatar);
              }}
              profile={controller.profile.value}
            />
            <PersonalTokenUsageSection usage={controller.profile.value?.token_usage} />
            <PersonalPasswordSection
              canChange={controller.password.canChange}
              canSubmit={controller.password.canSubmit}
              draft={controller.password.draft}
              hasInput={controller.password.hasInput}
              isSubmitting={controller.password.isSubmitting}
              onFieldChange={controller.password.setField}
              onSubmit={() => {
                void controller.password.submit();
              }}
              validationError={controller.password.validationError}
            />
          </>
        )}
      </div>

      <FeedbackBannerViewport
        item={controller.feedback.value ? {
          message: controller.feedback.value.message,
          onDismiss: controller.feedback.dismiss,
          title: controller.feedback.value.title,
          tone: controller.feedback.value.tone,
        } : null}
      />
    </>
  );
}
