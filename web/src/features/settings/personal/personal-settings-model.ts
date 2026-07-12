import type {
  PersonalProfile,
  TokenUsageSummary,
} from "@/lib/api/account/auth-api";
import { formatTokens } from "@/lib/format/token-count";
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

export interface PersonalProfilePresentation {
  authMethodLabel: string;
  avatarName: string;
  canUpdateProfile: boolean;
  displayName: string;
  roleLabel: string;
  subscriptionPlanName: string | null;
  username: string;
}

export type TokenUsageMetricKey = "quota" | "input" | "output" | "cache";
export type TokenUsageValueKey = "input" | "output" | "cache";

export interface TokenUsagePresentation {
  messageCount: number;
  metrics: Record<TokenUsageMetricKey, string>;
  sessionCount: number;
  tokenValues: Record<TokenUsageValueKey, number>;
  totalTokens: string;
  updatedAt: string;
}

export interface PersonalControllerPresentation {
  avatarCanUpdate: boolean;
  avatarValue: string;
  canChangePassword: boolean;
  canSubmitPassword: boolean;
  validationError: string | null;
}

interface PasswordValidationContext {
  canChangePassword: boolean;
  draft: PasswordDraft;
}

interface PasswordValidationRule {
  invalid: (context: PasswordValidationContext) => boolean;
  messageKey: TranslationKey;
}

type TranslateFn = (
  key: TranslationKey,
  params?: Record<string, string | number>,
) => string;

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

const EMPTY_TOKEN_USAGE: TokenUsageSummary = {
  cache_creation_input_tokens: 0,
  cache_read_input_tokens: 0,
  input_tokens: 0,
  message_count: 0,
  output_tokens: 0,
  quota_limit_tokens: null,
  session_count: 0,
  total_tokens: 0,
  updated_at: "",
};

const EMPTY_PERSONAL_USER: PersonalProfile["user"] = {
  auth_method: "",
  avatar: "",
  display_name: "",
  role: "",
  user_id: "",
  username: "",
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

function getAuthMethodLabelKey(value: string): TranslationKey {
  return AUTH_METHOD_LABEL_KEYS[value] ?? "settings.personal.auth_method_local";
}

function getRoleLabelKey(value: string): TranslationKey | null {
  return ROLE_LABEL_KEYS[value] ?? null;
}

export function buildPersonalProfilePresentation(
  profile: PersonalProfile | null,
  translate: TranslateFn,
): PersonalProfilePresentation {
  const user = resolveProfileUser(profile);
  return {
    authMethodLabel: translate(getAuthMethodLabelKey(user.auth_method)),
    avatarName: firstDisplayValue([
      user.display_name,
      user.username,
      translate("settings.personal.avatar_alt"),
    ]),
    canUpdateProfile: profileCanUpdate(profile),
    displayName: firstDisplayValue([user.display_name, user.username]),
    roleLabel: resolveRoleLabel(user.role, translate),
    subscriptionPlanName: resolveSubscriptionPlanName(profile),
    username: displayValue(user.username),
  };
}

export function buildPersonalControllerPresentation(
  profile: PersonalProfile | null,
  passwordDraft: PasswordDraft,
  isLoading: boolean,
  isSavingAvatar: boolean,
  isSubmittingPassword: boolean,
  translate: TranslateFn,
): PersonalControllerPresentation {
  const canChangePassword = profileCanChangePassword(profile);
  const validationError = resolvePasswordValidationError(
    getPasswordValidationKey({ canChangePassword, draft: passwordDraft }),
    translate,
  );
  return {
    avatarCanUpdate: profileCanUpdate(profile) && !isSavingAvatar,
    avatarValue: resolveProfileUser(profile).avatar,
    canChangePassword,
    canSubmitPassword: canSubmitPassword(
      validationError,
      isSubmittingPassword,
      isLoading,
    ),
    validationError,
  };
}

export function buildTokenUsagePresentation(
  usage: TokenUsageSummary | undefined,
  locale: "zh" | "en",
  translate: TranslateFn,
): TokenUsagePresentation {
  const source = usage ?? EMPTY_TOKEN_USAGE;
  const cacheTokens = source.cache_creation_input_tokens
    + source.cache_read_input_tokens;
  return {
    messageCount: source.message_count,
    metrics: {
      quota: formatQuota(source, translate),
      input: formatTokens(source.input_tokens),
      output: formatTokens(source.output_tokens),
      cache: formatTokens(cacheTokens),
    },
    sessionCount: source.session_count,
    tokenValues: {
      input: source.input_tokens,
      output: source.output_tokens,
      cache: cacheTokens,
    },
    totalTokens: formatTokens(source.total_tokens),
    updatedAt: formatUpdatedAt(source.updated_at, locale),
  };
}

function firstDisplayValue(values: Array<string | null | undefined>): string {
  return values.find(Boolean) ?? "--";
}

function resolveProfileUser(profile: PersonalProfile | null): PersonalProfile["user"] {
  return profile?.user ?? EMPTY_PERSONAL_USER;
}

function profileCanUpdate(profile: PersonalProfile | null): boolean {
  return profile?.can_update_profile === true;
}

function profileCanChangePassword(profile: PersonalProfile | null): boolean {
  return profile?.can_change_password === true;
}

function resolveSubscriptionPlanName(profile: PersonalProfile | null): string | null {
  return profile?.subscription?.plan_name ?? null;
}

function resolvePasswordValidationError(
  validationKey: TranslationKey | null,
  translate: TranslateFn,
): string | null {
  return validationKey ? translate(validationKey) : null;
}

function canSubmitPassword(
  validationError: string | null,
  isSubmitting: boolean,
  isLoading: boolean,
): boolean {
  return !validationError && !isSubmitting && !isLoading;
}

function displayValue(value?: string | null): string {
  return value || "--";
}

function resolveRoleLabel(role: string, translate: TranslateFn): string {
  const labelKey = getRoleLabelKey(role);
  return labelKey ? translate(labelKey) : displayValue(role);
}

function formatQuota(
  usage: TokenUsageSummary,
  translate: TranslateFn,
): string {
  if (usage.quota_limit_tokens === null) {
    return translate("settings.personal.quota_unset");
  }
  return `${formatTokens(usage.total_tokens)} / ${formatTokens(usage.quota_limit_tokens)}`;
}

function formatUpdatedAt(value: string, locale: "zh" | "en"): string {
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) {
    return "--";
  }
  return date.toLocaleString(locale === "zh" ? "zh-CN" : "en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function getPasswordValidationKey(
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
