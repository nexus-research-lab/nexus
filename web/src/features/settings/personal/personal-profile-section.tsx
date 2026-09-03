/**
 * INPUT: 当前用户身份展示模型与头像修改状态。
 * OUTPUT: 统一的个人身份、套餐、角色和登录方式摘要卡。
 * POS: Personal 设置身份视图；不读取 API，也不拥有基础排版与 Badge 样式。
 */

import { Info, KeyRound, ShieldCheck } from "lucide-react";

import type { PersonalProfile } from "@/lib/api/account/auth-api";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import {
  buildPersonalProfilePresentation,
} from "./personal-settings-model";
import { PersonalAvatarPicker } from "./personal-avatar-picker";
import { SETTINGS_CARD_CLASS_NAME } from "../shared/settings-panel-ui";

interface PersonalProfileSectionProps {
  avatar: string;
  canUpdateAvatar: boolean;
  isSavingAvatar: boolean;
  onAvatarChange: (avatar: string) => void;
  profile: PersonalProfile | null;
}

export function PersonalProfileSection({
  avatar,
  canUpdateAvatar,
  isSavingAvatar,
  onAvatarChange,
  profile,
}: PersonalProfileSectionProps) {
  const { t } = useI18n();
  const presentation = buildPersonalProfilePresentation(profile, t);

  return (
    <section className={cn("w-full", SETTINGS_CARD_CLASS_NAME)}>
      <div className="grid gap-5 px-4 py-5 sm:px-5 lg:grid-cols-[minmax(280px,0.85fr)_minmax(360px,1fr)] lg:items-center lg:gap-6">
        <div className="flex min-w-0 items-center gap-4">
          <PersonalAvatarPicker
            avatar={avatar}
            disabled={!canUpdateAvatar}
            isSaving={isSavingAvatar}
            name={presentation.avatarName}
            onChange={onAvatarChange}
          />

          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <h3 className={cn(
                "truncate",
                getUiTypographyClassName({ role: "pageTitle", tone: "strong" }),
              )}>
                {presentation.displayName}
              </h3>
              {presentation.subscriptionPlanName !== null ? (
                <UiBadge size="sm" tone="primary">
                  {presentation.subscriptionPlanName}
                </UiBadge>
              ) : null}
            </div>
            <p className={cn(
              "mt-0.5 truncate",
              getUiTypographyClassName({ role: "metadata", tone: "soft" }),
            )}>
              {presentation.username}
            </p>
          </div>
        </div>

        <div className="min-w-0">
          <div className="grid gap-2 sm:grid-cols-2">
            <span className="flex min-w-0 items-center gap-2 radius-control-md border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_72%,transparent)] px-3 py-2.5">
              <ShieldCheck className="h-4 w-4 shrink-0 text-(--icon-muted)" />
              <span className="min-w-0">
                <span className={cn(
                  "block",
                  getUiTypographyClassName({ role: "caption", tone: "soft" }),
                )}>
                  {t("settings.personal.role")}
                </span>
                <span className={cn(
                  "block truncate",
                  getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "medium" }),
                )}>
                  {presentation.roleLabel}
                </span>
              </span>
            </span>
            <span className="flex min-w-0 items-center gap-2 radius-control-md border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_72%,transparent)] px-3 py-2.5">
              <KeyRound className="h-4 w-4 shrink-0 text-(--icon-muted)" />
              <span className="min-w-0">
                <span className={cn(
                  "block",
                  getUiTypographyClassName({ role: "caption", tone: "soft" }),
                )}>
                  {t("settings.personal.auth_method")}
                </span>
                <span className={cn(
                  "block truncate",
                  getUiTypographyClassName({ role: "metadata", tone: "strong", weight: "medium" }),
                )}>
                  {presentation.authMethodLabel}
                </span>
              </span>
            </span>
          </div>

          {!presentation.canUpdateProfile ? (
            <p className={cn(
              "mt-2 flex items-start gap-1.5",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}>
              <Info className="mt-[3px] h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
              <span>{t("settings.personal.avatar_disabled")}</span>
            </p>
          ) : null}
        </div>
      </div>
    </section>
  );
}
