// INPUT: Optional browser language preference and navigator locale.
// OUTPUT: A usable locale even when browser storage is unavailable.
// POS: Provider-free locale preference; no catalog or application dependency.
export type Locale = "zh" | "en";
export const DEFAULT_LOCALE: Locale = "zh";
export const LOCALE_STORAGE_KEY = "nexus-locale";

export function detectInitialLocale(): Locale {
  if (typeof window === "undefined") return DEFAULT_LOCALE;
  try {
    const saved = window.localStorage.getItem(LOCALE_STORAGE_KEY);
    if (saved === "zh" || saved === "en") return saved;
  } catch {
    // Storage is optional; blocked storage must not prevent startup.
  }
  return window.navigator.language.toLowerCase().startsWith("zh") ? "zh" : "en";
}

export function persistLocale(locale: Locale): void {
  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    // The active language still works for this page lifetime.
  }
}
