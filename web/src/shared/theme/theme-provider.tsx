/**
 * =====================================================
 * @File   : theme-provider.tsx
 * @Date   : 2026-04-04 18:06
 * @Author : leemysw
 * 2026-04-04 18:06   Create
 * =====================================================
 */

"use client";

import { ReactNode, useEffect, useState } from "react";

import {
  apply_theme,
  detect_initial_theme,
  THEME_CONTEXT,
  Theme,
  ThemeContextValue,
  THEME_STORAGE_KEY,
} from "./theme-context";
import { ThemeOverlay } from "./theme-overlay";

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, set_theme] = useState<Theme>(detect_initial_theme);

  useEffect(() => {
    apply_theme(theme);
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  }, [theme]);

  const value: ThemeContextValue = {
    theme,
    set_theme,
  };

  return (
    <THEME_CONTEXT.Provider value={value}>
      {children}
      <ThemeOverlay />
    </THEME_CONTEXT.Provider>
  );
}
