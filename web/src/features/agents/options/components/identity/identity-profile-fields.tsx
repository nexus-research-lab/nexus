import type { ReactNode } from "react";

import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
} from "@/lib/utils";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { UiInput } from "@/shared/ui/form-control";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import type { AgentNameValidationResult } from "@/types/agent/agent";

import type { AgentIdentityVariant } from "./identity-layout";

interface IdentityProfileLayout {
  avatarClassName: string;
  avatarSize: "lg" | "md";
  iconSize: "md" | "sm";
  inputClassName: string;
  rowClassName: string;
}

const PROFILE_LAYOUTS: Record<AgentIdentityVariant, IdentityProfileLayout> = {
  dialog: {
    avatarClassName: "h-14 w-14 rounded-[14px]",
    avatarSize: "lg",
    iconSize: "md",
    inputClassName: "h-10 rounded-xl",
    rowClassName: "flex items-end gap-3",
  },
  inline: {
    avatarClassName: "h-13 w-13 rounded-[12px]",
    avatarSize: "md",
    iconSize: "sm",
    inputClassName: "rounded-xl",
    rowClassName: "flex items-end gap-2.5",
  },
};

interface IdentityProfileFieldsProps {
  avatar: string;
  avatarAlt: string;
  isValidatingName: boolean;
  nameAvailable: (path: string) => string;
  nameLabel: string;
  namePlaceholder: string;
  nameValidation: AgentNameValidationResult | null;
  onAvatarChange: (value: string) => void;
  onTitleChange: (value: string) => void;
  title: string;
  validatingLabel: string;
  variant: AgentIdentityVariant;
}

export function IdentityProfileFields({
  avatar,
  avatarAlt,
  isValidatingName,
  nameAvailable,
  nameLabel,
  namePlaceholder,
  nameValidation,
  onAvatarChange,
  onTitleChange,
  title,
  validatingLabel,
  variant,
}: IdentityProfileFieldsProps) {
  const layout = PROFILE_LAYOUTS[variant];
  const validationMessage = resolveValidationMessage({
    isValidatingName,
    nameAvailable,
    nameValidation,
    validatingLabel,
  });

  return (
    <>
      <div className={layout.rowClassName}>
        <UiAgentAvatar
          avatar={avatar}
          className={layout.avatarClassName}
          name={title || avatarAlt}
          shape="rounded"
          size={layout.avatarSize}
        />
        <div className="min-w-0 flex-1 space-y-1.5">
          <label className="text-[11px] font-semibold uppercase tracking-[0.12em] text-(--text-soft)">
            {nameLabel} <span className="text-(--destructive)">*</span>
          </label>
          <UiInput
            className={layout.inputClassName}
            controlSize="md"
            onChange={(event) => onTitleChange(event.target.value)}
            placeholder={namePlaceholder}
            type="text"
            value={title}
          />
        </div>
      </div>

      <IconPicker
        columns={6}
        iconSize={layout.iconSize}
        layout="row"
        maxIcons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
        onSelect={onAvatarChange}
        showClear={false}
        startIconId={AGENT_ICON_ID_START}
        value={avatar}
      />

      <div className="min-h-5 text-xs">{validationMessage}</div>
    </>
  );
}

function resolveValidationMessage({
  isValidatingName,
  nameAvailable,
  nameValidation,
  validatingLabel,
}: Pick<
  IdentityProfileFieldsProps,
  "isValidatingName" | "nameAvailable" | "nameValidation" | "validatingLabel"
>): ReactNode {
  const candidates = [
    {
      active: isValidatingName,
      content: <span className="text-muted-foreground">{validatingLabel}</span>,
    },
    {
      active: !isValidatingName && Boolean(nameValidation?.reason),
      content: <span className="text-(--destructive)">{nameValidation?.reason}</span>,
    },
    {
      active: !isValidatingName
        && Boolean(nameValidation?.is_valid)
        && Boolean(nameValidation?.is_available),
      content: (
        <span className="text-(--success)">
          {nameAvailable(nameValidation?.workspace_path ?? "")}
        </span>
      ),
    },
  ];
  return candidates.find((candidate) => candidate.active)?.content ?? null;
}
