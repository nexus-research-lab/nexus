import { Image, Loader2 } from "lucide-react";

import type { PersonalProfile } from "@/lib/api/account/auth-api";
import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
} from "@/lib/avatar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";

import {
  getAuthMethodLabelKey,
  getRoleLabelKey,
} from "./personal-settings-model";

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
  const role = profile?.user.role ?? "";
  const roleLabelKey = getRoleLabelKey(role);
  const roleLabel = roleLabelKey ? t(roleLabelKey) : role || "--";
  const authMethodLabel = t(getAuthMethodLabelKey(profile?.user.auth_method ?? ""));

  return (
    <section className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
      <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,300px)] lg:items-start">
        <div className="min-w-0 space-y-3">
          <div className="flex min-w-0 items-center gap-3">
            <UiAgentAvatar
              avatar={avatar}
              className="h-12 w-12 rounded-[16px]"
              name={profile?.user.display_name || profile?.user.username || t("settings.personal.avatar_alt")}
              shape="rounded"
            />
            <div className="min-w-0">
              <h3 className="truncate text-[15px] font-semibold tracking-tight text-(--text-strong)">
                {profile?.user.display_name || profile?.user.username || "--"}
              </h3>
              <p className="mt-1 flex min-w-0 items-center gap-1.5 text-[12px] leading-5 text-(--text-soft)">
                <span className="min-w-0 truncate">{profile?.user.username || "--"}</span>
                {profile?.subscription ? (
                  <>
                    <span className="shrink-0 text-(--text-muted)">·</span>
                    <span className="shrink-0 rounded-full border border-(--divider-subtle-color) px-2 py-0.5 text-[11px] font-semibold text-(--text-muted)">
                      {profile.subscription.plan_name}
                    </span>
                  </>
                ) : null}
              </p>
            </div>
          </div>
          <div className="grid gap-2 text-[11px] text-(--text-soft) sm:grid-cols-2">
            <span className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-2">
              {t("settings.personal.role")}: {roleLabel}
            </span>
            <span className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-2">
              {t("settings.personal.auth_method")}: {authMethodLabel}
            </span>
          </div>
        </div>

        <div className="min-w-0 lg:min-w-[300px]">
          <div className="mb-2 flex items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2 text-[11px] font-semibold text-(--text-muted)">
              <Image className="h-3.5 w-3.5" />
              <span>{t("settings.personal.avatar_title")}</span>
            </div>
            {isSavingAvatar ? (
              <span className="inline-flex items-center gap-1 text-[11px] text-(--text-soft)">
                <Loader2 className="h-3 w-3 animate-spin" />
                {t("common.saving")}
              </span>
            ) : null}
          </div>
          <IconPicker
            className="min-w-0"
            columns={8}
            disabled={!canUpdateAvatar}
            iconSize="sm"
            layout="row"
            maxIcons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
            onSelect={onAvatarChange}
            showClear
            startIconId={AGENT_ICON_ID_START}
            value={avatar}
          />
          {!profile?.can_update_profile ? (
            <p className="mt-2 text-[11px] text-(--text-soft)">
              {t("settings.personal.avatar_disabled")}
            </p>
          ) : null}
        </div>
      </div>
    </section>
  );
}
