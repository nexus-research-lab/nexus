"use client";

import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
} from "@/lib/avatar";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import {
  IconPickerPopover,
  IconPickerTriggerLabel,
} from "@/shared/ui/icon-picker/icon-picker-popover";
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
          <IconPickerTriggerLabel isOpen={isOpen}>
            {t("agent_options.identity.change_avatar")}
          </IconPickerTriggerLabel>
        </>
      )}
      startIconId={AGENT_ICON_ID_START}
      triggerAlign={variant === "inline" ? "start" : "center"}
      value={avatar}
    />
  );
}
