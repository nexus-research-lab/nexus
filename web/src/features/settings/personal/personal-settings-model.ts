import type { TranslationKey } from "@/shared/i18n/messages";

export interface PasswordDraft {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export type PasswordField = keyof PasswordDraft;

export interface PersonalSettingsFeedback {
  message: string;
  title: string;
  tone: "success" | "error";
}

interface PasswordValidationContext {
  canChangePassword: boolean;
  draft: PasswordDraft;
}

interface PasswordValidationRule {
  invalid: (context: PasswordValidationContext) => boolean;
  messageKey: TranslationKey;
}

export const EMPTY_PASSWORD_DRAFT: PasswordDraft = {
  confirmPassword: "",
  currentPassword: "",
  newPassword: "",
};

const AUTH_METHOD_LABEL_KEYS: Readonly<Record<string, TranslationKey>> = {
  bearer: "settings.personal.auth_method_bearer",
  password: "settings.personal.auth_method_password",
};

const ROLE_LABEL_KEYS: Readonly<Record<string, TranslationKey>> = {
  admin: "settings.personal.role_admin",
  member: "settings.personal.role_member",
  owner: "settings.personal.role_owner",
};

const PASSWORD_VALIDATION_RULES: readonly PasswordValidationRule[] = [
  {
    invalid: ({ canChangePassword }) => !canChangePassword,
    messageKey: "settings.personal.password_disabled",
  },
  {
    invalid: ({ draft }) => !draft.currentPassword.trim(),
    messageKey: "settings.personal.validation_current_required",
  },
  {
    invalid: ({ draft }) => !draft.newPassword.trim(),
    messageKey: "settings.personal.validation_new_required",
  },
  {
    invalid: ({ draft }) => draft.newPassword.length < 8,
    messageKey: "settings.personal.validation_new_length",
  },
  {
    invalid: ({ draft }) => draft.newPassword !== draft.confirmPassword,
    messageKey: "settings.personal.validation_confirm_mismatch",
  },
];

export function getAuthMethodLabelKey(value: string): TranslationKey {
  return AUTH_METHOD_LABEL_KEYS[value] ?? "settings.personal.auth_method_local";
}

export function getRoleLabelKey(value: string): TranslationKey | null {
  return ROLE_LABEL_KEYS[value] ?? null;
}

export function getPasswordValidationKey(
  context: PasswordValidationContext,
): TranslationKey | null {
  return PASSWORD_VALIDATION_RULES.find((rule) => rule.invalid(context))?.messageKey ?? null;
}

export function hasPasswordDraftInput(draft: PasswordDraft): boolean {
  return Object.values(draft).some(Boolean);
}

export function updatePasswordDraft(
  draft: PasswordDraft,
  field: PasswordField,
  value: string,
): PasswordDraft {
  return { ...draft, [field]: value };
}
