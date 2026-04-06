/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：launcher-surface-theme.ts
# @Date   ：2026-04-05 17:09
# @Author ：leemysw
# 2026-04-05 17:09   Create
# =====================================================
*/

import { CSSProperties } from "react";

import { Theme } from "@/shared/theme/theme-context";

type LauncherSurfaceThemeStyle = CSSProperties & Record<`--launcher-${string}`, string>;

const LIGHT_LAUNCHER_SURFACE_THEME_STYLE: LauncherSurfaceThemeStyle = {
    "--launcher-bridge-background":
      "radial-gradient(circle at 18% 50%, rgba(255, 255, 255, 0.28), transparent 34%), linear-gradient(90deg, rgba(181, 214, 255, 0.02), rgba(181, 214, 255, 0.22) 28%, rgba(181, 214, 255, 0.12) 62%, rgba(255, 255, 255, 0.03))",
    "--launcher-hero-aura":
      "radial-gradient(30% 16% at 50% 82%, rgba(133, 119, 255, 0.26), rgba(133, 119, 255, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(118, 231, 206, 0.14), rgba(118, 231, 206, 0) 76%), radial-gradient(14% 18% at 16% 34%, rgba(191, 219, 254, 0.14), rgba(191, 219, 254, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(255, 255, 255, 0.12), rgba(255, 255, 255, 0) 74%)",
    "--launcher-hero-stop-1": "rgba(236, 242, 255, 0.12)",
    "--launcher-hero-stop-2": "rgba(221, 231, 248, 0.1)",
    "--launcher-hero-stop-3": "rgba(211, 222, 241, 0.12)",
    "--launcher-hero-stroke": "rgba(255, 255, 255, 0.32)",
    "--launcher-hero-inner-fill": "rgba(216, 226, 247, 0.06)",
    "--launcher-hero-inner-stroke": "rgba(255, 255, 255, 0.08)",
    "--launcher-hero-tint-1": "transparent",
    "--launcher-hero-tint-2": "transparent",
    "--launcher-hero-tint-3": "transparent",
    "--launcher-hero-tint-4": "transparent",
    "--launcher-input-fill": "rgba(255, 255, 255, 0.08)",
    "--launcher-input-stroke": "rgba(255, 255, 255, 0.32)",
    "--launcher-input-inner-fill": "rgba(255, 255, 255, 0.04)",
    "--launcher-input-inner-stroke": "rgba(255, 255, 255, 0.08)",
    "--launcher-input-icon": "rgba(57, 70, 95, 0.66)",
    "--launcher-input-text": "rgba(24, 33, 49, 0.88)",
    "--launcher-input-placeholder": "rgba(76, 87, 109, 0.44)",
    "--launcher-submit-background": "linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(244, 248, 253, 0.92))",
    "--launcher-submit-color": "#182131",
    "--launcher-submit-shadow": "0 10px 20px rgba(110, 117, 142, 0.14)",
    "--launcher-agent-chip-background": "rgba(255, 255, 255, 0.10)",
    "--launcher-agent-chip-border": "rgba(255, 255, 255, 0.12)",
    "--launcher-agent-chip-text": "rgba(255, 255, 255, 0.84)",
    "--launcher-room-chip-background": "rgba(255, 255, 255, 0.08)",
    "--launcher-room-chip-border": "rgba(255, 255, 255, 0.10)",
    "--launcher-room-chip-text": "rgba(255, 255, 255, 0.76)",
    "--launcher-handoff-color": "rgba(126, 34, 206, 0.52)",
    "--launcher-handoff-hover-color": "rgba(126, 34, 206, 0.82)",
};

const LAUNCHER_SURFACE_THEME_STYLE_MAP: Record<Theme, LauncherSurfaceThemeStyle> = {
  light: LIGHT_LAUNCHER_SURFACE_THEME_STYLE,
  dark: {
    "--launcher-bridge-background":
      "radial-gradient(circle at 18% 50%, rgba(255, 255, 255, 0.14), transparent 34%), linear-gradient(90deg, rgba(118, 169, 255, 0.02), rgba(118, 169, 255, 0.20) 28%, rgba(243, 184, 109, 0.12) 62%, rgba(255, 255, 255, 0.02))",
    "--launcher-hero-aura":
      "radial-gradient(30% 16% at 50% 82%, rgba(118, 169, 255, 0.28), rgba(118, 169, 255, 0) 74%), radial-gradient(12% 20% at 86% 22%, rgba(117, 218, 195, 0.16), rgba(117, 218, 195, 0) 76%), radial-gradient(12% 18% at 14% 38%, rgba(243, 184, 109, 0.18), rgba(243, 184, 109, 0) 76%), radial-gradient(40% 12% at 50% 12%, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0) 74%)",
    "--launcher-hero-stop-1": "rgba(255, 255, 255, 0.08)",
    "--launcher-hero-stop-2": "rgba(229, 222, 209, 0.10)",
    "--launcher-hero-stop-3": "rgba(229, 222, 209, 0.14)",
    "--launcher-hero-stroke": "rgba(255, 255, 255, 0.20)",
    "--launcher-hero-inner-fill": "rgba(255, 255, 255, 0.04)",
    "--launcher-hero-inner-stroke": "rgba(255, 255, 255, 0.06)",
    "--launcher-hero-tint-1": "transparent",
    "--launcher-hero-tint-2": "transparent",
    "--launcher-hero-tint-3": "transparent",
    "--launcher-hero-tint-4": "transparent",
    "--launcher-input-fill": "rgba(255, 255, 255, 0.06)",
    "--launcher-input-stroke": "rgba(255, 255, 255, 0.20)",
    "--launcher-input-inner-fill": "rgba(255, 255, 255, 0.03)",
    "--launcher-input-inner-stroke": "rgba(255, 255, 255, 0.06)",
    "--launcher-input-icon": "rgba(236, 228, 214, 0.64)",
    "--launcher-input-text": "rgba(245, 238, 227, 0.92)",
    "--launcher-input-placeholder": "rgba(209, 199, 183, 0.42)",
    "--launcher-submit-background": "linear-gradient(180deg, rgba(255, 255, 255, 0.18), rgba(255, 255, 255, 0.12))",
    "--launcher-submit-color": "#f5ecde",
    "--launcher-submit-shadow": "0 10px 24px rgba(0, 0, 0, 0.24)",
    "--launcher-agent-chip-background": "rgba(255, 255, 255, 0.08)",
    "--launcher-agent-chip-border": "rgba(255, 255, 255, 0.12)",
    "--launcher-agent-chip-text": "rgba(244, 239, 230, 0.84)",
    "--launcher-room-chip-background": "rgba(255, 255, 255, 0.06)",
    "--launcher-room-chip-border": "rgba(255, 255, 255, 0.10)",
    "--launcher-room-chip-text": "rgba(225, 218, 207, 0.78)",
    "--launcher-handoff-color": "rgba(154, 187, 255, 0.72)",
    "--launcher-handoff-hover-color": "rgba(190, 210, 255, 0.96)",
  },
  sunny: LIGHT_LAUNCHER_SURFACE_THEME_STYLE,
};

export function getLauncherSurfaceThemeStyle(theme: Theme): LauncherSurfaceThemeStyle {
  return LAUNCHER_SURFACE_THEME_STYLE_MAP[theme];
}
