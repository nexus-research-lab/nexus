// INPUT: Agent identity fields, presentation context and name-validation feedback.
// OUTPUT: Avatar/name editing callbacks and current validation feedback using shared controls.
// POS: Identity field composition; validation authority and profile persistence remain with the caller.

import { useId } from "react";

import { UiField, UiInput } from "@/shared/ui/form/form-control";
import type { AgentNameValidationResult } from "@/types/agent/agent";

import { IdentityAvatarPicker } from "./identity-avatar-picker";
import type { AgentIdentityVariant } from "./identity-layout";

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
  const nameId = useId();

  return (
    <div className="flex items-start gap-3">
      <IdentityAvatarPicker
        avatar={avatar}
        avatarAlt={avatarAlt}
        name={title || avatarAlt}
        onChange={onAvatarChange}
        variant={variant}
      />
      <UiField
        className="min-w-0 flex-1 pt-0.5"
        description={isValidatingName ? <span role="status">{validatingLabel}</span> : undefined}
        error={isValidatingName ? undefined : nameValidation?.reason}
        htmlFor={nameId}
        label={nameLabel}
        required
      >
        <UiInput
          aria-busy={isValidatingName || undefined}
          controlSize="md"
          data-autofocus="true"
          id={nameId}
          onChange={(event) => onTitleChange(event.target.value)}
          placeholder={namePlaceholder}
          required
          type="text"
          value={title}
        />
      </UiField>
    </div>
  );
}
