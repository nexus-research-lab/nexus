/**
 * INPUT: 当前头像、保存状态与可修改权限。
 * OUTPUT: 公共 64px 头像与中性选择触发器，保留既有保存/锁定标签。
 * POS: Personal 设置头像交互 Pattern；不拥有头像、Popover 或排版基础样式。
 */
"use client";

import { Loader2, LockKeyhole } from "lucide-react";
import { cn } from "@/shared/ui/class-name";

import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
} from "@/lib/avatar";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  IconPickerPopover,
  IconPickerTriggerLabel,
} from "@/shared/ui/icon-picker/icon-picker-popover";
import { useI18n } from "@/shared/i18n/i18n-context";

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
            aria-hidden="true"
            avatar={avatar}
            className={cn(
              "rounded-full transition-colors duration-(--motion-duration-fast)",
              !disabled && "group-hover:border-[color:color-mix(in_srgb,var(--primary)_35%,var(--surface-avatar-border))]",
            )}
            name={name}
            size="xl"
          />
          <IconPickerTriggerLabel
            isOpen={isOpen}
            showChevron={!isSaving && !disabled}
          >
            {isSaving ? (
              <>
                <Loader2 aria-hidden="true" className={getUiSpinnerClassName({ size: "xs" })} />
                {t("common.saving")}
              </>
            ) : disabled ? (
              <>
                <LockKeyhole aria-hidden="true" className="h-3 w-3" />
                {t("settings.personal.avatar_locked")}
              </>
            ) : (
              t("settings.personal.change_avatar")
            )}
          </IconPickerTriggerLabel>
        </>
      )}
      startIconId={AGENT_ICON_ID_START}
      triggerRadius="surface"
      value={avatar}
    />
  );
}
