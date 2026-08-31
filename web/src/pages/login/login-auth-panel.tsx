// INPUT: 登录表单状态、已分类的认证/提交恢复事实与用户动作。
// OUTPUT: 保留输入、按 Problem/Impact/Recovery 展示的登录面板。
// POS: 登录页展示边界；不推断提交结果，也不自行重放登录请求。
import { ArrowRight, CheckCircle2, KeyRound } from "lucide-react";
import type { FormEvent } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiInput } from "@/shared/ui/form/form-control";

import type { LoginFormMode } from "./login-page-model";
import type { LoginRecoveryNotice } from "./login-page-model";

interface LoginAuthPanelProps {
  authFailure: LoginRecoveryNotice | null;
  formMode: LoginFormMode;
  isSubmitting: boolean;
  onChangePassword: (value: string) => void;
  onChangeUsername: (value: string) => void;
  onRefresh: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  password: string;
  submitFailure: LoginRecoveryNotice | null;
  username: string;
}

function LoginErrorBanner({
  notice,
  onCheckStatus,
}: {
  notice: LoginRecoveryNotice | null;
  onCheckStatus: () => void;
}) {
  const { t } = useI18n();
  if (!notice) {
    return null;
  }
  return (
    <div
      aria-atomic="true"
      aria-live="polite"
      className="mt-5 rounded-[10px] border border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] px-4 py-3"
      role="status"
    >
      <p className="text-sm font-semibold text-(--destructive)">{notice.title}</p>
      <p className="mt-1 text-sm leading-6 text-(--text-default)">{notice.message}</p>
      <p className="mt-1 text-xs leading-5 text-(--text-muted)">{notice.impact}</p>
      <p className="mt-1 text-xs font-medium leading-5 text-(--text-default)">{notice.nextStep}</p>
      {notice.action === "check_status" ? (
        <UiButton
          className="mt-2"
          onClick={onCheckStatus}
          size="sm"
          type="button"
          variant="text"
        >
          {t("login.refresh")}
        </UiButton>
      ) : null}
    </div>
  );
}

function DisabledLoginForm({ onRefresh }: { onRefresh: () => void }) {
  const { t } = useI18n();
  return (
    <div className="mt-7 space-y-4">
      <div className="rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--card)_56%,transparent)] px-4 py-4">
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
  authFailure,
  isSubmitting,
  onChangePassword,
  onChangeUsername,
  onRefresh,
  onSubmit,
  password,
  submitFailure,
  username,
}: Omit<LoginAuthPanelProps, "formMode">) {
  const { t } = useI18n();
  return (
    <form className="mt-7 space-y-4" onSubmit={onSubmit}>
      <label className="block" htmlFor="nexus-login-username">
        <span className="mb-2 block text-sm font-semibold text-(--text-default)">
          {t("login.username")}
        </span>
        <UiInput
          autoComplete="username"
          className="min-h-12 rounded-[10px] border-(--material-input-border) bg-[color:color-mix(in_srgb,var(--card)_83%,transparent)] px-4 text-base shadow-none"
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
          className="min-h-12 rounded-[10px] border-(--material-input-border) bg-[color:color-mix(in_srgb,var(--card)_83%,transparent)] px-4 text-base shadow-none"
          controlSize="lg"
          id="nexus-login-password"
          onChange={(event) => onChangePassword(event.target.value)}
          placeholder={t("login.password_placeholder")}
          type="password"
          value={password}
          variant="surface"
        />
      </label>
      <LoginErrorBanner notice={submitFailure} onCheckStatus={onRefresh} />
      <UiButton
        className="min-h-12 w-full rounded-[10px] px-5 text-base shadow-[0_14px_30px_color-mix(in_srgb,var(--shadow-color)_14%,transparent)]"
        disabled={
          isSubmitting
          || Boolean(authFailure?.blocksSubmit)
          || Boolean(submitFailure?.blocksSubmit)
        }
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
  authFailure,
  formMode,
  isSubmitting,
  onChangePassword,
  onChangeUsername,
  onRefresh,
  onSubmit,
  password,
  submitFailure,
  username,
}: LoginAuthPanelProps) {
  const { t } = useI18n();
  return (
    <section className="relative w-full overflow-hidden rounded-[12px] border border-[color:color-mix(in_srgb,var(--card)_97%,transparent)] bg-[color:color-mix(in_srgb,var(--card)_86%,transparent)] p-6 shadow-(--modal-dialog-surface-shadow) backdrop-blur-xl sm:p-7">
      <div className="flex items-start justify-between gap-5">
        <div className="min-w-0">
          <div className="inline-flex items-center gap-2 rounded-[10px] border border-(--material-input-border) bg-[color:color-mix(in_srgb,var(--card)_69%,transparent)] px-2.5 py-1.5 text-xs font-semibold text-(--text-muted)">
            <KeyRound className="h-3.5 w-3.5 text-[color:color-mix(in_srgb,var(--brand)_90%,transparent)]" />
            Secure session
          </div>
          <h2 className="mt-5 text-xl font-semibold leading-tight text-(--text-strong)">
            {t("login.title")}
          </h2>
          <p className="mt-2 text-sm leading-6 text-(--text-muted)">
            Use your Nexus password to continue.
          </p>
        </div>
        <img
          alt=""
          className="h-12 w-12 shrink-0 object-contain drop-shadow-[0_12px_24px_color-mix(in_srgb,var(--brand)_16%,transparent)]"
          src="/logo.webp"
        />
      </div>

      <LoginErrorBanner notice={authFailure} onCheckStatus={onRefresh} />
      {formMode === "disabled" ? (
        <DisabledLoginForm onRefresh={onRefresh} />
      ) : (
        <PasswordLoginForm
          authFailure={authFailure}
          isSubmitting={isSubmitting}
          onChangePassword={onChangePassword}
          onChangeUsername={onChangeUsername}
          onRefresh={onRefresh}
          onSubmit={onSubmit}
          password={password}
          submitFailure={submitFailure}
          username={username}
        />
      )}

      <div className="mt-7 flex items-center gap-2 border-t border-(--material-input-border) pt-4 text-xs leading-5 text-(--text-muted)">
        <CheckCircle2 className="h-4 w-4 shrink-0 text-[color:color-mix(in_srgb,var(--accent)_90%,transparent)]" />
        Authenticated sessions open the launcher without exposing public entry actions.
      </div>
    </section>
  );
}
