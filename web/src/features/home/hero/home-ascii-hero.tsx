// INPUT: Theme, reduced-motion preference, desktop platform and the launchpad content below the wordmark.
// OUTPUT: 首页画布：顶部 ASCII 字标带（Canvas 或静态字标）与其下方可滚动的启动台内容。
// POS: Hero presentation; scene lifecycle stays in useHomeAsciiScene.
"use client";

import { useRef, type CSSProperties, type ReactNode } from "react";

import { getDesktopRuntimeConfig } from "@/config/desktop-runtime";
import { usePrefersReducedMotion } from "@/shared/lib/react/use-prefers-reduced-motion";
import { useTheme } from "@/shared/theme/theme-context";

import { HOME_HERO_LABEL } from "./home-ascii-scene";
import { useHomeAsciiScene } from "./use-home-ascii-scene";

type HomeAsciiHeroStyle = CSSProperties & {
  "--home-ascii-clock-ink"?: string;
  "--home-ascii-hero-ink"?: string;
};

const PRESERVED_LIGHT_HERO_STYLE: HomeAsciiHeroStyle = {
  "--home-ascii-clock-ink": "#14202c",
  "--home-ascii-hero-ink": "#5b72ff",
};

function shouldReduceHomeHeroMotion(prefersReducedMotion: boolean): boolean {
  if (!prefersReducedMotion) {
    return false;
  }

  const runtimeConfig = getDesktopRuntimeConfig();
  return runtimeConfig?.appMode !== "desktop" || runtimeConfig.platform !== "windows";
}

export function HomeAsciiHero({ children }: { children?: ReactNode }) {
  const { theme } = useTheme();
  const sectionRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const prefersReducedMotion = usePrefersReducedMotion();
  const shouldReduceMotion = shouldReduceHomeHeroMotion(prefersReducedMotion);
  const preservedVisualStyle =
    theme === "light" || theme === "sunny"
      ? PRESERVED_LIGHT_HERO_STYLE
      : undefined;

  useHomeAsciiScene({
    canvasRef,
    enabled: !shouldReduceMotion,
    sectionRef,
    themeKey: theme,
  });

  return (
    // 画布与其他工作区页面同一纯色底面；字标只靠自身墨色出现，不再叠渐变或光晕。
    <div
      className="surface-radius-md relative h-full w-full overflow-hidden border"
      style={{
        ...preservedVisualStyle,
        background: "var(--surface-canvas-background)",
        borderColor: "var(--surface-canvas-border)",
      }}
    >

      <div className="relative flex h-full min-h-0 flex-col overflow-y-auto">
        {/* 字标带：Canvas 只铺满这一段，指针交互与时钟都限定在字标附近。 */}
        <div
          ref={sectionRef}
          className="relative h-[clamp(170px,30vh,280px)] w-full shrink-0"
        >
          <h2 className="sr-only">{HOME_HERO_LABEL}</h2>

          {shouldReduceMotion ? (
            <div
              className="absolute inset-0 flex items-center justify-center font-mono text-6xl font-light italic leading-none sm:text-7xl lg:text-8xl"
              style={{
                color: "var(--home-ascii-hero-ink, var(--primary))",
              }}
            >
              {HOME_HERO_LABEL}
            </div>
          ) : (
            <canvas
              ref={canvasRef}
              className="absolute inset-0 block cursor-crosshair"
            />
          )}
        </div>
        {children}
      </div>
    </div>
  );
}
