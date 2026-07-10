import { useCallback, useEffect, useRef, useState } from "react";

import { setUserPreferences } from "@/config/options";
import {
  getUserPreferencesApi,
  updateUserPreferencesApi,
} from "@/lib/api/settings-preferences-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { UserPreferences } from "@/types/settings/preferences";

import {
  type PreferenceFeedback,
  buildPreferencesUpdatePayload,
  normalizePreferences,
} from "./settings-preferences-model";

type PreferenceMutation = (current: UserPreferences) => UserPreferences;

export function useUserPreferences() {
  const { t } = useI18n();
  const [preferences, setPreferences] = useState<UserPreferences>(() =>
    normalizePreferences(null),
  );
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<PreferenceFeedback | null>(null);
  const preferencesRef = useRef(preferences);
  const lastSavedRef = useRef<UserPreferences | null>(null);
  const savingRef = useRef(false);

  const replacePreferences = useCallback((next: UserPreferences) => {
    const normalized = normalizePreferences(next);
    preferencesRef.current = normalized;
    setPreferences(normalized);
    setUserPreferences(normalized);
    return normalized;
  }, []);

  useEffect(() => {
    let cancelled = false;
    void getUserPreferencesApi()
      .then((result) => {
        if (cancelled) {
          return;
        }
        const normalized = replacePreferences(result);
        lastSavedRef.current = normalized;
        setFeedback(null);
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setFeedback({
            message: error instanceof Error
              ? error.message
              : t("settings.general.preferences_load_failed"),
          });
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [replacePreferences, t]);

  const persistPreferences = useCallback(async (next: UserPreferences) => {
    if (savingRef.current) {
      return null;
    }
    savingRef.current = true;
    const optimistic = replacePreferences(next);
    setFeedback(null);
    setSaving(true);

    try {
      const result = await updateUserPreferencesApi(
        buildPreferencesUpdatePayload(optimistic),
      );
      const saved = replacePreferences(result);
      lastSavedRef.current = saved;
      return saved;
    } catch (error) {
      if (lastSavedRef.current) {
        replacePreferences(lastSavedRef.current);
      }
      const normalizedError = error instanceof Error
        ? error
        : new Error(t("settings.general.preferences_save_failed"));
      setFeedback({ message: normalizedError.message });
      throw normalizedError;
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }, [replacePreferences, t]);

  const updatePreferences = useCallback((mutate: PreferenceMutation) => {
    void persistPreferences(mutate(preferencesRef.current)).catch(() => {});
  }, [persistPreferences]);

  const getCurrentPreferences = useCallback(
    () => preferencesRef.current,
    [],
  );

  return {
    feedback,
    getCurrentPreferences,
    loading,
    persistPreferences,
    preferences,
    saving,
    setFeedback,
    updatePreferences,
  };
}
