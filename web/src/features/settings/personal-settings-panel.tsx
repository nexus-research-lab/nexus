/**
 * # !/usr/bin/env tsx
 * # -*- coding: utf-8 -*-
 * # =====================================================
 * # @File   ：personal-settings-panel.tsx
 * # @Date   ：2026/04/24 00:00
 * # @Author ：leemysw
 * # 2026/04/24 00:00   Create
 * # =====================================================
 */

"use client";

import {
  type FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  Image,
  Loader2,
  LockKeyhole,
} from "lucide-react";

import {
  changePasswordApi,
  getPersonalProfileApi,
  type PersonalProfile,
  updatePersonalProfileApi,
} from "@/lib/api/auth-api";
import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
  cn,
} from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { getUiButtonClassName } from "@/shared/ui/button-styles";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { PersonalSettingsTokenUsageSection } from "./personal-settings-token-usage-section";

type FeedbackTone = "success" | "error";

interface PasswordDraft {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

interface FeedbackState {
  tone: FeedbackTone;
  title: string;
  message: string;
}

const EMPTY_PASSWORD_DRAFT: PasswordDraft = {
  currentPassword: "",
  newPassword: "",
  confirmPassword: "",
};

const PERSONAL_PRIMARY_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "md", tone: "primary", variant: "solid" },
  "gap-2 tracking-tight",
);
const PERSONAL_SECONDARY_BUTTON_CLASS_NAME = getUiButtonClassName(
  { size: "md", variant: "surface" },
  "gap-2 tracking-tight",
);

function authMethodLabel(value: string, t: ReturnType<typeof useI18n>["t"]): string {
  switch (value) {
    case "password":
      return t("settings.personal.auth_method_password");
    case "bearer":
      return t("settings.personal.auth_method_bearer");
    default:
      return t("settings.personal.auth_method_local");
  }
}

function userRoleLabel(value: string, t: ReturnType<typeof useI18n>["t"]): string {
  switch (value) {
    case "owner":
      return t("settings.personal.role_owner");
    case "admin":
      return t("settings.personal.role_admin");
    case "member":
      return t("settings.personal.role_member");
    default:
      return value || "--";
  }
}

export function PersonalSettingsPanel() {
  const { t } = useI18n();
  const { refreshStatus: refreshStatus } = useAuth();
  const [profile, setProfile] = useState<PersonalProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [passwordDraft, setPasswordDraft] = useState<PasswordDraft>(EMPTY_PASSWORD_DRAFT);
  const [submitting, setSubmitting] = useState(false);
  const [savingAvatar, setSavingAvatar] = useState(false);
  const [feedback, setFeedback] = useState<FeedbackState | null>(null);

  const loadProfile = useCallback(async () => {
    try {
      setLoading(true);
      const result = await getPersonalProfileApi();
      setProfile(result);
      setFeedback((current) => (current?.tone === "error" ? null : current));
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.personal.load_failed_title"),
        message: error instanceof Error ? error.message : t("settings.personal.load_failed_message"),
      });
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadProfile();
  }, [loadProfile]);

  const validationError = useMemo(() => {
    if (!profile?.can_change_password) {
      return t("settings.personal.password_disabled");
    }
    if (!passwordDraft.currentPassword.trim()) {
      return t("settings.personal.validation_current_required");
    }
    if (!passwordDraft.newPassword.trim()) {
      return t("settings.personal.validation_new_required");
    }
    if (passwordDraft.newPassword.length < 8) {
      return t("settings.personal.validation_new_length");
    }
    if (passwordDraft.newPassword !== passwordDraft.confirmPassword) {
      return t("settings.personal.validation_confirm_mismatch");
    }
    return null;
  }, [passwordDraft, profile?.can_change_password, t]);

  const hasPasswordInput = Boolean(
    passwordDraft.currentPassword ||
    passwordDraft.newPassword ||
    passwordDraft.confirmPassword,
  );
  const canSubmitPassword = !validationError && !submitting && !loading;
  const usage = profile?.token_usage;
  const avatar = profile?.user.avatar ?? "";
  const canUpdateAvatar = Boolean(profile?.can_update_profile) && !savingAvatar;

  const handleChangePassword = useCallback(async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (validationError || submitting) {
      if (validationError) {
        setFeedback({
          tone: "error",
          title: t("settings.personal.save_failed_title"),
          message: validationError,
        });
      }
      return;
    }

    try {
      setSubmitting(true);
      await changePasswordApi({
        current_password: passwordDraft.currentPassword,
        new_password: passwordDraft.newPassword,
      });
      await refreshStatus();
      setPasswordDraft(EMPTY_PASSWORD_DRAFT);
      setFeedback({
        tone: "success",
        title: t("settings.personal.save_success_title"),
        message: t("settings.personal.save_success_message"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.personal.save_failed_title"),
        message: error instanceof Error ? error.message : t("settings.personal.save_failed_message"),
      });
    } finally {
      setSubmitting(false);
    }
  }, [passwordDraft, refreshStatus, submitting, t, validationError]);

  const handleAvatarChange = useCallback(async (nextAvatar: string) => {
    if (!profile?.can_update_profile || savingAvatar || nextAvatar === (profile.user.avatar ?? "")) {
      return;
    }
    try {
      setSavingAvatar(true);
      const result = await updatePersonalProfileApi({ avatar: nextAvatar });
      setProfile(result);
      await refreshStatus();
      setFeedback({
        tone: "success",
        title: t("settings.personal.profile_save_success_title"),
        message: t("settings.personal.avatar_save_success_message"),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        title: t("settings.personal.profile_save_failed_title"),
        message: error instanceof Error ? error.message : t("settings.personal.avatar_save_failed_message"),
      });
    } finally {
      setSavingAvatar(false);
    }
  }, [profile, refreshStatus, savingAvatar, t]);

  return (
    <>
      <div className={cn("mx-auto flex w-full flex-col gap-3 px-1 py-3", WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME)}>
        <section>
          <h2 className="text-[20px] font-semibold tracking-tight text-(--text-strong)">
            {t("settings.personal.title")}
          </h2>
        </section>

        {loading ? (
          <section className="flex min-h-[220px] items-center justify-center rounded-[12px] border border-(--divider-subtle-color) bg-transparent text-(--text-soft)">
            <Loader2 className="h-5 w-5 animate-spin" />
          </section>
        ) : (
          <>
            <section className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
              <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,300px)] lg:items-start">
                <div className="min-w-0 space-y-3">
                  <div className="flex min-w-0 items-center gap-3">
                    <UiAgentAvatar
                      avatar={avatar}
                      className="h-12 w-12 rounded-[16px]"
                      name={profile?.user.display_name || profile?.user.username || t("settings.personal.avatar_alt")}
                      shape="rounded"
                    />
                    <div className="min-w-0">
                      <h3 className="truncate text-[15px] font-semibold tracking-tight text-(--text-strong)">
                        {profile?.user.display_name || profile?.user.username || "--"}
                      </h3>
                      <p className="mt-1 truncate text-[12px] leading-5 text-(--text-soft)">
                        {profile?.user.username || "--"}
                      </p>
                    </div>
                  </div>
                  <div className="grid gap-2 text-[11px] text-(--text-soft) sm:grid-cols-2">
                    <span className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-2">
                      {t("settings.personal.role")}: {userRoleLabel(profile?.user.role ?? "", t)}
                    </span>
                    <span className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-2">
                      {t("settings.personal.auth_method")}: {authMethodLabel(profile?.user.auth_method ?? "", t)}
                    </span>
                  </div>
                </div>
                <div className="min-w-0 lg:min-w-[300px]">
                  <div className="mb-2 flex items-center justify-between gap-3">
                    <div className="flex min-w-0 items-center gap-2 text-[11px] font-semibold text-(--text-muted)">
                      <Image className="h-3.5 w-3.5" />
                      <span>{t("settings.personal.avatar_title")}</span>
                    </div>
                    {savingAvatar ? (
                      <span className="inline-flex items-center gap-1 text-[11px] text-(--text-soft)">
                        <Loader2 className="h-3 w-3 animate-spin" />
                        {t("common.saving")}
                      </span>
                    ) : null}
                  </div>
                  <IconPicker
                    className="min-w-0"
                    columns={8}
                    disabled={!canUpdateAvatar}
                    iconSize="sm"
                    layout="row"
                    maxIcons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
                    onSelect={handleAvatarChange}
                    showClear
                    startIconId={AGENT_ICON_ID_START}
                    value={avatar}
                  />
                  {!profile?.can_update_profile ? (
                    <p className="mt-2 text-[11px] text-(--text-soft)">
                      {t("settings.personal.avatar_disabled")}
                    </p>
                  ) : null}
                </div>
              </div>
            </section>

            <PersonalSettingsTokenUsageSection usage={usage} />

            <section className="overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent">
              <form className="grid gap-3 px-3 py-3" onSubmit={handleChangePassword}>
                <div className="flex items-center gap-3">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                    <LockKeyhole className="h-3.5 w-3.5" />
                  </div>
                  <div className="min-w-0">
                    <h3 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
                      {t("settings.personal.password_title")}
                    </h3>
                    {!profile?.can_change_password ? (
                      <p className="mt-1 text-[12px] leading-5 text-(--text-soft)">
                        {t("settings.personal.password_disabled")}
                      </p>
                    ) : null}
                  </div>
                </div>

                <div className="grid gap-3 md:grid-cols-3">
                  <label className="space-y-1.5">
                    <span className="text-[11px] font-semibold text-(--text-muted)">
                      {t("settings.personal.password_current")}
                    </span>
                    <input
                      autoComplete="current-password"
                      className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)"
                      disabled={!profile?.can_change_password || submitting}
                      onChange={(event) => setPasswordDraft((current) => ({
                        ...current,
                        currentPassword: event.target.value,
                      }))}
                      type="password"
                      value={passwordDraft.currentPassword}
                    />
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-[11px] font-semibold text-(--text-muted)">
                      {t("settings.personal.password_new")}
                    </span>
                    <input
                      autoComplete="new-password"
                      className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)"
                      disabled={!profile?.can_change_password || submitting}
                      onChange={(event) => setPasswordDraft((current) => ({
                        ...current,
                        newPassword: event.target.value,
                      }))}
                      type="password"
                      value={passwordDraft.newPassword}
                    />
                  </label>
                  <label className="space-y-1.5">
                    <span className="text-[11px] font-semibold text-(--text-muted)">
                      {t("settings.personal.password_confirm")}
                    </span>
                    <input
                      autoComplete="new-password"
                      className="dialog-input h-9 w-full rounded-xl px-3 text-sm text-(--text-strong) outline-none disabled:opacity-(--disabled-opacity)"
                      disabled={!profile?.can_change_password || submitting}
                      onChange={(event) => setPasswordDraft((current) => ({
                        ...current,
                        confirmPassword: event.target.value,
                      }))}
                      type="password"
                      value={passwordDraft.confirmPassword}
                    />
                  </label>
                </div>

                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p className="min-w-0 text-[11px] text-(--text-soft)">
                    {validationError && profile?.can_change_password && hasPasswordInput
                      ? validationError
                      : t("settings.personal.password_rule")}
                  </p>
                  <button
                    className={cn(
                      canSubmitPassword ? PERSONAL_PRIMARY_BUTTON_CLASS_NAME : PERSONAL_SECONDARY_BUTTON_CLASS_NAME,
                      "min-w-28",
                    )}
                    disabled={!canSubmitPassword}
                    type="submit"
                  >
                    {submitting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
                    {submitting ? t("common.saving") : t("settings.personal.change_password")}
                  </button>
                </div>
              </form>
            </section>
          </>
        )}
      </div>

      <FeedbackBannerStack
        items={feedback ? [
          {
            key: "personal-settings-feedback",
            message: feedback.message,
            onDismiss: () => setFeedback(null),
            title: feedback.title,
            tone: feedback.tone,
          },
        ] : []}
      />
    </>
  );
}
