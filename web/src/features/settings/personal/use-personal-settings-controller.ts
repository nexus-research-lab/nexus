import { useCallback, useEffect, useMemo, useState } from "react";

import {
  changePasswordApi,
  getPersonalProfileApi,
  type PersonalProfile,
  updatePersonalProfileApi,
} from "@/lib/api/account/auth-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  EMPTY_PASSWORD_DRAFT,
  getPasswordValidationKey,
  hasPasswordDraftInput,
  updatePasswordDraft,
  type PasswordDraft,
  type PasswordField,
  type PersonalSettingsFeedback,
} from "./personal-settings-model";

export function usePersonalSettingsController() {
  const { t } = useI18n();
  const { refreshStatus } = useAuth();
  const [profile, setProfile] = useState<PersonalProfile | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [passwordDraft, setPasswordDraft] = useState<PasswordDraft>(EMPTY_PASSWORD_DRAFT);
  const [isSubmittingPassword, setIsSubmittingPassword] = useState(false);
  const [isSavingAvatar, setIsSavingAvatar] = useState(false);
  const [feedback, setFeedback] = useState<PersonalSettingsFeedback | null>(null);
  const validationKey = useMemo(() => getPasswordValidationKey({
    canChangePassword: Boolean(profile?.can_change_password),
    draft: passwordDraft,
  }), [passwordDraft, profile?.can_change_password]);
  const validationError = validationKey ? t(validationKey) : null;

  useEffect(() => {
    let isCurrent = true;
    setIsLoading(true);
    void getPersonalProfileApi()
      .then((result) => {
        if (!isCurrent) {
          return;
        }
        setProfile(result);
        setFeedback((current) => current?.tone === "error" ? null : current);
      })
      .catch((error) => {
        if (!isCurrent) {
          return;
        }
        setFeedback({
          message: toErrorMessage(error, t("settings.personal.load_failed_message")),
          title: t("settings.personal.load_failed_title"),
          tone: "error",
        });
      })
      .finally(() => {
        if (isCurrent) {
          setIsLoading(false);
        }
      });
    return () => {
      isCurrent = false;
    };
  }, [t]);

  const saveAvatar = useCallback(async (nextAvatar: string) => {
    if (
      !profile?.can_update_profile ||
      isSavingAvatar ||
      nextAvatar === (profile.user.avatar ?? "")
    ) {
      return;
    }
    setIsSavingAvatar(true);
    try {
      const result = await updatePersonalProfileApi({ avatar: nextAvatar });
      setProfile(result);
      await refreshStatus();
      setFeedback({
        message: t("settings.personal.avatar_save_success_message"),
        title: t("settings.personal.profile_save_success_title"),
        tone: "success",
      });
    } catch (error) {
      setFeedback({
        message: toErrorMessage(error, t("settings.personal.avatar_save_failed_message")),
        title: t("settings.personal.profile_save_failed_title"),
        tone: "error",
      });
    } finally {
      setIsSavingAvatar(false);
    }
  }, [isSavingAvatar, profile, refreshStatus, t]);

  const submitPassword = useCallback(async () => {
    if (validationError || isSubmittingPassword) {
      if (validationError) {
        setFeedback({
          message: validationError,
          title: t("settings.personal.save_failed_title"),
          tone: "error",
        });
      }
      return;
    }

    setIsSubmittingPassword(true);
    try {
      await changePasswordApi({
        current_password: passwordDraft.currentPassword,
        new_password: passwordDraft.newPassword,
      });
      await refreshStatus();
      setPasswordDraft(EMPTY_PASSWORD_DRAFT);
      setFeedback({
        message: t("settings.personal.save_success_message"),
        title: t("settings.personal.save_success_title"),
        tone: "success",
      });
    } catch (error) {
      setFeedback({
        message: toErrorMessage(error, t("settings.personal.save_failed_message")),
        title: t("settings.personal.save_failed_title"),
        tone: "error",
      });
    } finally {
      setIsSubmittingPassword(false);
    }
  }, [isSubmittingPassword, passwordDraft, refreshStatus, t, validationError]);

  const setPasswordField = useCallback((field: PasswordField, value: string) => {
    setPasswordDraft((current) => updatePasswordDraft(current, field, value));
  }, []);

  return {
    avatar: {
      canUpdate: Boolean(profile?.can_update_profile) && !isSavingAvatar,
      isSaving: isSavingAvatar,
      save: saveAvatar,
      value: profile?.user.avatar ?? "",
    },
    feedback: {
      dismiss: () => setFeedback(null),
      value: feedback,
    },
    password: {
      canChange: Boolean(profile?.can_change_password),
      canSubmit: !validationError && !isSubmittingPassword && !isLoading,
      draft: passwordDraft,
      hasInput: hasPasswordDraftInput(passwordDraft),
      isSubmitting: isSubmittingPassword,
      setField: setPasswordField,
      submit: submitPassword,
      validationError,
    },
    profile: {
      isLoading,
      value: profile,
    },
  };
}

function toErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}
