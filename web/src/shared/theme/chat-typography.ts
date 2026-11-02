/** INPUT: 本机阅读偏好。OUTPUT: 持久设置与正文 CSS 变量。POS: 聊天排版的唯一状态源。 */
import { create } from "zustand";
import { createJSONStorage, persist, type StateStorage } from "zustand/middleware";

import { readThemePreference, writeThemePreference, removeThemePreference } from "./theme-storage";

const STORAGE_KEY = "nexus-chat-typography";
export const DEFAULT_CHAT_TYPOGRAPHY = { font: "default", fontSize: 14, lineHeight: 1.7 } as const;
export const CHAT_TYPOGRAPHY_LIMITS = { fontSize: { min: 12, max: 26 }, lineHeight: { min: 1.2, max: 2.4 } } as const;
type ChatTypography = { font: string; fontSize: number; lineHeight: number };

const CHAT_TYPOGRAPHY_STORAGE: StateStorage = {
  getItem: readThemePreference,
  removeItem: removeThemePreference,
  setItem: writeThemePreference,
};

const RETIRED_FONT_PRESETS = new Set(["system", "serif"]);

export function normalizeChatTypography(value: unknown): ChatTypography {
  const input = (value && typeof value === "object" ? value : {}) as Partial<ChatTypography>;
  return {
    // 旧版 "system"/"serif" 预设已移除，统一并入 default，以免被当成自定义字体名。
    font: typeof input.font === "string" && input.font.trim() && !RETIRED_FONT_PRESETS.has(input.font) && !/[\x00-\x1f\x7f]/.test(input.font)
      ? input.font.slice(0, 100) : "default",
    fontSize: typeof input.fontSize === "number" && Number.isFinite(input.fontSize)
      ? Math.round(Math.min(CHAT_TYPOGRAPHY_LIMITS.fontSize.max, Math.max(CHAT_TYPOGRAPHY_LIMITS.fontSize.min, input.fontSize))) : DEFAULT_CHAT_TYPOGRAPHY.fontSize,
    lineHeight: typeof input.lineHeight === "number" && Number.isFinite(input.lineHeight)
      ? Math.min(CHAT_TYPOGRAPHY_LIMITS.lineHeight.max, Math.max(CHAT_TYPOGRAPHY_LIMITS.lineHeight.min, input.lineHeight)) : DEFAULT_CHAT_TYPOGRAPHY.lineHeight,
  };
}

export const useChatTypography = create<{
  typography: ChatTypography;
  setTypography: (value: Partial<ChatTypography>) => void;
}>()(persist((set) => ({
  typography: DEFAULT_CHAT_TYPOGRAPHY,
  setTypography: (value) => set((state) => ({ typography: normalizeChatTypography({ ...state.typography, ...value }) })),
}), {
  name: STORAGE_KEY,
  storage: createJSONStorage(() => CHAT_TYPOGRAPHY_STORAGE),
  partialize: (state) => ({ typography: state.typography }),
  merge: (saved, current) => ({ ...current, typography: normalizeChatTypography((saved as { typography?: unknown } | null)?.typography) }),
}));

export function applyChatTypography(typography: ChatTypography) {
  const style = document.documentElement.style;
  style.setProperty("--chat-font-size", `${typography.fontSize}px`);
  style.setProperty("--chat-line-height", String(typography.lineHeight));
  // 默认正文与界面同用系统无衬线（苹方/雅黑）；字体名作为单个 CSS 字符串使用，不能把用户输入解释为 CSS 表达式。
  const family = typography.font === "default"
    ? "var(--font-sans)" : `${JSON.stringify(typography.font.trim())}, var(--font-sans)`;
  style.setProperty("--chat-font-family", family);
}

export function syncChatTypography(event: StorageEvent) {
  if (event.key === STORAGE_KEY || event.key === null) void useChatTypography.persist.rehydrate();
}
