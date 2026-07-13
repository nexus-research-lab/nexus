import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
} from "@/lib/avatar";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiInput } from "@/shared/ui/form/form-control";
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

type NameValidationFeedbackTone = "danger" | "muted" | "success";

interface NameValidationFeedback {
  message: string;
  tone: NameValidationFeedbackTone;
}

type NameValidationFeedbackContext = Pick<
  IdentityProfileFieldsProps,
  "isValidatingName" | "nameAvailable" | "nameValidation" | "validatingLabel"
>;

type NameValidationFeedbackRule = (
  context: NameValidationFeedbackContext,
) => NameValidationFeedback | null;

const VALIDATION_FEEDBACK_CLASS: Record<
  NameValidationFeedbackTone,
  string
> = {
  danger: "text-(--destructive)",
  muted: "text-muted-foreground",
  success: "text-(--success)",
};

const NAME_VALIDATION_FEEDBACK_RULES: NameValidationFeedbackRule[] = [
  createValidatingFeedback,
  createRejectedNameFeedback,
  createAvailableNameFeedback,
];

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
  const validationFeedback = resolveValidationFeedback({
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
            data-autofocus="true"
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

      <div className="min-h-5 text-xs">
        {validationFeedback ? (
          <span className={VALIDATION_FEEDBACK_CLASS[validationFeedback.tone]}>
            {validationFeedback.message}
          </span>
        ) : null}
      </div>
    </>
  );
}

function resolveValidationFeedback(
  context: NameValidationFeedbackContext,
): NameValidationFeedback | null {
  for (const rule of NAME_VALIDATION_FEEDBACK_RULES) {
    const feedback = rule(context);
    if (feedback) {
      return feedback;
    }
  }
  return null;
}

function createValidatingFeedback(
  context: NameValidationFeedbackContext,
): NameValidationFeedback | null {
  return context.isValidatingName
    ? { message: context.validatingLabel, tone: "muted" }
    : null;
}

function createRejectedNameFeedback(
  context: NameValidationFeedbackContext,
): NameValidationFeedback | null {
  const reason = context.nameValidation?.reason;
  return reason ? { message: reason, tone: "danger" } : null;
}

function createAvailableNameFeedback(
  context: NameValidationFeedbackContext,
): NameValidationFeedback | null {
  const validation = context.nameValidation;
  if (!validation?.is_valid || !validation.is_available) {
    return null;
  }
  return {
    message: context.nameAvailable(validation.workspace_path ?? ""),
    tone: "success",
  };
}
