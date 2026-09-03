/**
 * INPUT: 当前头像、保存状态与可修改权限。
 * OUTPUT: 带共享图标目录的头像触发器及其状态标签。
 * POS: Personal 设置头像交互 Pattern；不拥有头像、Popover 或排版基础样式。
 */
"use client";

import { ChevronDown, Loader2, LockKeyhole } from "lucide-react";

import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
} from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { IconPickerPopover } from "@/shared/ui/icon-picker/icon-picker-popover";
import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface PersonalAvatarPickerProps {
  avatar: string;
  disabled: boolean;
  isSaving: boolean;
  name: string;
  onChange: (avatar: string) => void;
}

export function PersonalAvatarPicker({
  avatar,
  disabled,
  isSaving,
  name,
  onChange,
}: PersonalAvatarPickerProps) {
  const { t } = useI18n();

  return (
    <IconPickerPopover
      ariaLabel={t("settings.personal.choose_avatar")}
      disabled={disabled}
      iconFamily="agent"
      maxIcons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
      onSelect={onChange}
      renderTrigger={(isOpen) => (
        <>
          <UiAgentAvatar
            avatar={avatar}
            className={cn(
              "h-[72px] w-[72px] surface-radius-lg transition-colors duration-(--motion-duration-fast)",
              !disabled && "group-hover:border-[color:color-mix(in_srgb,var(--primary)_35%,var(--surface-avatar-border))]",
            )}
            name={name}
            size="xl"
          />
          <span
            className={cn(
              "inline-flex min-h-7 items-center gap-1 rounded-full border px-2.5 transition-[background,border-color]",
              getUiTypographyClassName({ role: "metadata", weight: "semibold" }),
              disabled
                ? "border-(--divider-subtle-color) bg-transparent text-(--text-soft)"
                : "border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] text-(--primary) group-hover:border-[color:color-mix(in_srgb,var(--primary)_32%,var(--divider-subtle-color))] group-hover:bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)]",
            )}
          >
            {isSaving ? (
              <>
                <Loader2 className={getUiSpinnerClassName({ size: "xs" })} />
                {t("common.saving")}
              </>
            ) : disabled ? (
              <>
                <LockKeyhole className="h-3 w-3" />
                {t("settings.personal.avatar_locked")}
              </>
            ) : (
              <>
                {t("settings.personal.change_avatar")}
                <ChevronDown
                  className={cn(
                    "h-3 w-3 transition-transform duration-(--motion-duration-fast)",
                    isOpen && "rotate-180",
                  )}
                />
              </>
            )}
          </span>
        </>
      )}
      startIconId={AGENT_ICON_ID_START}
      triggerClassName="group relative flex shrink-0 flex-col items-center gap-1.5 surface-radius-lg text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_30%,transparent)] focus-visible:ring-offset-2 focus-visible:ring-offset-(--background) disabled:cursor-not-allowed"
      value={avatar}
    />
  );
}
