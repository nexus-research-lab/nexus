// INPUT: Login controller page state and access-domain presentation.
// OUTPUT: Accessible bootstrap loading and current-language introduction with the shared authentication panel.
// POS: Login route assembly; authentication decisions stay in the controller and common presentation in features/access.

import { Compass, PanelRightOpen, ShieldCheck } from "lucide-react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { AppLoadingScreen } from "@/shared/ui/layout/app-loading-screen";
import { Navigate } from "react-router-dom";

import { AccessPageFrame, AccessPageIntroduction } from "@/features/access/access-page-frame";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { LoginAuthPanel } from "./login-auth-panel";
import "./login-page.css";
import { useLoginPageController } from "./use-login-page-controller";

const LOGIN_SIGNAL_ITEMS = [
  {
    Icon: Compass,
    copy: "login.intro_launcher_copy",
    title: "login.intro_launcher_title",
  },
  {
    Icon: PanelRightOpen,
    copy: "login.intro_workspace_copy",
    title: "login.intro_workspace_title",
  },
  {
    Icon: ShieldCheck,
    copy: "login.intro_control_copy",
    title: "login.intro_control_title",
  },
] as const;

function LoginIntroduction() {
  const { t } = useI18n();
  return (
    <AccessPageIntroduction
      backHomeLabel={t("login.intro_back_home")}
      eyebrow={t("login.intro_eyebrow")}
      title={t("login.intro_headline")}
      description={t("login.intro_description")}
      artwork={
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -top-10 right-[72px] hidden lg:block xl:right-24"
        >
          <div className="absolute bottom-2 left-8 h-[74px] w-[144px] rounded-full bg-[color:color-mix(in_srgb,var(--brand)_10%,transparent)] blur-2xl" />
          <img
            alt=""
            className="login-hero-illustration relative h-auto w-[228px] xl:w-[246px]"
            src="/nexus/relaxing-generated.png"
          />
        </div>
      }
    >
      <div className="mt-10 hidden max-w-[680px] gap-3 sm:grid sm:grid-cols-3">
        {LOGIN_SIGNAL_ITEMS.map(({ title, copy, Icon }) => (
          <div
            className="min-w-0 border-t border-(--material-input-border) bg-[color:color-mix(in_srgb,var(--card)_28%,transparent)] px-1 py-4"
            key={title}
          >
            <div className="flex items-center gap-2 text-(--text-strong)">
              <Icon className="h-4 w-4 text-[color:color-mix(in_srgb,var(--brand)_88%,transparent)]" />
              <strong className={getUiTypographyClassName({ role: "supporting", weight: "semibold" })}>{t(title)}</strong>
            </div>
            <p className={cn("mt-2", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
              {t(copy)}
            </p>
          </div>
        ))}
      </div>
    </AccessPageIntroduction>
  );
}

export function LoginPage() {
  const controller = useLoginPageController();
  if (controller.pageState.kind === "bootstrapping") {
    return <AppLoadingScreen />;
  }
  if (controller.pageState.kind === "redirect") {
    return <Navigate replace to={controller.pageState.path} />;
  }
  return (
    <AccessPageFrame introduction={<LoginIntroduction />}>
      <LoginAuthPanel
        authFailure={controller.authFailure}
        formMode={controller.pageState.formMode}
        isSubmitting={controller.isSubmitting}
        onChangePassword={controller.setPassword}
        onChangeUsername={controller.setUsername}
        onRefresh={controller.refresh}
        onSubmit={controller.submit}
        password={controller.password}
        submitFailure={controller.submitFailure}
        username={controller.username}
      />
    </AccessPageFrame>
  );
}
