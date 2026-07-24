"use client";

import { ChevronDown } from "lucide-react";

import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
} from "@/lib/avatar";
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { IconPickerPopover } from "@/shared/ui/icon-picker/icon-picker-popover";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { AgentIdentityVariant } from "./identity-layout";

interface IdentityAvatarPickerProps {
  avatar: string;
  avatarAlt: string;
  name: string;
  onChange: (value: string) => void;
  variant: AgentIdentityVariant;
}

const AVATAR_TRIGGER_CLASS: Record<AgentIdentityVariant, string> = {
  dialog: "h-[72px] w-[72px] rounded-[16px]",
  inline: "h-16 w-16 rounded-[14px]",
};

export function IdentityAvatarPicker({
  avatar,
  avatarAlt,
  name,
  onChange,
  variant,
}: IdentityAvatarPickerProps) {
  const { t } = useI18n();
  return (
    <IconPickerPopover
      ariaLabel={t("agent_options.identity.choose_avatar")}
      iconFamily="agent"
      maxIcons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
      onSelect={onChange}
      renderTrigger={(isOpen) => (
        <>
          <UiAgentAvatar
            avatar={avatar}
            className={cn(
              AVATAR_TRIGGER_CLASS[variant],
              "transition-[border-color,box-shadow,transform] duration-(--motion-duration-fast) group-hover:border-[color:color-mix(in_srgb,var(--primary)_35%,var(--surface-avatar-border))] group-hover:shadow-[0_8px_20px_color-mix(in_srgb,var(--shadow-color)_12%,transparent)]",
            )}
            name={name || avatarAlt}
            size="xl"
          />
          <span className="inline-flex min-h-7 items-center gap-1 rounded-full border border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-2.5 text-compact font-semibold text-(--primary) transition-[background,border-color] group-hover:border-[color:color-mix(in_srgb,var(--primary)_32%,var(--divider-subtle-color))] group-hover:bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)]">
            {t("agent_options.identity.change_avatar")}
            <ChevronDown
              className={cn(
                "h-3 w-3 transition-transform duration-(--motion-duration-fast)",
                isOpen && "rotate-180",
              )}
            />
          </span>
        </>
      )}
      startIconId={AGENT_ICON_ID_START}
      triggerClassName="group relative flex shrink-0 flex-col items-center gap-1.5 rounded-[18px] text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_30%,transparent)] focus-visible:ring-offset-2 focus-visible:ring-offset-(--background)"
      value={avatar}
    />
  );
}
