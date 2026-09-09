/**
 * INPUT: 当前用户身份展示模型与头像修改状态。
 * OUTPUT: 居中的个人身份与次要账号信息，取消重复摘要卡。
 * POS: Personal 设置身份视图；不读取 API，也不拥有基础排版与 Badge 样式。
 */


import type { PersonalProfile } from "@/lib/api/account/auth-api";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import {
  buildPersonalProfilePresentation,
} from "./personal-settings-model";
import { PersonalAvatarPicker } from "./personal-avatar-picker";

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
    <section className="flex flex-col items-center gap-3 py-5 text-center">
      <PersonalAvatarPicker
        avatar={avatar}
        disabled={!canUpdateAvatar}
        isSaving={isSavingAvatar}
        name={presentation.avatarName}
        onChange={onAvatarChange}
      />
      <div className="flex max-w-full flex-col items-center gap-2">
        <h2 className={cn("max-w-full break-words", getUiTypographyClassName({ role: "objectTitle", tone: "strong" }))}>
          {presentation.displayName}
        </h2>
        <div className="flex max-w-full flex-wrap items-center justify-center gap-2">
          {presentation.username !== presentation.displayName ? <span className="ui-type-supporting break-all text-(--text-muted)">@{presentation.username}</span> : null}
          <UiTooltip label={`${t("settings.personal.role")} · ${presentation.roleLabel} / ${t("settings.personal.auth_method")} · ${presentation.authMethodLabel}`}>
            <UiButton variant="ghost" className="h-auto min-h-0 p-0"><UiBadge size="sm" tone="default">{presentation.subscriptionPlanName ?? presentation.roleLabel}</UiBadge></UiButton>
          </UiTooltip>
        </div>
      </div>
      {!presentation.canUpdateProfile ? (
        <p className="ui-type-metadata text-(--text-muted)">{t("settings.personal.avatar_disabled")}</p>
      ) : null}
    </section>
  );
}
