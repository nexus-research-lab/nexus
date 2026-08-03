import { UiInput } from "@/shared/ui/form/form-control";
import type { AgentNameValidationResult } from "@/types/agent/agent";

import { IdentityAvatarPicker } from "./identity-avatar-picker";
import {
  IDENTITY_FIELD_LABEL_CLASS_NAMES,
  type AgentIdentityVariant,
} from "./identity-layout";

interface IdentityProfileLayout {
  inputClassName: string;
  rowClassName: string;
}

const PROFILE_LAYOUTS: Record<AgentIdentityVariant, IdentityProfileLayout> = {
  dialog: {
    inputClassName: "h-10 radius-control-md",
    rowClassName: "flex items-start gap-3",
  },
  inline: {
    inputClassName: "h-9 radius-control-md",
    rowClassName: "flex items-start gap-3",
  },
};

interface IdentityProfileFieldsProps {
  avatar: string;
  avatarAlt: string;
  isValidatingName: boolean;
  nameLabel: string;
  namePlaceholder: string;
  nameValidation: AgentNameValidationResult | null;
  onAvatarChange: (value: string) => void;
  onTitleChange: (value: string) => void;
  title: string;
  validatingLabel: string;
  variant: AgentIdentityVariant;
}

type NameValidationFeedbackTone = "danger" | "muted";

interface NameValidationFeedback {
  message: string;
  tone: NameValidationFeedbackTone;
}

type NameValidationFeedbackContext = Pick<
  IdentityProfileFieldsProps,
  "isValidatingName" | "nameValidation" | "validatingLabel"
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
};

const NAME_VALIDATION_FEEDBACK_RULES: NameValidationFeedbackRule[] = [
  createValidatingFeedback,
  createRejectedNameFeedback,
];

export function IdentityProfileFields({
  avatar,
  avatarAlt,
  isValidatingName,
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
  const labelClassName = IDENTITY_FIELD_LABEL_CLASS_NAMES[variant];
  const validationFeedback = resolveValidationFeedback({
    isValidatingName,
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
        <div className="min-w-0 flex-1 space-y-2 pt-0.5">
          <label className={labelClassName}>
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
