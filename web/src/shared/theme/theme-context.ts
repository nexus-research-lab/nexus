/**
 * =====================================================
 * @File   : theme-context.ts
 * @Date   : 2026-04-04 18:06
 * @Author : leemysw
 * 2026-04-04 18:06   Create
 * =====================================================
 */

"use client";

import { createContext, useContext } from "react";
import { apply_theme_background_pattern } from "./theme-background-pattern";

export type Theme = "light" | "dark" | "sunny" | "rain";
export type VisualTheme = "light" | "dark" | "rain";

export const THEME_STORAGE_KEY = "nexus-theme";

export interface ThemeContextValue {
  theme: Theme;
  set_theme: (theme: Theme) => void;
}

/** 中文注释：晴天主题视觉上直接复用亮色，避免维护两套几乎相同的设计令牌。 */
export function resolve_visual_theme(theme: Theme): VisualTheme {
  return theme === "sunny" ? "light" : theme;
}

export function detect_initial_theme(): Theme {
  if (typeof window === "undefined") {
    return "light";
  }

  const saved_theme = window.localStorage.getItem(THEME_STORAGE_KEY);
  if (
    saved_theme === "light" ||
    saved_theme === "dark" ||
    saved_theme === "sunny" ||
    saved_theme === "rain"
  ) {
    return saved_theme;
  }

  if (window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }

  return "light";
}

export function apply_theme(theme: Theme) {
  if (typeof document === "undefined") {
    return;
  }

  const visual_theme = resolve_visual_theme(theme);

  document.documentElement.dataset.theme = visual_theme;
  apply_theme_background_pattern(theme, document.documentElement);
  document.documentElement.style.colorScheme =
    visual_theme === "dark" || visual_theme === "rain" ? "dark" : "light";
}

export const THEME_CONTEXT = createContext<ThemeContextValue | null>(null);

export function useTheme() {
  const context = useContext(THEME_CONTEXT);

  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider.");
  }

  return context;
}
