// INPUT: 登录表单状态、已分类的认证/提交恢复事实与用户动作。
// OUTPUT: 共享 Field、Panel、Typography 与恢复提示组成的登录表单，保留输入和提交阻塞态。
// POS: 登录页展示边界；控件视觉归 shared/ui，不推断提交结果或自行重放登录请求。
import { ArrowRight } from "lucide-react";
import type { FormEvent } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
    <UiInlineNotice
      action={notice.action === "check_status"
        ? {
            label: t("login.refresh"),
            onClick: onCheckStatus,
          }
        : undefined}
      className="mt-5"
      message={(
        <RecoverySummary
          impact={notice.impact}
          nextStep={notice.action ? undefined : notice.nextStep}
        />
      )}
      title={notice.title}
      tone="danger"
    />
  );
}

function DisabledLoginForm({ onRefresh }: { onRefresh: () => void }) {
  const { t } = useI18n();
  return (
    <div className="mt-7 space-y-4">
      <div>
        <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
          {t("login.disabled_title")}
        </h3>
        <p className={cn("mt-2", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
          {t("login.disabled_description")}
        </p>
      </div>
      <UiButton
        className="w-full"
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
      <UiField htmlFor="nexus-login-username" label={t("login.username")}>
        <UiInput
          autoComplete="username"
          controlSize="lg"
          id="nexus-login-username"
          onChange={(event) => onChangeUsername(event.target.value)}
          placeholder={t("login.username_placeholder")}
          type="text"
          value={username}
          variant="surface"
        />
      </UiField>
      <UiField htmlFor="nexus-login-password" label={t("login.password")}>
        <UiInput
          autoComplete="current-password"
          controlSize="lg"
          id="nexus-login-password"
          onChange={(event) => onChangePassword(event.target.value)}
          placeholder={t("login.password_placeholder")}
          type="password"
          value={password}
          variant="surface"
        />
      </UiField>
      <LoginErrorBanner notice={submitFailure} onCheckStatus={onRefresh} />
      <UiButton
        className="w-full"
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
    <UiPanel aria-labelledby="nexus-login-title" className="w-full" padding="lg" radius="lg">
      <h2 className={getUiTypographyClassName({ role: "objectTitle", tone: "strong" })} id="nexus-login-title">
        {t("login.title")}
      </h2>

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
    </UiPanel>
  );
}
