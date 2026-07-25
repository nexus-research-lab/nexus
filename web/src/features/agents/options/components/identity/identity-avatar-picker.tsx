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

const AVATAR_TRIGGER_SIZE = {
  dialog: "lg",
  inline: "lg",
} as const satisfies Record<AgentIdentityVariant, "lg" | "xl">;

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
            className="transition-[border-color] duration-(--motion-duration-fast) group-hover:border-(--surface-interactive-active-border)"
            name={name || avatarAlt}
            size={AVATAR_TRIGGER_SIZE[variant]}
          />
          <span className="inline-flex h-7 items-center gap-1 radius-control-sm px-2 text-compact font-medium text-(--text-muted) transition-[background,color] duration-(--motion-duration-fast) group-hover:bg-(--surface-interactive-hover-background) group-hover:text-(--text-strong)">
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
      triggerClassName="group relative flex shrink-0 flex-col items-center gap-1.5 rounded-[12px] text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_30%,transparent)] focus-visible:ring-offset-2 focus-visible:ring-offset-(--background)"
      value={avatar}
    />
  );
}
