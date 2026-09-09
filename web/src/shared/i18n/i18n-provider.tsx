"use client";

import {
  ReactNode,
  useEffect,
  useState,
} from "react";

import { I18N_CONTEXT } from "./i18n-context";
import type { I18nContextValue, TranslateParams } from "./i18n-context";
import {
  DEFAULT_LOCALE,
  MESSAGES,
} from "./messages";
import type { Locale } from "./messages";
import { detectInitialLocale, persistLocale } from "./locale-settings";

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
    persistLocale(locale);
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
