import { enMessages } from "./catalog/en";
import { zhMessages } from "./catalog/zh";
import type { TranslationKey } from "./catalog/zh";

import type { Locale } from "./locale-settings";
export type { Locale } from "./locale-settings";
export type { TranslationKey } from "./catalog/zh";

export { DEFAULT_LOCALE, LOCALE_STORAGE_KEY } from "./locale-settings";

export const MESSAGES: Record<Locale, Record<TranslationKey, string>> = {
  zh: zhMessages,
  en: enMessages,
};
