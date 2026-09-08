/**
 * INPUT: 当前用户资料、密码与用量资源。
 * OUTPUT: 个人设置分区及反馈，移动端页面身份由应用栏承载。
 * POS: 设置目录的个人账户页面装配层。
 */
"use client";

import { Loader2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";

import { PersonalPasswordSection } from "./personal-password-section";
import { PersonalProfileSection } from "./personal-profile-section";
import { PersonalTokenUsageSection } from "./personal-token-usage-section";
import { usePersonalSettingsController } from "./use-personal-settings-controller";
import { SETTINGS_CARD_CLASS_NAME } from "../shared/settings-panel-ui";

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
          {controller.profile.isLoading && !controller.profile.value ? (
            <section className={cn(
              SETTINGS_CARD_CLASS_NAME,
              "flex min-h-[220px] items-center justify-center text-(--text-soft)",
            )}>
              <Loader2
                className={getUiSpinnerClassName({ size: "lg", tone: "muted" })}
              />
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
                mutationBlocked={controller.password.mutationBlocked}
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
        item={controller.feedback.value
          ? {
              ...controller.feedback.value,
              onDismiss: controller.feedback.value.tone === "success"
                ? controller.feedback.dismiss
                : undefined,
            }
          : null}
      />
    </>
  );
}
