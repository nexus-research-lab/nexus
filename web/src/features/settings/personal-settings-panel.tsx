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
  change_password_api,
  get_personal_profile_api,
  type PersonalProfile,
  update_personal_profile_api,
} from "@/lib/api/auth-api";
import {
  AGENT_ICON_ID_END,
  AGENT_ICON_ID_START,
  cn,
} from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { get_ui_button_class_name } from "@/shared/ui/button-styles";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";
import { IconPicker } from "@/shared/ui/icon-picker/icon-picker";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { PersonalSettingsTokenUsageSection } from "./personal-settings-token-usage-section";

type FeedbackTone = "success" | "error";

interface PasswordDraft {
  current_password: string;
  new_password: string;
  confirm_password: string;
}

interface FeedbackState {
  tone: FeedbackTone;
  title: string;
  message: string;
}

const EMPTY_PASSWORD_DRAFT: PasswordDraft = {
  current_password: "",
  new_password: "",
  confirm_password: "",
};

const PERSONAL_PRIMARY_BUTTON_CLASS_NAME = get_ui_button_class_name(
  { size: "md", tone: "primary", variant: "solid" },
  "gap-2 tracking-tight",
);
const PERSONAL_SECONDARY_BUTTON_CLASS_NAME = get_ui_button_class_name(
  { size: "md", variant: "surface" },
  "gap-2 tracking-tight",
);

function auth_method_label(value: string, t: ReturnType<typeof useI18n>["t"]): string {
  switch (value) {
    case "password":
      return t("settings.personal.auth_method_password");
    case "bearer":
      return t("settings.personal.auth_method_bearer");
    default:
      return t("settings.personal.auth_method_local");
  }
}

function user_role_label(value: string, t: ReturnType<typeof useI18n>["t"]): string {
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
  const { refresh_status } = useAuth();
  const [profile, set_profile] = useState<PersonalProfile | null>(null);
  const [loading, set_loading] = useState(true);
  const [password_draft, set_password_draft] = useState<PasswordDraft>(EMPTY_PASSWORD_DRAFT);
  const [submitting, set_submitting] = useState(false);
  const [saving_avatar, set_saving_avatar] = useState(false);
  const [feedback, set_feedback] = useState<FeedbackState | null>(null);

  const load_profile = useCallback(async () => {
    try {
      set_loading(true);
      const result = await get_personal_profile_api();
      set_profile(result);
      set_feedback((current) => (current?.tone === "error" ? null : current));
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.personal.load_failed_title"),
        message: error instanceof Error ? error.message : t("settings.personal.load_failed_message"),
      });
    } finally {
      set_loading(false);
    }
  }, [t]);

  useEffect(() => {
    void load_profile();
  }, [load_profile]);

  const validation_error = useMemo(() => {
    if (!profile?.can_change_password) {
      return t("settings.personal.password_disabled");
    }
    if (!password_draft.current_password.trim()) {
      return t("settings.personal.validation_current_required");
    }
    if (!password_draft.new_password.trim()) {
      return t("settings.personal.validation_new_required");
    }
    if (password_draft.new_password.length < 8) {
      return t("settings.personal.validation_new_length");
    }
    if (password_draft.new_password !== password_draft.confirm_password) {
      return t("settings.personal.validation_confirm_mismatch");
    }
    return null;
  }, [password_draft, profile?.can_change_password, t]);

  const has_password_input = Boolean(
    password_draft.current_password ||
    password_draft.new_password ||
    password_draft.confirm_password,
  );
  const can_submit_password = !validation_error && !submitting && !loading;
  const usage = profile?.token_usage;
  const avatar = profile?.user.avatar ?? "";
  const can_update_avatar = Boolean(profile?.can_update_profile) && !saving_avatar;

  const handle_change_password = useCallback(async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (validation_error || submitting) {
      if (validation_error) {
        set_feedback({
          tone: "error",
          title: t("settings.personal.save_failed_title"),
          message: validation_error,
        });
      }
      return;
    }

    try {
      set_submitting(true);
      await change_password_api({
        current_password: password_draft.current_password,
        new_password: password_draft.new_password,
      });
      await refresh_status();
      set_password_draft(EMPTY_PASSWORD_DRAFT);
      set_feedback({
        tone: "success",
        title: t("settings.personal.save_success_title"),
        message: t("settings.personal.save_success_message"),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.personal.save_failed_title"),
        message: error instanceof Error ? error.message : t("settings.personal.save_failed_message"),
      });
    } finally {
      set_submitting(false);
    }
  }, [password_draft, refresh_status, submitting, t, validation_error]);

  const handle_avatar_change = useCallback(async (next_avatar: string) => {
    if (!profile?.can_update_profile || saving_avatar || next_avatar === (profile.user.avatar ?? "")) {
      return;
    }
    try {
      set_saving_avatar(true);
      const result = await update_personal_profile_api({ avatar: next_avatar });
      set_profile(result);
      await refresh_status();
      set_feedback({
        tone: "success",
        title: t("settings.personal.profile_save_success_title"),
        message: t("settings.personal.avatar_save_success_message"),
      });
    } catch (error) {
      set_feedback({
        tone: "error",
        title: t("settings.personal.profile_save_failed_title"),
        message: error instanceof Error ? error.message : t("settings.personal.avatar_save_failed_message"),
      });
    } finally {
      set_saving_avatar(false);
    }
  }, [profile, refresh_status, saving_avatar, t]);

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
                      class_name="h-12 w-12 rounded-[16px]"
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
                      {t("settings.personal.role")}: {user_role_label(profile?.user.role ?? "", t)}
                    </span>
                    <span className="rounded-[10px] border border-(--divider-subtle-color) bg-transparent px-3 py-2">
                      {t("settings.personal.auth_method")}: {auth_method_label(profile?.user.auth_method ?? "", t)}
                    </span>
                  </div>
                </div>
                <div className="min-w-0 lg:min-w-[300px]">
                  <div className="mb-2 flex items-center justify-between gap-3">
                    <div className="flex min-w-0 items-center gap-2 text-[11px] font-semibold text-(--text-muted)">
                      <Image className="h-3.5 w-3.5" />
                      <span>{t("settings.personal.avatar_title")}</span>
                    </div>
                    {saving_avatar ? (
                      <span className="inline-flex items-center gap-1 text-[11px] text-(--text-soft)">
                        <Loader2 className="h-3 w-3 animate-spin" />
                        {t("common.saving")}
                      </span>
                    ) : null}
                  </div>
                  <IconPicker
                    class_name="min-w-0"
                    columns={8}
                    disabled={!can_update_avatar}
                    icon_size="sm"
                    layout="row"
                    max_icons={AGENT_ICON_ID_END - AGENT_ICON_ID_START + 1}
                    on_select={handle_avatar_change}
                    show_clear
                    start_icon_id={AGENT_ICON_ID_START}
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
              <form className="grid gap-3 px-3 py-3" onSubmit={handle_change_password}>
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
                      onChange={(event) => set_password_draft((current) => ({
                        ...current,
                        current_password: event.target.value,
                      }))}
                      type="password"
                      value={password_draft.current_password}
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
                      onChange={(event) => set_password_draft((current) => ({
                        ...current,
                        new_password: event.target.value,
                      }))}
                      type="password"
                      value={password_draft.new_password}
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
                      onChange={(event) => set_password_draft((current) => ({
                        ...current,
                        confirm_password: event.target.value,
                      }))}
                      type="password"
                      value={password_draft.confirm_password}
                    />
                  </label>
                </div>

                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p className="min-w-0 text-[11px] text-(--text-soft)">
                    {validation_error && profile?.can_change_password && has_password_input
                      ? validation_error
                      : t("settings.personal.password_rule")}
                  </p>
                  <button
                    className={cn(
                      can_submit_password ? PERSONAL_PRIMARY_BUTTON_CLASS_NAME : PERSONAL_SECONDARY_BUTTON_CLASS_NAME,
                      "min-w-28",
                    )}
                    disabled={!can_submit_password}
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
            on_dismiss: () => set_feedback(null),
            title: feedback.title,
            tone: feedback.tone,
          },
        ] : []}
      />
    </>
  );
}
