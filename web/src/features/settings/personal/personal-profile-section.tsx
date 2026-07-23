import { Info, KeyRound, ShieldCheck } from "lucide-react";

import type { PersonalProfile } from "@/lib/api/account/auth-api";
import { useI18n } from "@/shared/i18n/i18n-context";

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
    <section className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
      <div className="grid gap-4 px-4 py-4 sm:grid-cols-[auto_minmax(0,1fr)] sm:items-center sm:px-5 lg:grid-cols-[auto_minmax(180px,1fr)_minmax(260px,auto)] lg:gap-6">
        <PersonalAvatarPicker
          avatar={avatar}
          disabled={!canUpdateAvatar}
          isSaving={isSavingAvatar}
          name={presentation.avatarName}
          onChange={onAvatarChange}
        />

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <h3 className="truncate text-[17px] font-semibold tracking-tight text-(--text-strong)">
              {presentation.displayName}
            </h3>
            {presentation.subscriptionPlanName !== null ? (
              <span className="shrink-0 rounded-full border border-[color:color-mix(in_srgb,var(--primary)_16%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)] px-2 py-0.5 text-[10.5px] font-semibold text-(--primary)">
                {presentation.subscriptionPlanName}
              </span>
            ) : null}
          </div>
          <p className="mt-0.5 truncate text-[12px] leading-5 text-(--text-soft)">
            {presentation.username}
          </p>
        </div>

        <div className="min-w-0 sm:col-start-2 lg:col-auto">
          <div className="flex flex-wrap gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_72%,transparent)] px-2.5 py-1 text-[11px] text-(--text-soft)">
              <ShieldCheck className="h-3.5 w-3.5 text-(--icon-muted)" />
              {t("settings.personal.role")}: {presentation.roleLabel}
            </span>
            <span className="inline-flex items-center gap-1.5 rounded-full border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_72%,transparent)] px-2.5 py-1 text-[11px] text-(--text-soft)">
              <KeyRound className="h-3.5 w-3.5 text-(--icon-muted)" />
              {t("settings.personal.auth_method")}: {presentation.authMethodLabel}
            </span>
          </div>

          {!presentation.canUpdateProfile ? (
            <p className="mt-2 flex items-start gap-1.5 text-[11px] leading-5 text-(--text-soft)">
              <Info className="mt-[3px] h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
              <span>{t("settings.personal.avatar_disabled")}</span>
            </p>
          ) : null}
        </div>
      </div>
    </section>
  );
}
