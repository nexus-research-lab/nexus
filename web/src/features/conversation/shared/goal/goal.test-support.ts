// INPUT: Explicit locale and a stable scoped Goal fixture for offline tests.
// OUTPUT: Real catalog translation and reusable Goal data without HTTP or stores.
// POS: Goal test support; production modules must not import this fixture.
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { Goal } from "@/types/conversation/goal";

export function goalTestI18n(locale: Locale): I18nContextValue {
  return {
    locale,
    setLocale: () => {},
    t: (key, params) => Object.entries(params ?? {}).reduce(
      (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)),
      MESSAGES[locale][key],
    ),
  };
}

export const ACTIVE_GOAL: Goal = {
  continuation_count: 1,
  continuation_state: "ready",
  created_at: "2026-09-04T00:00:00Z",
  empty_progress_count: 0,
  id: "goal-1",
  objective: "统一前端组件规范",
  session_key: "session-1",
  status: "active",
  token_budget: 10_000,
  updated_at: "2026-09-04T00:01:00Z",
  usage: { actual_tokens: 2_500 },
  usage_finalized: false,
  version: 1,
};
