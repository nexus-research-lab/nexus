/**
 * =====================================================
 * @File   : i18n-provider.tsx
 * @Date   : 2026-04-04 17:05
 * @Author : leemysw
 * 2026-04-04 17:05   Create
 * =====================================================
 */

"use client";

import {
  ReactNode,
  useEffect,
  useState,
} from "react";

import { I18N_CONTEXT, I18nContextValue } from "./i18n-context";
import {
  DEFAULT_LOCALE,
  LOCALE_STORAGE_KEY,
  Locale,
  MESSAGES,
} from "./messages";

interface TranslateParams {
  [key: string]: string | number;
}

function detect_initial_locale(): Locale {
  if (typeof window === "undefined") {
    return DEFAULT_LOCALE;
  }

  const saved_locale = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  if (saved_locale === "zh" || saved_locale === "en") {
    return saved_locale;
  }

  const navigator_locale = window.navigator.language.toLowerCase();
  if (navigator_locale.startsWith("zh")) {
    return "zh";
  }
  return "en";
}

function format_message(template: string, params?: TranslateParams): string {
  if (!params) {
    return template;
  }

  return template.replace(/\{(\w+)\}/g, (match, key: string) => {
    const value = params[key];
    if (value === undefined || value === null) {
      return match;
    }
    return String(value);
  });
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, set_locale] = useState<Locale>(detect_initial_locale);

  useEffect(() => {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
  }, [locale]);

  const value: I18nContextValue = {
    locale,
    set_locale,
    t: (key, params) => format_message(MESSAGES[locale][key] ?? MESSAGES[DEFAULT_LOCALE][key], params),
  };

  return (
    <I18N_CONTEXT.Provider value={value}>
      {children}
    </I18N_CONTEXT.Provider>
  );
}
