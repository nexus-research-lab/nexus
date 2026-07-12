import { ArrowRight, CheckCircle2, KeyRound } from "lucide-react";
import type { FormEvent } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiInput } from "@/shared/ui/form/form-control";

import type { LoginFormMode } from "./login-page-model";

interface LoginAuthPanelProps {
  authError: string | null;
  formMode: LoginFormMode;
  isSubmitting: boolean;
  onChangePassword: (value: string) => void;
  onChangeUsername: (value: string) => void;
  onRefresh: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  password: string;
  submitError: string | null;
  username: string;
}

function LoginErrorBanner({ message }: { message: string | null }) {
  if (!message) {
    return null;
  }
  return (
    <div className="mt-5 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-4 py-3 text-sm text-(--destructive)">
      {message}
    </div>
  );
}

function DisabledLoginForm({ onRefresh }: { onRefresh: () => void }) {
  const { t } = useI18n();
  return (
    <div className="mt-7 space-y-4">
      <div className="rounded-[10px] border border-(--divider-subtle-color) bg-white/40 px-4 py-4">
        <h3 className="text-base font-semibold text-(--text-strong)">
          {t("login.disabled_title")}
        </h3>
        <p className="mt-2 text-sm leading-6 text-(--text-muted)">
          {t("login.disabled_description")}
        </p>
      </div>
      <UiButton
        className="min-h-11 w-full rounded-[10px] px-5 text-sm"
        onClick={onRefresh}
        size="lg"
        variant="solid"
      >
        {t("login.refresh")}
      </UiButton>
    </div>
  );
}

function PasswordLoginForm({
  isSubmitting,
  onChangePassword,
  onChangeUsername,
  onSubmit,
  password,
  submitError,
  username,
}: Omit<LoginAuthPanelProps, "authError" | "formMode" | "onRefresh">) {
  const { t } = useI18n();
  return (
    <form className="mt-7 space-y-4" onSubmit={onSubmit}>
      <label className="block" htmlFor="nexus-login-username">
        <span className="mb-2 block text-sm font-semibold text-(--text-default)">
          {t("login.username")}
        </span>
        <UiInput
          autoComplete="username"
          className="min-h-12 rounded-[10px] border-[rgba(117,131,149,0.2)] bg-white/60 px-4 text-base shadow-none"
          controlSize="lg"
          id="nexus-login-username"
          onChange={(event) => onChangeUsername(event.target.value)}
          placeholder={t("login.username_placeholder")}
          type="text"
          value={username}
          variant="surface"
        />
      </label>
      <label className="block" htmlFor="nexus-login-password">
        <span className="mb-2 block text-sm font-semibold text-(--text-default)">
          {t("login.password")}
        </span>
        <UiInput
          autoComplete="current-password"
          className="min-h-12 rounded-[10px] border-[rgba(117,131,149,0.2)] bg-white/60 px-4 text-base shadow-none"
          controlSize="lg"
          id="nexus-login-password"
          onChange={(event) => onChangePassword(event.target.value)}
          placeholder={t("login.password_placeholder")}
          type="password"
          value={password}
          variant="surface"
        />
      </label>
      <LoginErrorBanner message={submitError} />
      <UiButton
        className="min-h-12 w-full rounded-[10px] px-5 text-base shadow-[0_14px_30px_rgba(23,33,44,0.14)]"
        disabled={isSubmitting}
        size="lg"
        tone="primary"
        type="submit"
        variant="solid"
      >
        <span>{isSubmitting ? t("login.submitting") : t("login.submit")}</span>
        <ArrowRight className="h-4 w-4" />
      </UiButton>
    </form>
  );
}

export function LoginAuthPanel({
  authError,
  formMode,
  isSubmitting,
  onChangePassword,
  onChangeUsername,
  onRefresh,
  onSubmit,
  password,
  submitError,
  username,
}: LoginAuthPanelProps) {
  const { t } = useI18n();
  return (
    <section className="relative w-full overflow-hidden rounded-[12px] border border-white/70 bg-[rgba(255,255,255,0.62)] p-6 shadow-[0_26px_58px_rgba(94,108,127,0.14),0_3px_14px_rgba(94,108,127,0.07)] backdrop-blur-xl sm:p-7">
      <div className="flex items-start justify-between gap-5">
        <div className="min-w-0">
          <div className="inline-flex items-center gap-2 rounded-[10px] border border-[rgba(117,131,149,0.18)] bg-white/50 px-2.5 py-1.5 text-xs font-semibold text-[rgba(35,49,63,0.76)]">
            <KeyRound className="h-3.5 w-3.5 text-[rgba(91,114,255,0.9)]" />
            Secure session
          </div>
          <h2 className="mt-5 text-[28px] font-semibold leading-tight text-[#17212c]">
            {t("login.title")}
          </h2>
          <p className="mt-2 text-sm leading-6 text-(--text-muted)">
            Use your Nexus password to continue.
          </p>
        </div>
        <img
          alt=""
          className="h-12 w-12 shrink-0 object-contain drop-shadow-[0_12px_24px_rgba(91,114,255,0.16)]"
          src="/logo.webp"
        />
      </div>

      <LoginErrorBanner message={authError} />
      {formMode === "disabled" ? (
        <DisabledLoginForm onRefresh={onRefresh} />
      ) : (
        <PasswordLoginForm
          isSubmitting={isSubmitting}
          onChangePassword={onChangePassword}
          onChangeUsername={onChangeUsername}
          onSubmit={onSubmit}
          password={password}
          submitError={submitError}
          username={username}
        />
      )}

      <div className="mt-7 flex items-center gap-2 border-t border-[rgba(117,131,149,0.16)] pt-4 text-xs leading-5 text-[rgba(66,81,98,0.72)]">
        <CheckCircle2 className="h-4 w-4 shrink-0 text-[rgba(79,162,159,0.9)]" />
        Authenticated sessions open the launcher without exposing public entry actions.
      </div>
    </section>
  );
}
