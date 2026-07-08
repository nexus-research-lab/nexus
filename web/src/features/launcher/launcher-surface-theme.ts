import { CSSProperties } from "react";

import { Theme } from "@/shared/theme/theme-context";

type LauncherSurfaceThemeStyle = CSSProperties &
  Record<`--launcher-${string}`, string>;

interface LauncherSurfaceConfig {
  accentColor: string;
  accentHoverColor: string;
  dividerColor: string;
  heroAura: string;
  heroInnerFill: string;
  heroInnerStroke: string;
  heroStop1: string;
  heroStop2: string;
  heroStop3: string;
  heroStroke: string;
  heroTint1: string;
  heroTint2: string;
  heroTint3: string;
  heroTint4: string;
  inputFill: string;
  inputIcon: string;
  inputInnerFill: string;
  inputInnerStroke: string;
  inputPlaceholder: string;
  inputStroke: string;
  inputText: string;
  metaText: string;
  submitBackground: string;
  submitBorder: string;
  submitColor: string;
  submitShadow: string;
}

function buildLauncherSurfaceThemeStyle(
  config: LauncherSurfaceConfig,
): LauncherSurfaceThemeStyle {
  return {
    "--launcher-stage-background": "var(--ambient-page-background)",
    "--launcher-stage-pattern": "var(--ambient-page-pattern)",
    "--launcher-divider-color": config.dividerColor,
    "--launcher-hero-aura": config.heroAura,
    "--launcher-hero-stop-1": config.heroStop1,
    "--launcher-hero-stop-2": config.heroStop2,
    "--launcher-hero-stop-3": config.heroStop3,
    "--launcher-hero-stroke": config.heroStroke,
    "--launcher-hero-inner-fill": config.heroInnerFill,
    "--launcher-hero-inner-stroke": config.heroInnerStroke,
    "--launcher-hero-tint-1": config.heroTint1,
    "--launcher-hero-tint-2": config.heroTint2,
    "--launcher-hero-tint-3": config.heroTint3,
    "--launcher-hero-tint-4": config.heroTint4,
    "--launcher-input-fill": config.inputFill,
    "--launcher-input-stroke": config.inputStroke,
    "--launcher-input-inner-fill": config.inputInnerFill,
    "--launcher-input-inner-stroke": config.inputInnerStroke,
    "--launcher-input-icon": config.inputIcon,
    "--launcher-input-text": config.inputText,
    "--launcher-input-placeholder": config.inputPlaceholder,
    "--launcher-submit-background": config.submitBackground,
    "--launcher-submit-border": config.submitBorder,
    "--launcher-submit-color": config.submitColor,
    "--launcher-submit-shadow": config.submitShadow,
    "--launcher-meta-text": config.metaText,
    "--launcher-handoff-color": config.accentColor,
    "--launcher-handoff-hover-color": config.accentHoverColor,
    backgroundAttachment: "fixed, fixed",
    backgroundColor: "var(--background)",
    backgroundImage:
      "var(--launcher-stage-pattern), var(--launcher-stage-background)",
    backgroundPosition: "top left, center top",
    backgroundRepeat: "repeat, no-repeat",
    backgroundSize: "var(--ambient-page-pattern-size), 100% 100%",
  };
}

const LIGHT_LAUNCHER_SURFACE_THEME_STYLE = buildLauncherSurfaceThemeStyle({
  accentColor: "rgba(126, 34, 206, 0.52)",
  accentHoverColor: "rgba(126, 34, 206, 0.82)",
  dividerColor: "rgba(83, 88, 101, 0.10)",
  heroAura:
    "radial-gradient(30% 16% at 50% 82%, rgba(133, 119, 255, 0.22), rgba(133, 119, 255, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(118, 231, 206, 0.12), rgba(118, 231, 206, 0) 76%), radial-gradient(14% 18% at 16% 34%, rgba(191, 219, 254, 0.12), rgba(191, 219, 254, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(255, 255, 255, 0.10), rgba(255, 255, 255, 0) 74%)",
  heroInnerFill: "rgba(216, 226, 247, 0.10)",
  heroInnerStroke: "rgba(255, 255, 255, 0.14)",
  heroStop1: "rgba(236, 242, 255, 0.28)",
  heroStop2: "rgba(221, 231, 248, 0.22)",
  heroStop3: "rgba(211, 222, 241, 0.26)",
  heroStroke: "rgba(255, 255, 255, 0.34)",
  heroTint1: "transparent",
  heroTint2: "transparent",
  heroTint3: "transparent",
  heroTint4: "rgba(255, 255, 255, 0)",
  inputFill: "rgba(255, 255, 255, 0.08)",
  inputIcon: "rgba(76, 82, 96, 0.72)",
  inputInnerFill: "rgba(255, 255, 255, 0.04)",
  inputInnerStroke: "rgba(255, 255, 255, 0.08)",
  inputPlaceholder: "rgba(76, 87, 109, 0.84)",
  inputStroke: "rgba(255, 255, 255, 0.32)",
  inputText: "rgba(28, 31, 39, 0.92)",
  metaText: "rgba(74, 80, 94, 0.76)",
  submitBackground:
    "linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(244, 248, 253, 0.92))",
  submitBorder: "rgba(255,255,255,0.34)",
  submitColor: "#182131",
  submitShadow: "0 10px 20px rgba(110, 117, 142, 0.14)",
});

const DARK_LAUNCHER_SURFACE_THEME_STYLE = buildLauncherSurfaceThemeStyle({
  accentColor: "rgba(154, 187, 255, 0.72)",
  accentHoverColor: "rgba(190, 210, 255, 0.96)",
  dividerColor: "rgba(255, 255, 255, 0.08)",
  heroAura:
    "radial-gradient(30% 16% at 50% 82%, rgba(118, 169, 255, 0.24), rgba(118, 169, 255, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(117, 218, 195, 0.14), rgba(117, 218, 195, 0) 76%), radial-gradient(12% 18% at 14% 38%, rgba(243, 184, 109, 0.16), rgba(243, 184, 109, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0) 74%)",
  heroInnerFill: "rgba(255, 255, 255, 0.04)",
  heroInnerStroke: "rgba(255, 255, 255, 0.10)",
  heroStop1: "rgba(255, 255, 255, 0.08)",
  heroStop2: "rgba(229, 222, 209, 0.10)",
  heroStop3: "rgba(229, 222, 209, 0.14)",
  heroStroke: "rgba(255, 255, 255, 0.14)",
  heroTint1: "transparent",
  heroTint2: "transparent",
  heroTint3: "transparent",
  heroTint4: "rgba(255, 255, 255, 0)",
  inputFill: "rgba(255, 255, 255, 0.06)",
  inputIcon: "rgba(204, 208, 218, 0.70)",
  inputInnerFill: "rgba(255, 255, 255, 0.03)",
  inputInnerStroke: "rgba(255, 255, 255, 0.09)",
  inputPlaceholder: "rgba(209, 199, 183, 0.42)",
  inputStroke: "rgba(255, 255, 255, 0.20)",
  inputText: "rgba(244, 245, 248, 0.94)",
  metaText: "rgba(186, 190, 200, 0.74)",
  submitBackground:
    "linear-gradient(180deg, rgba(255, 255, 255, 0.18), rgba(255, 255, 255, 0.12))",
  submitBorder: "rgba(255,255,255,0.34)",
  submitColor: "#f5ecde",
  submitShadow: "0 10px 24px rgba(0, 0, 0, 0.24)",
});

const RAIN_LAUNCHER_SURFACE_THEME_STYLE = buildLauncherSurfaceThemeStyle({
  accentColor: "rgba(138, 168, 212, 0.7)",
  accentHoverColor: "rgba(180, 206, 238, 0.96)",
  dividerColor: "rgba(73, 88, 111, 0.10)",
  heroAura:
    "radial-gradient(30% 16% at 50% 82%, rgba(100, 140, 190, 0.22), rgba(100, 140, 190, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(90, 160, 180, 0.12), rgba(90, 160, 180, 0) 76%), radial-gradient(14% 18% at 16% 34%, rgba(120, 148, 180, 0.12), rgba(120, 148, 180, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(160, 185, 220, 0.10), rgba(160, 185, 220, 0) 74%)",
  heroInnerFill: "rgba(138, 168, 212, 0.08)",
  heroInnerStroke: "rgba(160, 185, 220, 0.14)",
  heroStop1: "rgba(255, 255, 255, 0.14)",
  heroStop2: "rgba(138, 168, 212, 0.18)",
  heroStop3: "rgba(100, 138, 180, 0.22)",
  heroStroke: "rgba(160, 185, 220, 0.24)",
  heroTint1: "transparent",
  heroTint2: "transparent",
  heroTint3: "transparent",
  heroTint4: "rgba(255, 255, 255, 0)",
  inputFill: "rgba(138, 168, 212, 0.07)",
  inputIcon: "rgba(84, 100, 124, 0.70)",
  inputInnerFill: "rgba(138, 168, 212, 0.03)",
  inputInnerStroke: "rgba(160, 185, 220, 0.07)",
  inputPlaceholder: "rgba(180, 198, 220, 0.42)",
  inputStroke: "rgba(160, 185, 220, 0.22)",
  inputText: "rgba(35, 44, 58, 0.92)",
  metaText: "rgba(78, 93, 115, 0.76)",
  submitBackground:
    "linear-gradient(180deg, rgba(255, 255, 255, 0.14), rgba(138, 168, 212, 0.18))",
  submitBorder: "rgba(255,255,255,0.28)",
  submitColor: "#d4e2f2",
  submitShadow: "0 10px 24px rgba(0, 0, 0, 0.28)",
});

const LAUNCHER_SURFACE_THEME_STYLE_MAP: Record<
  Theme,
  LauncherSurfaceThemeStyle
> = {
  dark: DARK_LAUNCHER_SURFACE_THEME_STYLE,
  light: LIGHT_LAUNCHER_SURFACE_THEME_STYLE,
  rain: RAIN_LAUNCHER_SURFACE_THEME_STYLE,
  sunny: LIGHT_LAUNCHER_SURFACE_THEME_STYLE,
};

export function getLauncherSurfaceThemeStyle(
  theme: Theme,
): LauncherSurfaceThemeStyle {
  return LAUNCHER_SURFACE_THEME_STYLE_MAP[theme];
}
