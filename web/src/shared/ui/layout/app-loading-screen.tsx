// INPUT: 启动或认证关键路径的用户可见文案、外部布局和品牌动画尺寸。
// OUTPUT: 支持 reduced-motion 静态帧、可访问状态与共享排版的 Nexus 启动加载面。
// POS: App 启动品牌状态；不替代普通页面、按钮或资源加载指示器。

import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface AppLoadingStateProps {
  className?: string;
  animationClassName?: string;
  message: ReactNode;
}

export function AppLoadingState({
  className: className,
  animationClassName: animationClassName = "h-32 w-32 shrink-0",
  message,
}: AppLoadingStateProps) {
  return (
    <div
      aria-atomic="true"
      aria-busy="true"
      aria-live="polite"
      className={cn("flex flex-col items-center gap-3 px-12 py-10 text-center", className)}
      role="status"
    >
      <picture className={cn("block", animationClassName)}>
        <source
          media="(prefers-reduced-motion: reduce)"
          srcSet="/lotties/cat-loading-static.webp"
          type="image/webp"
        />
        <img
          alt=""
          className="h-full w-full object-contain"
          decoding="async"
          src="/lotties/cat-loading.webp"
        />
      </picture>
      <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>
        {message}
      </p>
    </div>
  );
}

export function AppLoadingScreen() {
  const { t } = useI18n();

  return (
    <main className="relative flex h-screen w-full items-center justify-center overflow-hidden bg-background px-6 text-foreground">
      <AppLoadingState message={t("common.loading")} />
    </main>
  );
}
