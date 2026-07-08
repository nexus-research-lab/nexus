/**
 * =====================================================
 * @File   : messages.ts
 * @Date   : 2026-04-04 17:05
 * @Author : leemysw
 * 2026-04-04 17:05   Create
 * =====================================================
 */

import { enMessages } from "./messages.en";
import { zhMessages } from "./messages.zh";
import type { TranslationKey } from "./messages.zh";

export type Locale = "zh" | "en";
export type { TranslationKey } from "./messages.zh";

export const DEFAULT_LOCALE: Locale = "zh";
export const LOCALE_STORAGE_KEY = "nexus-locale";

export const MESSAGES: Record<Locale, Record<TranslationKey, string>> = {
  zh: zhMessages,
  en: enMessages,
};
