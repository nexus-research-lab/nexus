import { UiInput } from "@/shared/ui/form/form-control";
import type { AgentNameValidationResult } from "@/types/agent/agent";

import { IdentityAvatarPicker } from "./identity-avatar-picker";
import type { AgentIdentityVariant } from "./identity-layout";

interface IdentityProfileLayout {
  inputClassName: string;
  rowClassName: string;
}

const PROFILE_LAYOUTS: Record<AgentIdentityVariant, IdentityProfileLayout> = {
  dialog: {
    inputClassName: "h-11 radius-control-md",
    rowClassName: "flex items-start gap-4",
  },
  inline: {
    inputClassName: "h-10 radius-control-md",
    rowClassName: "flex items-start gap-3",
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
        <IdentityAvatarPicker
          avatar={avatar}
          avatarAlt={avatarAlt}
          name={title || avatarAlt}
          onChange={onAvatarChange}
          variant={variant}
        />
        <div className="min-w-0 flex-1 space-y-1.5 pt-1">
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

      {validationFeedback ? (
        <div className="text-xs">
          <span className={VALIDATION_FEEDBACK_CLASS[validationFeedback.tone]}>
            {validationFeedback.message}
          </span>
        </div>
      ) : null}
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
