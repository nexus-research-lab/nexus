/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：launcher-surface-theme.ts
# @Date   ：2026-04-12 19:42
# @Author ：leemysw
# 2026-04-12 19:42   Create
# =====================================================
*/

import { CSSProperties } from "react";

import { Theme } from "@/shared/theme/theme-context";

type LauncherSurfaceThemeStyle = CSSProperties & Record<`--launcher-${string}`, string>;

interface LauncherSurfaceConfig {
  accent_color: string;
  accent_hover_color: string;
  divider_color: string;
  hero_aura: string;
  hero_inner_fill: string;
  hero_inner_stroke: string;
  hero_stop_1: string;
  hero_stop_2: string;
  hero_stop_3: string;
  hero_stroke: string;
  hero_tint_1: string;
  hero_tint_2: string;
  hero_tint_3: string;
  hero_tint_4: string;
  input_fill: string;
  input_icon: string;
  input_inner_fill: string;
  input_inner_stroke: string;
  input_placeholder: string;
  input_stroke: string;
  input_text: string;
  meta_text: string;
  submit_background: string;
  submit_border: string;
  submit_color: string;
  submit_shadow: string;
}

function build_launcher_surface_theme_style(
  config: LauncherSurfaceConfig,
): LauncherSurfaceThemeStyle {
  return {
    "--launcher-stage-background": "var(--ambient-page-background)",
    "--launcher-stage-pattern": "var(--ambient-page-pattern)",
    "--launcher-divider-color": config.divider_color,
    "--launcher-hero-aura": config.hero_aura,
    "--launcher-hero-stop-1": config.hero_stop_1,
    "--launcher-hero-stop-2": config.hero_stop_2,
    "--launcher-hero-stop-3": config.hero_stop_3,
    "--launcher-hero-stroke": config.hero_stroke,
    "--launcher-hero-inner-fill": config.hero_inner_fill,
    "--launcher-hero-inner-stroke": config.hero_inner_stroke,
    "--launcher-hero-tint-1": config.hero_tint_1,
    "--launcher-hero-tint-2": config.hero_tint_2,
    "--launcher-hero-tint-3": config.hero_tint_3,
    "--launcher-hero-tint-4": config.hero_tint_4,
    "--launcher-input-fill": config.input_fill,
    "--launcher-input-stroke": config.input_stroke,
    "--launcher-input-inner-fill": config.input_inner_fill,
    "--launcher-input-inner-stroke": config.input_inner_stroke,
    "--launcher-input-icon": config.input_icon,
    "--launcher-input-text": config.input_text,
    "--launcher-input-placeholder": config.input_placeholder,
    "--launcher-submit-background": config.submit_background,
    "--launcher-submit-border": config.submit_border,
    "--launcher-submit-color": config.submit_color,
    "--launcher-submit-shadow": config.submit_shadow,
    "--launcher-meta-text": config.meta_text,
    "--launcher-handoff-color": config.accent_color,
    "--launcher-handoff-hover-color": config.accent_hover_color,
    backgroundAttachment: "fixed, fixed",
    backgroundColor: "var(--background)",
    backgroundImage: "var(--launcher-stage-pattern), var(--launcher-stage-background)",
    backgroundPosition: "top left, center top",
    backgroundRepeat: "repeat, no-repeat",
    backgroundSize: "var(--ambient-page-pattern-size), 100% 100%",
  };
}

const LIGHT_LAUNCHER_SURFACE_THEME_STYLE = build_launcher_surface_theme_style({
  accent_color: "rgba(126, 34, 206, 0.52)",
  accent_hover_color: "rgba(126, 34, 206, 0.82)",
  divider_color: "rgba(83, 88, 101, 0.10)",
  hero_aura: "radial-gradient(30% 16% at 50% 82%, rgba(133, 119, 255, 0.22), rgba(133, 119, 255, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(118, 231, 206, 0.12), rgba(118, 231, 206, 0) 76%), radial-gradient(14% 18% at 16% 34%, rgba(191, 219, 254, 0.12), rgba(191, 219, 254, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(255, 255, 255, 0.10), rgba(255, 255, 255, 0) 74%)",
  hero_inner_fill: "rgba(216, 226, 247, 0.10)",
  hero_inner_stroke: "rgba(255, 255, 255, 0.14)",
  hero_stop_1: "rgba(236, 242, 255, 0.28)",
  hero_stop_2: "rgba(221, 231, 248, 0.22)",
  hero_stop_3: "rgba(211, 222, 241, 0.26)",
  hero_stroke: "rgba(255, 255, 255, 0.34)",
  hero_tint_1: "transparent",
  hero_tint_2: "transparent",
  hero_tint_3: "transparent",
  hero_tint_4: "rgba(255, 255, 255, 0)",
  input_fill: "rgba(255, 255, 255, 0.08)",
  input_icon: "rgba(76, 82, 96, 0.72)",
  input_inner_fill: "rgba(255, 255, 255, 0.04)",
  input_inner_stroke: "rgba(255, 255, 255, 0.08)",
  input_placeholder: "rgba(76, 87, 109, 0.84)",
  input_stroke: "rgba(255, 255, 255, 0.32)",
  input_text: "rgba(28, 31, 39, 0.92)",
  meta_text: "rgba(74, 80, 94, 0.76)",
  submit_background: "linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(244, 248, 253, 0.92))",
  submit_border: "rgba(255,255,255,0.34)",
  submit_color: "#182131",
  submit_shadow: "0 10px 20px rgba(110, 117, 142, 0.14)",
});

const DARK_LAUNCHER_SURFACE_THEME_STYLE = build_launcher_surface_theme_style({
  accent_color: "rgba(154, 187, 255, 0.72)",
  accent_hover_color: "rgba(190, 210, 255, 0.96)",
  divider_color: "rgba(255, 255, 255, 0.08)",
  hero_aura: "radial-gradient(30% 16% at 50% 82%, rgba(118, 169, 255, 0.24), rgba(118, 169, 255, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(117, 218, 195, 0.14), rgba(117, 218, 195, 0) 76%), radial-gradient(12% 18% at 14% 38%, rgba(243, 184, 109, 0.16), rgba(243, 184, 109, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0) 74%)",
  hero_inner_fill: "rgba(255, 255, 255, 0.04)",
  hero_inner_stroke: "rgba(255, 255, 255, 0.10)",
  hero_stop_1: "rgba(255, 255, 255, 0.08)",
  hero_stop_2: "rgba(229, 222, 209, 0.10)",
  hero_stop_3: "rgba(229, 222, 209, 0.14)",
  hero_stroke: "rgba(255, 255, 255, 0.14)",
  hero_tint_1: "transparent",
  hero_tint_2: "transparent",
  hero_tint_3: "transparent",
  hero_tint_4: "rgba(255, 255, 255, 0)",
  input_fill: "rgba(255, 255, 255, 0.06)",
  input_icon: "rgba(204, 208, 218, 0.70)",
  input_inner_fill: "rgba(255, 255, 255, 0.03)",
  input_inner_stroke: "rgba(255, 255, 255, 0.09)",
  input_placeholder: "rgba(209, 199, 183, 0.42)",
  input_stroke: "rgba(255, 255, 255, 0.20)",
  input_text: "rgba(244, 245, 248, 0.94)",
  meta_text: "rgba(186, 190, 200, 0.74)",
  submit_background: "linear-gradient(180deg, rgba(255, 255, 255, 0.18), rgba(255, 255, 255, 0.12))",
  submit_border: "rgba(255,255,255,0.34)",
  submit_color: "#f5ecde",
  submit_shadow: "0 10px 24px rgba(0, 0, 0, 0.24)",
});

const RAIN_LAUNCHER_SURFACE_THEME_STYLE = build_launcher_surface_theme_style({
  accent_color: "rgba(138, 168, 212, 0.7)",
  accent_hover_color: "rgba(180, 206, 238, 0.96)",
  divider_color: "rgba(73, 88, 111, 0.10)",
  hero_aura: "radial-gradient(30% 16% at 50% 82%, rgba(100, 140, 190, 0.22), rgba(100, 140, 190, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(90, 160, 180, 0.12), rgba(90, 160, 180, 0) 76%), radial-gradient(14% 18% at 16% 34%, rgba(120, 148, 180, 0.12), rgba(120, 148, 180, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(160, 185, 220, 0.10), rgba(160, 185, 220, 0) 74%)",
  hero_inner_fill: "rgba(138, 168, 212, 0.08)",
  hero_inner_stroke: "rgba(160, 185, 220, 0.14)",
  hero_stop_1: "rgba(255, 255, 255, 0.14)",
  hero_stop_2: "rgba(138, 168, 212, 0.18)",
  hero_stop_3: "rgba(100, 138, 180, 0.22)",
  hero_stroke: "rgba(160, 185, 220, 0.24)",
  hero_tint_1: "transparent",
  hero_tint_2: "transparent",
  hero_tint_3: "transparent",
  hero_tint_4: "rgba(255, 255, 255, 0)",
  input_fill: "rgba(138, 168, 212, 0.07)",
  input_icon: "rgba(84, 100, 124, 0.70)",
  input_inner_fill: "rgba(138, 168, 212, 0.03)",
  input_inner_stroke: "rgba(160, 185, 220, 0.07)",
  input_placeholder: "rgba(180, 198, 220, 0.42)",
  input_stroke: "rgba(160, 185, 220, 0.22)",
  input_text: "rgba(35, 44, 58, 0.92)",
  meta_text: "rgba(78, 93, 115, 0.76)",
  submit_background: "linear-gradient(180deg, rgba(255, 255, 255, 0.14), rgba(138, 168, 212, 0.18))",
  submit_border: "rgba(255,255,255,0.28)",
  submit_color: "#d4e2f2",
  submit_shadow: "0 10px 24px rgba(0, 0, 0, 0.28)",
});

const LAUNCHER_SURFACE_THEME_STYLE_MAP: Record<Theme, LauncherSurfaceThemeStyle> = {
  dark: DARK_LAUNCHER_SURFACE_THEME_STYLE,
  light: LIGHT_LAUNCHER_SURFACE_THEME_STYLE,
  rain: RAIN_LAUNCHER_SURFACE_THEME_STYLE,
  sunny: LIGHT_LAUNCHER_SURFACE_THEME_STYLE,
};

export function get_launcher_surface_theme_style(theme: Theme): LauncherSurfaceThemeStyle {
  return LAUNCHER_SURFACE_THEME_STYLE_MAP[theme];
}
