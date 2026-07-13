import { enMessages } from "./catalog/en";
import { zhMessages } from "./catalog/zh";
import type { TranslationKey } from "./catalog/zh";

export type Locale = "zh" | "en";
export type { TranslationKey } from "./catalog/zh";

export const DEFAULT_LOCALE: Locale = "zh";
export const LOCALE_STORAGE_KEY = "nexus-locale";

export const MESSAGES: Record<Locale, Record<TranslationKey, string>> = {
  zh: zhMessages,
  en: enMessages,
};
