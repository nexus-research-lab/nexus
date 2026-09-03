/**
 * INPUT: 个人资料读取、头像保存、密码修改及当前认证状态刷新结果。
 * OUTPUT: 保留资料/密码草稿，未知 mutation 独立锁定重复提交；密码通过 durable exact request 回执核对。
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
  getPasswordChangeReceiptApi,
  getPersonalProfileApi,
  settlePasswordChangeNotAppliedApi,
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
import {
  createPasswordChangeRequestID,
  forgetPendingPasswordChangeRequest,
  readPendingPasswordChangeRequest,
  rememberPendingPasswordChangeRequest,
} from "./password-change-receipt";

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
  const [passwordMutationRequestID, setPasswordMutationRequestID] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<PersonalSettingsFeedback | null>(null);
  const profileLoadingRef = useRef(false);
  const profileRequestRef = useRef(0);
  const profileOwnerGenerationRef = useRef(ownerGeneration);
  const reloadProfileRef = useRef<() => void>(() => {});
  const reconcilePasswordRef = useRef<() => void>(() => {});
  const retryPasswordRef = useRef<() => void>(() => {});
  const abandonPasswordRef = useRef<() => void>(() => {});
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
      setPasswordMutationRequestID(null);
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
        const pendingRequestID = readPendingPasswordChangeRequest(result.user.user_id);
        if (pendingRequestID) {
          setPasswordMutationRequestID(pendingRequestID);
          setPasswordMutationBlocked(true);
          setFeedback(buildPasswordReceiptPendingFeedback(
            t,
            () => reconcilePasswordRef.current(),
          ));
        }
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
    try {
      await updatePersonalProfileApi({ avatar: nextAvatar });
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
    setProfile((current) => current
      ? { ...current, user: { ...current.user, avatar: nextAvatar } }
      : current);
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

  const runPasswordMutation = useCallback(async (
    requestID: string,
    userID: string,
  ) => {
    setIsSubmittingPassword(true);
    setFeedback(null);
    setPasswordMutationRequestID(requestID);
    rememberPendingPasswordChangeRequest(userID, requestID);
    try {
      await changePasswordApi({
        request_id: requestID,
        current_password: scopedPasswordDraft.currentPassword,
        new_password: scopedPasswordDraft.newPassword,
      });
    } catch (error) {
      const failure = projectMutationFailure(
        error,
        t("settings.personal.save_failed_message"),
      );
      const blocked = failure.effect !== "not_applied"
        && failure.effect !== "committed";
      if (!blocked) {
        forgetPendingPasswordChangeRequest(userID, requestID);
      }
      if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        return;
      }
      setPasswordMutationBlocked(blocked);
      if (failure.effect === "committed") {
        setPasswordDraft(EMPTY_PASSWORD_DRAFT);
      }
      if (!blocked) {
        setPasswordMutationRequestID(null);
      }
      setFeedback(blocked
        ? buildPasswordReceiptPendingFeedback(
            t,
            () => reconcilePasswordRef.current(),
          )
        : buildPersonalMutationFailure(failure, "password", t));
      setIsSubmittingPassword(false);
      return;
    }
    forgetPendingPasswordChangeRequest(userID, requestID);
    if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
      return;
    }
    setPasswordDraft(EMPTY_PASSWORD_DRAFT);
    setPasswordMutationBlocked(false);
    setPasswordMutationRequestID(null);
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
    ownerGeneration,
    refreshStatus,
    scopedPasswordDraft,
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
          nextStep: t("state.validation_failure_next_step"),
          title: t("settings.personal.save_failed_title"),
          tone: "error",
        });
      }
      return;
    }
    const userID = scopedProfile?.user.user_id ?? "";
    if (!userID) {
      return;
    }
    const pendingRequestID = readPendingPasswordChangeRequest(userID);
    if (pendingRequestID) {
      setPasswordMutationRequestID(pendingRequestID);
      setPasswordMutationBlocked(true);
      setFeedback(buildPasswordReceiptPendingFeedback(
        t,
        () => reconcilePasswordRef.current(),
      ));
      return;
    }
    await runPasswordMutation(createPasswordChangeRequestID(), userID);
  }, [
    isSubmittingPassword,
    ownerGeneration,
    passwordMutationBlocked,
    presentation.validationError,
    runPasswordMutation,
    scopeCurrent,
    scopedProfile,
    t,
  ]);

  const retryPassword = useCallback(async () => {
    const requestID = passwordMutationRequestID;
    const userID = scopedProfile?.user.user_id ?? "";
    if (
      !requestID
      || !userID
      || isSubmittingPassword
      || presentation.validationError
      || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
    ) {
      return;
    }
    await runPasswordMutation(requestID, userID);
  }, [
    isSubmittingPassword,
    ownerGeneration,
    passwordMutationRequestID,
    presentation.validationError,
    runPasswordMutation,
    scopedProfile,
  ]);
  retryPasswordRef.current = () => {
    void retryPassword();
  };

  const abandonPassword = useCallback(async () => {
    const requestID = passwordMutationRequestID;
    const userID = scopedProfile?.user.user_id ?? "";
    if (
      !requestID
      || !userID
      || isSubmittingPassword
      || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
    ) {
      return;
    }
    setIsSubmittingPassword(true);
    try {
      const receipt = await settlePasswordChangeNotAppliedApi(requestID);
      if (receipt.request_id === requestID && receipt.effect !== "unknown") {
        forgetPendingPasswordChangeRequest(userID, requestID);
      }
      if (
        !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        || receipt.request_id !== requestID
      ) {
        return;
      }
      if (receipt.effect === "committed") {
        setPasswordDraft(EMPTY_PASSWORD_DRAFT);
        setPasswordMutationBlocked(false);
        setPasswordMutationRequestID(null);
        try {
          await refreshStatus();
          setFeedback({
            message: t("settings.personal.save_success_message"),
            title: t("settings.personal.save_success_title"),
            tone: "success",
          });
        } catch {
          setFeedback({
            impact: t("settings.personal.password_committed_impact"),
            nextStep: t("settings.personal.password_committed_next_step"),
            title: t("settings.personal.password_refresh_failed_title"),
            tone: "warning",
          });
        }
        return;
      }
      if (receipt.effect === "not_applied") {
        setPasswordMutationBlocked(false);
        setPasswordMutationRequestID(null);
        setFeedback({
          impact: t("settings.personal.password_abandoned_impact"),
          nextStep: t("settings.personal.password_abandoned_next_step"),
          title: t("settings.personal.password_abandoned_title"),
          tone: "warning",
        });
        return;
      }
      setFeedback(buildPasswordReceiptPendingFeedback(
        t,
        () => reconcilePasswordRef.current(),
      ));
    } catch {
      if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setFeedback(buildPasswordReceiptPendingFeedback(
          t,
          () => reconcilePasswordRef.current(),
        ));
      }
    } finally {
      if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setIsSubmittingPassword(false);
      }
    }
  }, [
    isSubmittingPassword,
    ownerGeneration,
    passwordMutationRequestID,
    refreshStatus,
    scopedProfile,
    t,
  ]);
  abandonPasswordRef.current = () => {
    void abandonPassword();
  };

  const reconcilePassword = useCallback(async () => {
    const requestID = passwordMutationRequestID;
    const userID = scopedProfile?.user.user_id ?? "";
    if (
      !requestID
      || !userID
      || isSubmittingPassword
      || !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
    ) {
      return;
    }
    setIsSubmittingPassword(true);
    try {
      const receipt = await getPasswordChangeReceiptApi(requestID);
      if (receipt.request_id === requestID && receipt.effect !== "unknown") {
        forgetPendingPasswordChangeRequest(userID, requestID);
      }
      if (
        !isAuthOwnerScopeGenerationCurrent(ownerGeneration)
        || receipt.request_id !== requestID
      ) {
        return;
      }
      if (receipt.effect === "not_applied") {
        setPasswordMutationBlocked(false);
        setPasswordMutationRequestID(null);
        setFeedback({
          impact: t("settings.personal.password_abandoned_impact"),
          nextStep: t("settings.personal.password_abandoned_next_step"),
          title: t("settings.personal.password_abandoned_title"),
          tone: "warning",
        });
        return;
      }
      if (receipt.effect !== "committed") {
        setFeedback(
          hasPasswordDraftInput(scopedPasswordDraft)
          && !presentation.validationError
            ? buildPasswordReceiptRetryFeedback(
                t,
                () => retryPasswordRef.current(),
              )
            : buildPasswordReceiptAbandonFeedback(
                t,
                () => abandonPasswordRef.current(),
              ),
        );
        return;
      }
      setPasswordDraft(EMPTY_PASSWORD_DRAFT);
      setPasswordMutationBlocked(false);
      setPasswordMutationRequestID(null);
      try {
        await refreshStatus();
        setFeedback({
          message: t("settings.personal.save_success_message"),
          title: t("settings.personal.save_success_title"),
          tone: "success",
        });
      } catch {
        if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          setFeedback({
            impact: t("settings.personal.password_committed_impact"),
            nextStep: t("settings.personal.password_committed_next_step"),
            title: t("settings.personal.password_refresh_failed_title"),
            tone: "warning",
          });
        }
      }
    } catch {
      if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setFeedback(buildPasswordReceiptPendingFeedback(
          t,
          () => reconcilePasswordRef.current(),
        ));
      }
    } finally {
      if (isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        setIsSubmittingPassword(false);
      }
    }
  }, [
    isSubmittingPassword,
    ownerGeneration,
    passwordMutationRequestID,
    refreshStatus,
    presentation.validationError,
    scopedPasswordDraft,
    scopedProfile,
    t,
  ]);
  reconcilePasswordRef.current = () => {
    void reconcilePassword();
  };

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
type PersonalSettingsFailureFeedback = Extract<
  PersonalSettingsFeedback,
  { tone: "error" | "warning" }
>;
type Translate = ReturnType<typeof useI18n>["t"];

export function buildPersonalMutationFailure(
  failure: MutationFailure,
  kind: PersonalMutationKind,
  t: Translate,
): PersonalSettingsFailureFeedback {
  if (failure.effect === "not_applied") {
    return {
      impact: t(kind === "password"
        ? "settings.personal.password_not_applied_impact"
        : "settings.personal.avatar_not_applied_impact"),
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
      nextStep: t("settings.personal.password_unknown_next_step"),
      title: t("settings.personal.password_unknown_title"),
      tone: "warning",
    };
  }
  return {
    impact: t("settings.personal.avatar_unknown_impact"),
    nextStep: t("settings.personal.avatar_unknown_next_step"),
    title: t("settings.personal.avatar_unknown_title"),
    tone: "warning",
  };
}

function buildPasswordReceiptPendingFeedback(
  t: Translate,
  onReconcile: () => void,
): PersonalSettingsFailureFeedback {
  return {
    action: {
      label: t("settings.personal.password_reconcile"),
      onClick: onReconcile,
    },
    impact: t("settings.personal.password_unknown_impact"),
    title: t("settings.personal.password_unknown_title"),
    tone: "warning",
  };
}

function buildPasswordReceiptRetryFeedback(
  t: Translate,
  onRetry: () => void,
): PersonalSettingsFailureFeedback {
  return {
    action: {
      label: t("settings.personal.password_retry_same_request"),
      onClick: onRetry,
    },
    impact: t("settings.personal.password_receipt_not_found_impact"),
    title: t("settings.personal.password_unknown_title"),
    tone: "warning",
  };
}

function buildPasswordReceiptAbandonFeedback(
  t: Translate,
  onAbandon: () => void,
): PersonalSettingsFailureFeedback {
  return {
    action: {
      label: t("settings.personal.password_abandon_request"),
      onClick: onAbandon,
    },
    impact: t("settings.personal.password_receipt_missing_draft_impact"),
    title: t("settings.personal.password_unknown_title"),
    tone: "warning",
  };
}

function withNewIntentAction(
  feedback: PersonalSettingsFailureFeedback,
  action?: NonNullable<PersonalSettingsFeedback["action"]>,
): PersonalSettingsFailureFeedback {
  if (!action) {
    return feedback;
  }
  if ("nextStep" in feedback) {
    const { nextStep: _nextStep, ...withoutGuidance } = feedback;
    return { ...withoutGuidance, action };
  }
  return { ...feedback, action };
}
