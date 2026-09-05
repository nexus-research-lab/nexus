// INPUT: Agent identity fields, presentation context and name-validation feedback.
// OUTPUT: Avatar/name editing callbacks and current validation feedback using shared controls.
// POS: Identity field composition; validation authority and profile persistence remain with the caller.

import { UiInput } from "@/shared/ui/form/form-control";
import type { AgentNameValidationResult } from "@/types/agent/agent";

import { IdentityAvatarPicker } from "./identity-avatar-picker";
import {
  IDENTITY_FIELD_LABEL_CLASS_NAMES,
  type AgentIdentityVariant,
} from "./identity-layout";

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
  const labelClassName = IDENTITY_FIELD_LABEL_CLASS_NAMES[variant];
  const validationFeedback = resolveValidationFeedback({
    isValidatingName,
    nameValidation,
    validatingLabel,
  });

  return (
    <>
      <div className="flex items-start gap-3">
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
