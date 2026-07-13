"use client";

import { createContext, useContext } from "react";

import type { Locale, TranslationKey } from "./messages";

export interface TranslateParams {
  [key: string]: string | number;
}

export interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey, params?: TranslateParams) => string;
}

export const I18N_CONTEXT = createContext<I18nContextValue | null>(null);

export function useI18n() {
  const context = useContext(I18N_CONTEXT);

  if (!context) {
    throw new Error("useI18n must be used within I18nProvider.");
  }

  return context;
}
