import { Loader2, LockKeyhole } from "lucide-react";
import type { FormEvent } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { getUiButtonClassName } from "@/shared/ui/button/button-styles";

import type { PasswordDraft, PasswordField } from "./personal-settings-model";

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

const PRIMARY_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "md", tone: "primary", variant: "solid" },
  "gap-2 tracking-tight",
);
const SECONDARY_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "md", variant: "surface" },
  "gap-2 tracking-tight",
);

export function PersonalPasswordSection({
  canChange,
  canSubmit,
  draft,
  hasInput,
  isSubmitting,
  onFieldChange,
  onSubmit,
  validationError,
}: PersonalPasswordSectionProps) {
  const { t } = useI18n();
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit();
  };

  return (
    <section className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
      <form className="grid gap-3 px-3 py-3" onSubmit={handleSubmit}>
        <PasswordSectionHeader canChange={canChange} />

        <div className="grid gap-3 md:grid-cols-3">
          {PASSWORD_INPUTS.map((input) => (
            <label className="space-y-1.5" key={input.field}>
              <span className="text-[11px] font-semibold text-(--text-muted)">
                {t(input.labelKey)}
              </span>
              <input
                autoComplete={input.autoComplete}
                className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)"
                disabled={!canChange || isSubmitting}
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
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
        <LockKeyhole className="h-3.5 w-3.5" />
      </div>
      <div className="min-w-0">
        <h3 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
          {t("settings.personal.password_title")}
        </h3>
        {!canChange ? (
          <p className="mt-1 text-[12px] leading-5 text-(--text-soft)">
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
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <p className="min-w-0 text-[11px] text-(--text-soft)">{helperText}</p>
      <button
        className={cn(
          canSubmit ? PRIMARY_BUTTON_CLASS_NAME : SECONDARY_BUTTON_CLASS_NAME,
          "min-w-28",
        )}
        disabled={!canSubmit}
        type="submit"
      >
        {isSubmitting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
        {isSubmitting ? t("common.saving") : t("settings.personal.change_password")}
      </button>
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
