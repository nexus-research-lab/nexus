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
    <section className="w-full overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
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
              <h3 className="truncate text-md font-semibold tracking-tight text-(--text-strong)">
                {presentation.displayName}
              </h3>
              {presentation.subscriptionPlanName !== null ? (
                <span className="shrink-0 rounded-full border border-[color:color-mix(in_srgb,var(--primary)_16%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)] px-2 py-0.5 text-xs font-semibold text-(--primary)">
                  {presentation.subscriptionPlanName}
                </span>
              ) : null}
            </div>
            <p className="mt-0.5 truncate text-compact leading-5 text-(--text-soft)">
              {presentation.username}
            </p>
          </div>
        </div>

        <div className="min-w-0">
          <div className="grid gap-2 sm:grid-cols-2">
            <span className="flex min-w-0 items-center gap-2 rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_72%,transparent)] px-3 py-2.5">
              <ShieldCheck className="h-4 w-4 shrink-0 text-(--icon-muted)" />
              <span className="min-w-0">
                <span className="block text-xs leading-4 text-(--text-soft)">
                  {t("settings.personal.role")}
                </span>
                <span className="block truncate text-compact font-medium leading-4 text-(--text-strong)">
                  {presentation.roleLabel}
                </span>
              </span>
            </span>
            <span className="flex min-w-0 items-center gap-2 rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--background)_72%,transparent)] px-3 py-2.5">
              <KeyRound className="h-4 w-4 shrink-0 text-(--icon-muted)" />
              <span className="min-w-0">
                <span className="block text-xs leading-4 text-(--text-soft)">
                  {t("settings.personal.auth_method")}
                </span>
                <span className="block truncate text-compact font-medium leading-4 text-(--text-strong)">
                  {presentation.authMethodLabel}
                </span>
              </span>
            </span>
          </div>

          {!presentation.canUpdateProfile ? (
            <p className="mt-2 flex items-start gap-1.5 text-xs leading-5 text-(--text-soft)">
              <Info className="mt-[3px] h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
              <span>{t("settings.personal.avatar_disabled")}</span>
            </p>
          ) : null}
        </div>
      </div>
    </section>
  );
}
