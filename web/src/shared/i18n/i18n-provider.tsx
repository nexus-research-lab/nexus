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

function detectInitialLocale(): Locale {
  if (typeof window === "undefined") {
    return DEFAULT_LOCALE;
  }

  const savedLocale = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  if (savedLocale === "zh" || savedLocale === "en") {
    return savedLocale;
  }

  const navigatorLocale = window.navigator.language.toLowerCase();
  if (navigatorLocale.startsWith("zh")) {
    return "zh";
  }
  return "en";
}

function formatMessage(template: string, params?: TranslateParams): string {
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
  const [locale, setLocale] = useState<Locale>(detectInitialLocale);

  useEffect(() => {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
  }, [locale]);

  const value: I18nContextValue = {
    locale,
    setLocale,
    t: (key, params) => formatMessage(MESSAGES[locale][key] ?? MESSAGES[DEFAULT_LOCALE][key], params),
  };

  return (
    <I18N_CONTEXT.Provider value={value}>
      {children}
    </I18N_CONTEXT.Provider>
  );
}
