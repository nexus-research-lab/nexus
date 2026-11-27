// INPUT: 当前主题、聊天排版、侧栏天空偏好及跨窗口存储事件。
// OUTPUT: 应用到文档的主题与排版；持久化不可用时保持实时偏好。
// POS: 主题 Context 与文档同步 owner，不拥有业务设置事务。

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
import { syncSidebarSkyPreference } from "./sidebar-sky-preference";
import { writeThemePreference } from "./theme-storage";
import { ThemeOverlay } from "./theme-overlay";

export function ThemeProvider({ children }: { children: ReactNode }) {
  const typography = useChatTypography((state) => state.typography);
  useEffect(() => { applyChatTypography(typography); }, [typography]);
  useEffect(() => {
    window.addEventListener("storage", syncChatTypography);
    window.addEventListener("storage", syncSidebarSkyPreference);
    return () => {
      window.removeEventListener("storage", syncChatTypography);
      window.removeEventListener("storage", syncSidebarSkyPreference);
    };
  }, []);
  const [theme, setTheme] = useState<Theme>(detectInitialTheme);

  useEffect(() => {
    applyTheme(theme);
    writeThemePreference(THEME_STORAGE_KEY, theme);
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
