/**
 * INPUT: 密码修改能力、草稿、校验与提交状态。
 * OUTPUT: 可修改时显示表单；不可修改时只显示原因，不渲染禁用字段。
 * POS: 个人设置的密码区，不能提供的动作不得伪装成可配置表单。
 */
import { ChevronDown, Loader2 } from "lucide-react";
import { type FormEvent, useId } from "react";

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
  const helperId = useId();
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
    <details className={cn(SETTINGS_CARD_CLASS_NAME, "group")}>
      <summary className="ui-type-section-title flex cursor-pointer list-none items-center justify-between px-4 py-3 text-(--text-strong) focus-visible:outline-2 focus-visible:outline-(--ring) [&::-webkit-details-marker]:hidden">
        {t("settings.personal.password_title")}
        <ChevronDown aria-hidden="true" className="h-4 w-4 text-(--text-muted) transition-transform group-open:rotate-180" />
      </summary>
      <form aria-busy={isSubmitting} className="grid gap-4 border-t border-(--divider-subtle-color) p-4" onSubmit={handleSubmit}>

        <div className="grid gap-3">
          {PASSWORD_INPUTS.map((input) => (
            <label className="grid items-center gap-2 sm:grid-cols-[1fr_minmax(0,20rem)]" key={input.field}>
              <span className={getUiTypographyClassName({
                role: "supporting",
                tone: "default",
                weight: "regular",
              })}>
                {t(input.labelKey)}
              </span>
              <UiInput
                aria-describedby={helperId}
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
          helperId={helperId}
          canChange={canChange}
          canSubmit={canSubmit}
          hasInput={hasInput}
          isSubmitting={isSubmitting}
          validationError={validationError}
        />
      </form>
    </details>
  );
}

function PasswordSectionHeader({ canChange }: { canChange: boolean }) {
  const { t } = useI18n();
  return (
    <div className="flex items-center gap-3">
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
  helperId,
  canChange,
  canSubmit,
  hasInput,
  isSubmitting,
  validationError,
}: Pick<
  PersonalPasswordSectionProps,
  "canChange" | "canSubmit" | "hasInput" | "isSubmitting" | "validationError"
> & { helperId: string }) {
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
        <p id={helperId} className={getUiTypographyClassName({
          role: "caption",
          tone: showValidation ? "danger" : "soft",
          weight: showValidation ? "medium" : undefined,
        })}
        >
          {helperText}
        </p>
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
