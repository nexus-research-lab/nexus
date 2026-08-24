/**
 * INPUT: 当前用户资料、密码与用量资源。
 * OUTPUT: 个人设置分区及反馈，移动端页面身份由应用栏承载。
 * POS: 设置目录的个人账户页面装配层。
 */
"use client";

import { Loader2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";

import { PersonalPasswordSection } from "./personal-password-section";
import { PersonalProfileSection } from "./personal-profile-section";
import { PersonalTokenUsageSection } from "./personal-token-usage-section";
import { usePersonalSettingsController } from "./use-personal-settings-controller";

export function PersonalSettingsPanel() {
  const { t } = useI18n();
  const controller = usePersonalSettingsController();

  return (
    <>
      <div className={cn(
        WORKSPACE_CONTENT_PAGE_CLASS_NAME,
        "flex flex-col",
      )}>
        <WorkspaceContentHeader
          className="max-sm:hidden"
          description={t("settings.personal.section_description")}
          title={t("settings.personal.section_title")}
        />
        <div className="flex flex-col gap-3">
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
