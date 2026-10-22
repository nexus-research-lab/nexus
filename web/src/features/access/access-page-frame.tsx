// INPUT: Access-page introduction, form content and single/paired-field width requirements.
// OUTPUT: One branded entry layout and introduction shared by login and initial deployment setup.
// POS: Access-domain presentation pattern; owns brand artwork geometry but no authentication state or commands.

import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import "./access-page.css";

export function AccessPageFrame({ children, formWidth = "regular", headerAction, introduction }: {
  children?: ReactNode;
  formWidth?: "regular" | "wide";
  /** 顶栏左侧动作（如桌面端返回）；与窗口控制同一行，不进入表单。 */
  headerAction?: ReactNode;
  introduction?: ReactNode;
}) {
  return (
    <main
      className="relative min-h-screen overflow-hidden bg-(--secondary) px-5 py-8 text-foreground sm:px-8 lg:px-10"
      data-desktop-window-drag-region
    >
      <div aria-hidden="true" className="access-page-background pointer-events-none absolute inset-0" />
      {/* 入口页没有应用壳顶栏：整页空白背景即窗口手势面（控件按全局规则 no-drag），
          返回等页面级动作只占左上角、排在 traffic lights 之后，不铺全宽遮罩。 */}
      {headerAction ? (
        <div className="absolute left-0 top-0 z-10 flex h-12 items-center px-3" data-desktop-window-controls-leading>
          {headerAction}
        </div>
      ) : null}
      {introduction || children ? (
        <div className={cn(
          "relative mx-auto grid min-h-[calc(100vh-4rem)] w-full max-w-[1180px] grid-cols-1 items-center gap-8 lg:gap-16",
          formWidth === "wide" ? "lg:grid-cols-[minmax(0,0.9fr)_minmax(420px,500px)]" : "lg:grid-cols-[minmax(0,0.96fr)_minmax(360px,430px)]",
        )}>
          {introduction}
          {children}
        </div>
      ) : null}
    </main>
  );
}

export function AccessPageIntroduction({ artwork, backHomeLabel, children, description, eyebrow, title }: {
  artwork?: ReactNode;
  backHomeLabel: string;
  children?: ReactNode;
  description: string;
  eyebrow: string;
  title: string;
}) {
  return (
    <section className="relative min-w-0 py-6" data-access-introduction>
      <Link aria-label={backHomeLabel} className="inline-flex items-center gap-3 text-(--text-strong) no-underline" to={APP_ROUTE_PATHS.root}>
        <img alt="" className="access-brand-mark h-10 w-10 object-contain" src="/logo.png" />
        <span className={getUiTypographyClassName({ role: "objectTitle", tone: "strong" })}>Nexus</span>
      </Link>
      {artwork}
      <div className="mt-10 max-w-[620px] sm:mt-14 lg:mt-20">
        <p className={getUiTypographyClassName({ role: "supporting", tone: "soft", weight: "semibold" })}>{eyebrow}</p>
        <h1 className="access-hero-title mt-4 max-w-[560px] text-(--text-strong)">{title}</h1>
        <p className={cn("mt-6 max-w-[520px]", getUiTypographyClassName({ role: "body", tone: "muted" }))}>{description}</p>
      </div>
      {children}
    </section>
  );
}
