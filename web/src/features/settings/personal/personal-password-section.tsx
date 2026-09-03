/**
 * INPUT: 密码修改能力、草稿、校验与提交状态。
 * OUTPUT: 可修改时显示表单；不可修改时只显示原因，不渲染禁用字段。
 * POS: 个人设置的密码区，不能提供的动作不得伪装成可配置表单。
 */
import { Loader2, LockKeyhole } from "lucide-react";
import type { FormEvent } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInput } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type { PasswordDraft, PasswordField } from "./personal-settings-model";
import { SETTINGS_CARD_CLASS_NAME } from "../shared/settings-panel-ui";

interface PasswordInputConfig {
  autoComplete: "current-password" | "new-password";
  field: PasswordField;
  labelKey: TranslationKey;
}

interface PersonalPasswordSectionProps {
  canChange: boolean;
  canSubmit: boolean;
  draft: PasswordDraft;
  hasInput: boolean;
  isSubmitting: boolean;
  mutationBlocked: boolean;
  onFieldChange: (field: PasswordField, value: string) => void;
  onSubmit: () => void;
  validationError: string | null;
}

const PASSWORD_INPUTS: readonly PasswordInputConfig[] = [
  {
    autoComplete: "current-password",
    field: "currentPassword",
    labelKey: "settings.personal.password_current",
  },
  {
    autoComplete: "new-password",
    field: "newPassword",
    labelKey: "settings.personal.password_new",
  },
  {
    autoComplete: "new-password",
    field: "confirmPassword",
    labelKey: "settings.personal.password_confirm",
  },
];

export function PersonalPasswordSection({
  canChange,
  canSubmit,
  draft,
  hasInput,
  isSubmitting,
  mutationBlocked,
  onFieldChange,
  onSubmit,
  validationError,
}: PersonalPasswordSectionProps) {
  const { t } = useI18n();
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit();
  };

  if (!canChange) {
    return (
      <section className={cn(SETTINGS_CARD_CLASS_NAME, "px-3 py-3")}>
        <PasswordSectionHeader canChange={false} />
      </section>
    );
  }

  return (
    <section className={SETTINGS_CARD_CLASS_NAME}>
      <form className="grid gap-3 px-3 py-3" onSubmit={handleSubmit}>
        <PasswordSectionHeader canChange={canChange} />

        <div className="grid gap-3 md:grid-cols-3">
          {PASSWORD_INPUTS.map((input) => (
            <label className="space-y-1.5" key={input.field}>
              <span className={getUiTypographyClassName({
                role: "caption",
                tone: "muted",
                weight: "semibold",
              })}>
                {t(input.labelKey)}
              </span>
              <UiInput
                autoComplete={input.autoComplete}
                disabled={isSubmitting || mutationBlocked}
                onChange={(event) => onFieldChange(input.field, event.target.value)}
                type="password"
                value={draft[input.field]}
              />
            </label>
          ))}
        </div>

        <PasswordSubmitActions
          canChange={canChange}
          canSubmit={canSubmit}
          hasInput={hasInput}
          isSubmitting={isSubmitting}
          validationError={validationError}
        />
      </form>
    </section>
  );
}

function PasswordSectionHeader({ canChange }: { canChange: boolean }) {
  const { t } = useI18n();
  return (
    <div className="flex items-center gap-3">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center radius-control-lg bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
        <LockKeyhole className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0">
        <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
          {t("settings.personal.password_title")}
        </h3>
        {!canChange ? (
          <p className={cn(
            "mt-1",
            getUiTypographyClassName({ role: "metadata", tone: "soft" }),
          )}>
            {t("settings.personal.password_disabled")}
          </p>
        ) : null}
      </div>
    </div>
  );
}

function PasswordSubmitActions({
  canChange,
  canSubmit,
  hasInput,
  isSubmitting,
  validationError,
}: Pick<
  PersonalPasswordSectionProps,
  "canChange" | "canSubmit" | "hasInput" | "isSubmitting" | "validationError"
>) {
  const { t } = useI18n();
  const helperText = resolvePasswordHelperText(
    validationError,
    canChange,
    hasInput,
    t("settings.personal.password_rule"),
  );
  const showValidation = Boolean(validationError && canChange && hasInput);
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div
        aria-atomic={showValidation ? "true" : undefined}
        aria-live={showValidation ? "polite" : undefined}
        className="min-w-0 flex-1"
        role={showValidation ? "status" : undefined}
      >
        <p className={getUiTypographyClassName({
          role: "caption",
          tone: showValidation ? "danger" : "soft",
          weight: showValidation ? "medium" : undefined,
        })}
        >
          {helperText}
        </p>
        {showValidation ? (
          <>
            <p className={getUiTypographyClassName({ role: "caption", tone: "muted" })}>
              {t("state.validation_failure_impact")}
            </p>
            <p className={getUiTypographyClassName({ role: "caption", tone: "default" })}>
              {t("state.validation_failure_next_step")}
            </p>
          </>
        ) : null}
      </div>
      <UiButton
        className="min-w-28"
        disabled={!canSubmit}
        size="md"
        tone={canSubmit ? "primary" : "default"}
        type="submit"
        variant={canSubmit ? "solid" : "surface"}
      >
        {isSubmitting ? (
          <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
        ) : null}
        {isSubmitting ? t("common.saving") : t("settings.personal.change_password")}
      </UiButton>
    </div>
  );
}

function resolvePasswordHelperText(
  validationError: string | null,
  canChange: boolean,
  hasInput: boolean,
  fallback: string,
): string {
  return validationError && canChange && hasInput ? validationError : fallback;
}
