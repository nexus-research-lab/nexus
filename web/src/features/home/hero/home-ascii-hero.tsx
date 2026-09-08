"use client";

import { useRef, type CSSProperties } from "react";

import { getDesktopRuntimeConfig } from "@/config/desktop-runtime";
import { usePrefersReducedMotion } from "@/shared/lib/react/use-prefers-reduced-motion";
import { useTheme } from "@/shared/theme/theme-context";

import { HOME_HERO_LABEL } from "./home-ascii-scene";
import { useHomeAsciiScene } from "./use-home-ascii-scene";

type HomeAsciiHeroStyle = CSSProperties & {
  "--home-ascii-clock-ink"?: string;
  "--home-ascii-hero-ink"?: string;
  "--home-ascii-surface-background"?: string;
  "--home-ascii-surface-border"?: string;
};

const PRESERVED_LIGHT_HERO_STYLE: HomeAsciiHeroStyle = {
  "--home-ascii-clock-ink": "#14202c",
  "--home-ascii-hero-ink": "#5b72ff",
  "--home-ascii-surface-background":
    "linear-gradient(180deg, rgba(251, 253, 255, 0.68), rgba(246, 249, 253, 0.6) 100%), #ebeae4",
  "--home-ascii-surface-border": "rgba(255, 255, 255, 0.32)",
};

function shouldReduceHomeHeroMotion(prefersReducedMotion: boolean): boolean {
  if (!prefersReducedMotion) {
    return false;
  }

  const runtimeConfig = getDesktopRuntimeConfig();
  return runtimeConfig?.appMode !== "desktop" || runtimeConfig.platform !== "windows";
}

export function HomeAsciiHero() {
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
    <div
      ref={sectionRef}
      className="surface-radius-md relative h-full w-full overflow-hidden border"
      style={{
        ...preservedVisualStyle,
        background:
          "var(--home-ascii-surface-background, var(--surface-canvas-background))",
        borderColor:
          "var(--home-ascii-surface-border, var(--surface-canvas-border))",
      }}
    >
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(ellipse 60% 50% at 50% 50%, color-mix(in srgb, var(--home-ascii-hero-ink, var(--primary)) 12%, transparent), transparent)",
        }}
      />

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
  );
}
