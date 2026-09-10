/** INPUT: 本机阅读偏好。OUTPUT: 持久设置与正文 CSS 变量。POS: 聊天排版的唯一状态源。 */
import { create } from "zustand";
import { createJSONStorage, persist, type StateStorage } from "zustand/middleware";

import { readThemePreference, writeThemePreference, removeThemePreference } from "./theme-storage";

const STORAGE_KEY = "nexus-chat-typography";
export const DEFAULT_CHAT_TYPOGRAPHY = { font: "default", fontSize: 16, lineHeight: 1.65 } as const;
export const CHAT_TYPOGRAPHY_LIMITS = { fontSize: { min: 14, max: 22 }, lineHeight: { min: 1.4, max: 2 } } as const;
type ChatTypography = { font: string; fontSize: number; lineHeight: number };

const CHAT_TYPOGRAPHY_STORAGE: StateStorage = {
  getItem: readThemePreference,
  removeItem: removeThemePreference,
  setItem: writeThemePreference,
};

export function normalizeChatTypography(value: unknown): ChatTypography {
  const input = (value && typeof value === "object" ? value : {}) as Partial<ChatTypography>;
  return {
    font: typeof input.font === "string" && input.font.trim() && !/[\x00-\x1f\x7f]/.test(input.font)
      ? input.font.slice(0, 100) : "default",
    fontSize: typeof input.fontSize === "number" && Number.isFinite(input.fontSize)
      ? Math.round(Math.min(CHAT_TYPOGRAPHY_LIMITS.fontSize.max, Math.max(CHAT_TYPOGRAPHY_LIMITS.fontSize.min, input.fontSize))) : 16,
    lineHeight: typeof input.lineHeight === "number" && Number.isFinite(input.lineHeight)
      ? Math.min(CHAT_TYPOGRAPHY_LIMITS.lineHeight.max, Math.max(CHAT_TYPOGRAPHY_LIMITS.lineHeight.min, input.lineHeight)) : 1.65,
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
  const families: Record<string, string> = {
    default: "var(--font-sans)",
    system: "var(--font-sans)",
    serif: '"Songti SC", "SimSun", serif',
  };
  // 字体名作为单个 CSS 字符串使用，不能把用户输入解释为 CSS 表达式。
  const family = Object.hasOwn(families, typography.font)
    ? families[typography.font] : `${JSON.stringify(typography.font.trim())}, var(--font-sans)`;
  style.setProperty("--chat-font-family", family);
  style.setProperty("--chat-cjk-font-family", typography.font === "default" ? "var(--font-prose)" : "var(--chat-font-family, var(--font-sans))");
}

export function syncChatTypography(event: StorageEvent) {
  if (event.key === STORAGE_KEY || event.key === null) void useChatTypography.persist.rehydrate();
}
