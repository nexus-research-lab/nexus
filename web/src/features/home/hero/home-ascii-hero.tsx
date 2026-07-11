"use client";

import { useRef } from "react";

import { getDesktopRuntimeConfig } from "@/config/desktop-runtime";
import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { useTheme } from "@/shared/theme/theme-context";

import { HOME_HERO_LABEL } from "./home-ascii-scene";
import { useHomeAsciiScene } from "./use-home-ascii-scene";

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

  useHomeAsciiScene({
    canvasRef,
    enabled: !shouldReduceMotion,
    sectionRef,
    themeKey: theme,
  });

  return (
    <div
      ref={sectionRef}
      className="relative h-full w-full overflow-hidden rounded-[14px] border"
      style={{
        background: "var(--surface-canvas-background)",
        borderColor: "var(--surface-canvas-border)",
      }}
    >
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          background: "radial-gradient(ellipse 60% 50% at 50% 50%, color-mix(in srgb, var(--primary) 12%, transparent), transparent)",
        }}
      />

      <h2 className="sr-only">{HOME_HERO_LABEL}</h2>

      {shouldReduceMotion ? (
        <div
          className="absolute inset-0 flex items-center justify-center font-mono text-6xl font-light italic leading-none sm:text-7xl lg:text-8xl"
          style={{ color: "var(--primary)" }}
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
