/**
 * =====================================================
 * @File   : i18n-context.ts
 * @Date   : 2026-04-04 17:22
 * @Author : leemysw
 * 2026-04-04 17:22   Create
 * =====================================================
 */

"use client";

import { createContext, useContext } from "react";

import { Locale, TranslationKey } from "./messages";

interface TranslateParams {
  [key: string]: string | number;
}

export interface I18nContextValue {
  locale: Locale;
  set_locale: (locale: Locale) => void;
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
