// INPUT: 保存的主题偏好与系统配色。
// OUTPUT: 当前主题身份、文档颜色和背景投影。
// POS: 主题定义与 Context；可选存储失败由 theme-storage 容错。

"use client";

import { createContext, useContext } from "react";
import { readThemePreference } from "./theme-storage";
import { applyThemeBackgroundPattern } from "./theme-background-pattern";

export type Theme = "light" | "dark" | "sunny" | "rain";
export type VisualTheme = "light" | "dark" | "rain";

export const THEME_STORAGE_KEY = "nexus-theme";

export interface ThemeContextValue {
  theme: Theme;
  setTheme: (theme: Theme) => void;
}

/** 中文注释：晴天主题视觉上直接复用亮色，避免维护两套几乎相同的设计令牌。 */
function resolveVisualTheme(theme: Theme): VisualTheme {
  return theme === "sunny" ? "light" : theme;
}

export function detectInitialTheme(): Theme {
  if (typeof window === "undefined") {
    return "light";
  }

  const savedTheme = readThemePreference(THEME_STORAGE_KEY);
  if (
    savedTheme === "light" ||
    savedTheme === "dark" ||
    savedTheme === "sunny" ||
    savedTheme === "rain"
  ) {
    return savedTheme;
  }

  return defaultTheme();
}

// 首次打开与一键恢复共用默认值，不读取已有的主题偏好。
export function defaultTheme(): Theme {
  return typeof window !== "undefined" && window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark" : "light";
}

export function applyTheme(theme: Theme) {
  if (typeof document === "undefined") {
    return;
  }

  const visualTheme = resolveVisualTheme(theme);

  document.documentElement.dataset.theme = visualTheme;
  applyThemeBackgroundPattern(theme, document.documentElement);
  document.documentElement.style.colorScheme =
    visualTheme === "dark" || visualTheme === "rain" ? "dark" : "light";
}

export const THEME_CONTEXT = createContext<ThemeContextValue | null>(null);

export function useTheme() {
  const context = useContext(THEME_CONTEXT);

  if (!context) {
    throw new Error("useTheme must be used within ThemeProvider.");
  }

  return context;
}
