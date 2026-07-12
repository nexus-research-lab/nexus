import { Compass, PanelRightOpen, ShieldCheck } from "lucide-react";
import { Link, Navigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";

import { LoginAuthPanel } from "./login-auth-panel";
import { useLoginPageController } from "./use-login-page-controller";

const LOGIN_SIGNAL_ITEMS = [
  {
    Icon: Compass,
    copy: "Route work to the right room, DM, or app surface.",
    title: "Launcher",
  },
  {
    Icon: PanelRightOpen,
    copy: "Keep files, history, and review context in one place.",
    title: "Workspace",
  },
  {
    Icon: ShieldCheck,
    copy: "Open Nexus with one authenticated operating surface.",
    title: "Control",
  },
] as const;

function LoginBackground() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none absolute inset-0 bg-[#ededec] bg-[linear-gradient(rgba(255,255,255,0.18),rgba(255,255,255,0.18)),linear-gradient(90deg,rgba(255,255,255,0.46)_1px,transparent_1px),linear-gradient(60deg,rgba(255,255,255,0.42)_1px,transparent_1px),linear-gradient(120deg,rgba(255,255,255,0.42)_1px,transparent_1px)] bg-[length:100%_100%,160px_138px,160px_138px,160px_138px]"
    />
  );
}

function LoginIntroduction() {
  return (
    <section className="relative min-w-0 py-6">
      <Link
        aria-label="Back to Nexus landing"
        className="inline-flex items-center gap-3 text-(--text-strong) no-underline"
        to={APP_ROUTE_PATHS.landing}
      >
        <img
          alt=""
          className="h-10 w-10 object-contain drop-shadow-[0_12px_24px_rgba(91,114,255,0.18)]"
          src="/logo.webp"
        />
        <span className="text-[28px] font-semibold leading-none">NEXUS</span>
      </Link>

      <div
        aria-hidden="true"
        className="pointer-events-none absolute -top-10 right-[72px] hidden lg:block xl:right-24"
      >
        <div className="absolute bottom-2 left-8 h-[74px] w-[144px] rounded-full bg-[rgba(91,114,255,0.10)] blur-2xl" />
        <img
          alt=""
          className="relative h-auto w-[228px] drop-shadow-[0_22px_30px_rgba(91,114,255,0.15)] xl:w-[246px]"
          src="/nexus/relaxing-generated.png"
        />
      </div>

      <div className="mt-10 max-w-[620px] sm:mt-14 lg:mt-20">
        <p className="text-sm font-semibold text-(--text-soft)">
          Private workspace access
        </p>
        <h1 className="mt-4 max-w-[560px] text-[44px] font-semibold leading-[0.98] text-[#17212c] sm:text-[64px]">
          Enter the operating surface.
        </h1>
        <p className="mt-6 max-w-[520px] text-[17px] leading-8 text-[rgba(66,81,98,0.76)]">
          Sign in to open the launcher, rooms, workspace files, and review surfaces that keep
          agent work visible.
        </p>
      </div>

      <div className="mt-10 hidden max-w-[680px] gap-3 sm:grid sm:grid-cols-3">
        {LOGIN_SIGNAL_ITEMS.map(({ title, copy, Icon }) => (
          <div
            className="min-w-0 border-t border-[rgba(117,131,149,0.18)] bg-white/20 px-1 py-4"
            key={title}
          >
            <div className="flex items-center gap-2 text-[#17212c]">
              <Icon className="h-4 w-4 text-[rgba(91,114,255,0.88)]" />
              <strong className="text-sm font-semibold">{title}</strong>
            </div>
            <p className="mt-2 text-[13px] leading-5 text-[rgba(66,81,98,0.72)]">
              {copy}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}

export function LoginPage() {
  const controller = useLoginPageController();
  if (controller.pageState.kind === "bootstrapping") {
    return (
      <main className="relative min-h-screen overflow-hidden bg-[#ededec] text-foreground">
        <LoginBackground />
      </main>
    );
  }
  if (controller.pageState.kind === "redirect") {
    return <Navigate replace to={controller.pageState.path} />;
  }
  return (
    <main className="relative min-h-screen overflow-hidden bg-[#ededec] px-5 py-8 text-foreground sm:px-8 lg:px-10">
      <LoginBackground />
      <div className="relative mx-auto grid min-h-[calc(100vh-4rem)] w-full max-w-[1180px] grid-cols-1 items-center gap-8 lg:grid-cols-[minmax(0,0.96fr)_minmax(360px,430px)] lg:gap-16">
        <LoginIntroduction />
        <LoginAuthPanel
          authError={controller.authError}
          formMode={controller.pageState.formMode}
          isSubmitting={controller.isSubmitting}
          onChangePassword={controller.setPassword}
          onChangeUsername={controller.setUsername}
          onRefresh={controller.refresh}
          onSubmit={controller.submit}
          password={controller.password}
          submitError={controller.submitError}
          username={controller.username}
        />
      </div>
    </main>
  );
}
