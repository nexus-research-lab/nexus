/**
 * INPUT: 个人资料读取、头像保存、密码修改及当前认证状态刷新结果。
 * OUTPUT: 保留资料/密码草稿，未知 mutation 锁定重复提交，并提供明确的新意图入口。
 * POS: 个人设置可靠性控制器；不透传服务端或网络异常细节。
 */
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

import {
  changePasswordApi,
  getPersonalProfileApi,
  type PersonalProfile,
  updatePersonalProfileApi,
} from "@/lib/api/account/auth-api";
import {
  projectMutationFailure,
  type MutationFailure,
} from "@/lib/error-message";
import { useAuth } from "@/shared/auth/auth-context";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
  subscribeAuthOwnerScopeGeneration,
} from "@/shared/auth/auth-owner-generation";
import { useI18n } from "@/shared/i18n/i18n-context";

import {
  EMPTY_PASSWORD_DRAFT,
  buildPersonalControllerPresentation,
  hasPasswordDraftInput,
  updatePasswordDraft,
  type PasswordDraft,
  type PasswordField,
  type PersonalSettingsFeedback,
} from "./personal-settings-model";

export function usePersonalSettingsController() {
  const { t } = useI18n();
  const { refreshStatus } = useAuth();
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
    captureAuthOwnerScopeGeneration,
  );
  const [profile, setProfile] = useState<PersonalProfile | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [passwordDraft, setPasswordDraft] = useState<PasswordDraft>(EMPTY_PASSWORD_DRAFT);
  const [isSubmittingPassword, setIsSubmittingPassword] = useState(false);
  const [isSavingAvatar, setIsSavingAvatar] = useState(false);
  const [avatarMutationBlocked, setAvatarMutationBlocked] = useState(false);
  const [passwordMutationBlocked, setPasswordMutationBlocked] = useState(false);
  const [feedback, setFeedback] = useState<PersonalSettingsFeedback | null>(null);
  const profileLoadingRef = useRef(false);
  const profileRequestRef = useRef(0);
  const profileOwnerGenerationRef = useRef(ownerGeneration);
  const reloadProfileRef = useRef<() => void>(() => {});
  const scopeCurrent = profileOwnerGenerationRef.current === ownerGeneration;
  const scopedProfile = scopeCurrent ? profile : null;
  const scopedPasswordDraft = scopeCurrent
    ? passwordDraft
    : EMPTY_PASSWORD_DRAFT;
  const presentation = useMemo(() => buildPersonalControllerPresentation(
    scopedProfile,
    scopedPasswordDraft,
    isLoading,
    isSavingAvatar,
    isSubmittingPassword,
    t,
  ), [
    isLoading,
    isSavingAvatar,
    isSubmittingPassword,
    scopedPasswordDraft,
    scopedProfile,
    t,
  ]);

  const loadProfile = useCallback(() => {
    if (profileLoadingRef.current) {
      return;
    }
    profileLoadingRef.current = true;
    if (profileOwnerGenerationRef.current !== ownerGeneration) {
      profileOwnerGenerationRef.current = ownerGeneration;
      setProfile(null);
      setPasswordDraft(EMPTY_PASSWORD_DRAFT);
      setAvatarMutationBlocked(false);
      setPasswordMutationBlocked(false);
      setFeedback(null);
      setIsSavingAvatar(false);
      setIsSubmittingPassword(false);
    }
    const requestId = profileRequestRef.current + 1;
    profileRequestRef.current = requestId;
    setIsLoading(true);
    void getPersonalProfileApi()
      .then((result) => {
        if (
          profileRequestRef.current !== requestId
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        setProfile(result);
        setFeedback((current) => current?.tone === "error" ? null : current);
      })
      .catch(() => {
        if (
          profileRequestRef.current !== requestId
          || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          return;
        }
        setFeedback({
          action: {
            label: t("settings.personal.load_retry"),
            onClick: () => reloadProfileRef.current(),
          },
          impact: t("settings.personal.load_failed_impact"),
          message: t("settings.personal.load_failed_message"),
          nextStep: t("settings.personal.load_failed_next_step"),
          title: t("settings.personal.load_failed_title"),
          tone: "error",
        });
      })
      .finally(() => {
        if (
          profileRequestRef.current === requestId
          && isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        ) {
          profileLoadingRef.current = false;
          setIsLoading(false);
        }
      });
  }, [ownerGeneration, t]);
  reloadProfileRef.current = loadProfile;

  useEffect(() => {
    loadProfile();
    return () => {
      profileRequestRef.current += 1;
      profileLoadingRef.current = false;
    };
  }, [loadProfile]);

  const startNewAvatarIntent = useCallback(() => {
    setAvatarMutationBlocked(false);
    setFeedback(null);
  }, []);

  const startNewPasswordIntent = useCallback(() => {
    setPasswordDraft(EMPTY_PASSWORD_DRAFT);
    setPasswordMutationBlocked(false);
    setFeedback(null);
  }, []);

  const saveAvatar = useCallback(async (nextAvatar: string) => {
    if (
      !scopeCurrent ||
      !isAuthOwnerScopeGenerationCurrent(ownerGeneration) ||
      !profile?.can_update_profile ||
      avatarMutationBlocked ||
      isSavingAvatar ||
      nextAvatar === (profile.user.avatar ?? "")
    ) {
      return;
    }
    setIsSavingAvatar(true);
    setFeedback(null);
    let result: PersonalProfile;
    try {
      result = await updatePersonalProfileApi({ avatar: nextAvatar });
    } catch (error) {
      if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        return;
      }
      const failure = projectMutationFailure(
        error,
        t("settings.personal.avatar_save_failed_message"),
      );
      if (failure.effect === "committed") {
        setProfile((current) => current
          ? { ...current, user: { ...current.user, avatar: nextAvatar } }
          : current);
      }
      const blocked = failure.effect !== "not_applied"
        && failure.effect !== "committed";
      setAvatarMutationBlocked(blocked);
      setFeedback(withNewIntentAction(
        buildPersonalMutationFailure(failure, "avatar", t),
        blocked ? {
          label: t("settings.personal.avatar_new_intent"),
          onClick: startNewAvatarIntent,
        } : undefined,
      ));
      setIsSavingAvatar(false);
      return;
    }
    if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
      return;
    }
    setProfile(result);
    setAvatarMutationBlocked(false);
    try {
      await refreshStatus();
      setFeedback({
        message: t("settings.personal.avatar_save_success_message"),
        title: t("settings.personal.profile_save_success_title"),
        tone: "success",
      });
    } catch {
      if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        return;
      }
      setFeedback({
        impact: t("state.committed_refresh_impact"),
        message: t("settings.personal.profile_refresh_failed_message"),
        nextStep: t("state.committed_refresh_next_step"),
        title: t("settings.personal.profile_refresh_failed_title"),
        tone: "warning",
      });
    } finally {
      if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setIsSavingAvatar(false);
      }
    }
  }, [
    avatarMutationBlocked,
    isSavingAvatar,
    profile,
    refreshStatus,
    ownerGeneration,
    scopeCurrent,
    startNewAvatarIntent,
    t,
  ]);

  const submitPassword = useCallback(async () => {
    if (
      !scopeCurrent
      || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
      || passwordMutationBlocked
      || presentation.validationError
      || isSubmittingPassword
    ) {
      if (presentation.validationError) {
        setFeedback({
          impact: t("state.validation_failure_impact"),
          message: presentation.validationError,
          nextStep: t("state.validation_failure_next_step"),
          title: t("settings.personal.save_failed_title"),
          tone: "error",
        });
      }
      return;
    }

    setIsSubmittingPassword(true);
    setFeedback(null);
    try {
      await changePasswordApi({
        current_password: scopedPasswordDraft.currentPassword,
        new_password: scopedPasswordDraft.newPassword,
      });
    } catch (error) {
      if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        return;
      }
      const failure = projectMutationFailure(
        error,
        t("settings.personal.save_failed_message"),
      );
      const blocked = failure.effect !== "not_applied"
        && failure.effect !== "committed";
      setPasswordMutationBlocked(blocked);
      if (failure.effect === "committed") {
        setPasswordDraft(EMPTY_PASSWORD_DRAFT);
      }
      setFeedback(withNewIntentAction(
        buildPersonalMutationFailure(failure, "password", t),
        blocked ? {
          label: t("settings.personal.password_new_intent"),
          onClick: startNewPasswordIntent,
        } : undefined,
      ));
      setIsSubmittingPassword(false);
      return;
    }
    if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
      return;
    }
    setPasswordDraft(EMPTY_PASSWORD_DRAFT);
    setPasswordMutationBlocked(false);
    try {
      await refreshStatus();
      setFeedback({
        message: t("settings.personal.save_success_message"),
        title: t("settings.personal.save_success_title"),
        tone: "success",
      });
    } catch {
      if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        return;
      }
      setFeedback({
        impact: t("settings.personal.password_committed_impact"),
        message: t("settings.personal.password_refresh_failed_message"),
        nextStep: t("settings.personal.password_committed_next_step"),
        title: t("settings.personal.password_refresh_failed_title"),
        tone: "warning",
      });
    } finally {
      if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setIsSubmittingPassword(false);
      }
    }
  }, [
    isSubmittingPassword,
    ownerGeneration,
    passwordMutationBlocked,
    presentation.validationError,
    refreshStatus,
    scopeCurrent,
    scopedPasswordDraft,
    startNewPasswordIntent,
    t,
  ]);

  const setPasswordField = useCallback((field: PasswordField, value: string) => {
    setPasswordDraft((current) => updatePasswordDraft(current, field, value));
  }, []);

  return {
    avatar: {
      canUpdate: presentation.avatarCanUpdate && !avatarMutationBlocked,
      isSaving: isSavingAvatar,
      save: saveAvatar,
      value: presentation.avatarValue,
    },
    feedback: {
      dismiss: () => setFeedback(null),
      value: scopeCurrent ? feedback : null,
    },
    password: {
      canChange: presentation.canChangePassword,
      canSubmit: presentation.canSubmitPassword && !passwordMutationBlocked,
      draft: scopedPasswordDraft,
      hasInput: hasPasswordDraftInput(scopedPasswordDraft),
      isSubmitting: isSubmittingPassword,
      mutationBlocked: passwordMutationBlocked,
      setField: setPasswordField,
      submit: submitPassword,
      validationError: presentation.validationError,
    },
    profile: {
      isLoading: isLoading || !scopeCurrent,
      value: scopedProfile,
    },
  };
}

type PersonalMutationKind = "avatar" | "password";
type Translate = ReturnType<typeof useI18n>["t"];

export function buildPersonalMutationFailure(
  failure: MutationFailure,
  kind: PersonalMutationKind,
  t: Translate,
): PersonalSettingsFeedback {
  if (failure.effect === "not_applied") {
    return {
      impact: t(kind === "password"
        ? "settings.personal.password_not_applied_impact"
        : "settings.personal.avatar_not_applied_impact"),
      message: t(kind === "password"
        ? "settings.personal.save_failed_message"
        : "settings.personal.avatar_save_failed_message"),
      nextStep: t(kind === "password"
        ? "settings.personal.password_not_applied_next_step"
        : "settings.personal.avatar_not_applied_next_step"),
      title: t("settings.personal.save_failed_title"),
      tone: "error",
    };
  }
  if (failure.effect === "committed") {
    return {
      impact: t(kind === "password"
        ? "settings.personal.password_committed_impact"
        : "settings.personal.avatar_committed_impact"),
      message: t(kind === "password"
        ? "settings.personal.password_committed_message"
        : "settings.personal.avatar_committed_message"),
      nextStep: t(kind === "password"
        ? "settings.personal.password_committed_next_step"
        : "state.committed_refresh_next_step"),
      title: t(kind === "password"
        ? "settings.personal.password_refresh_failed_title"
        : "settings.personal.profile_refresh_failed_title"),
      tone: "warning",
    };
  }
  if (kind === "password") {
    return {
      impact: t("settings.personal.password_unknown_impact"),
      message: t("settings.personal.password_unknown_message"),
      nextStep: t("settings.personal.password_unknown_next_step"),
      title: t("settings.personal.password_unknown_title"),
      tone: "warning",
    };
  }
  return {
    impact: t("settings.personal.avatar_unknown_impact"),
    message: t("settings.personal.avatar_unknown_message"),
    nextStep: t("settings.personal.avatar_unknown_next_step"),
    title: t("settings.personal.avatar_unknown_title"),
    tone: "warning",
  };
}

function withNewIntentAction(
  feedback: PersonalSettingsFeedback,
  action?: NonNullable<PersonalSettingsFeedback["action"]>,
): PersonalSettingsFeedback {
  return action ? { ...feedback, action } : feedback;
}
