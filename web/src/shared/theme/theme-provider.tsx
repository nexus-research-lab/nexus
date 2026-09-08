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
  applyTheme,
  detectInitialTheme,
  THEME_CONTEXT,
  Theme,
  ThemeContextValue,
  THEME_STORAGE_KEY,
} from "./theme-context";
import { applyChatTypography, syncChatTypography, useChatTypography } from "./chat-typography";
import { ThemeOverlay } from "./theme-overlay";

export function ThemeProvider({ children }: { children: ReactNode }) {
  const typography = useChatTypography((state) => state.typography);
  useEffect(() => { applyChatTypography(typography); }, [typography]);
  useEffect(() => {
    window.addEventListener("storage", syncChatTypography);
    return () => window.removeEventListener("storage", syncChatTypography);
  }, []);
  const [theme, setTheme] = useState<Theme>(detectInitialTheme);

  useEffect(() => {
    applyTheme(theme);
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  }, [theme]);

  const value: ThemeContextValue = {
    theme,
    setTheme,
  };

  return (
    <THEME_CONTEXT.Provider value={value}>
      {children}
      <ThemeOverlay />
    </THEME_CONTEXT.Provider>
  );
}
