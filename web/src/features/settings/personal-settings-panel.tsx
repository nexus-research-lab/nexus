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
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  Database,
  Gauge,
  KeyRound,
  Loader2,
  LockKeyhole,
  ShieldCheck,
  UserRound,
} from "lucide-react";

import {
  change_password_api,
  get_personal_profile_api,
  type PersonalProfile,
  type TokenUsageSummary,
} from "@/lib/api/auth-api";
import { cn, format_tokens } from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { FeedbackBannerStack } from "@/shared/ui/feedback/feedback-banner-stack";

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

const PERSONAL_ACTION_BUTTON_CLASS_NAME = "inline-flex h-9 items-center justify-center gap-2 rounded-xl border px-3 text-sm font-medium tracking-tight transition-[border-color,background,color,box-shadow,transform] duration-(--motion-duration-fast) ease-out disabled:pointer-events-none disabled:opacity-(--disabled-opacity)";
const PERSONAL_PRIMARY_BUTTON_CLASS_NAME = `${PERSONAL_ACTION_BUTTON_CLASS_NAME} border-(--surface-interactive-active-border) bg-primary text-white shadow-[0_8px_24px_rgba(16,185,129,0.16)] hover:-translate-y-px hover:shadow-[0_12px_28px_rgba(16,185,129,0.22)]`;
const PERSONAL_SECONDARY_BUTTON_CLASS_NAME = `${PERSONAL_ACTION_BUTTON_CLASS_NAME} border-(--divider-subtle-color) bg-(--surface-base-background) text-(--text-strong) hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background)`;

function format_updated_at(value: string, locale: "zh" | "en"): string {
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) {
    return "--";
  }
  return date.toLocaleString(locale === "zh" ? "zh-CN" : "en-US", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

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

export function PersonalSettingsPanel() {
  const { locale, t } = useI18n();
  const { refresh_status } = useAuth();
  const [profile, set_profile] = useState<PersonalProfile | null>(null);
  const [loading, set_loading] = useState(true);
  const [password_draft, set_password_draft] = useState<PasswordDraft>(EMPTY_PASSWORD_DRAFT);
  const [submitting, set_submitting] = useState(false);
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
  const quota_text = usage?.quota_limit_tokens == null
    ? t("settings.personal.quota_unset")
    : `${format_tokens(usage.total_tokens)} / ${format_tokens(usage.quota_limit_tokens)}`;

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

  return (
    <>
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-3 px-1 py-3">
        <section>
          <h2 className="text-[20px] font-semibold tracking-tight text-(--text-strong)">
            {t("settings.personal.title")}
          </h2>
        </section>

        {loading ? (
          <section className="flex min-h-[220px] items-center justify-center rounded-[18px] border border-(--divider-subtle-color) bg-(--surface-card-background) text-(--text-soft)">
            <Loader2 className="h-5 w-5 animate-spin" />
          </section>
        ) : (
          <>
            <section className="overflow-hidden rounded-[18px] border border-(--divider-subtle-color) bg-(--surface-card-background)">
              <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
                <div className="flex min-w-0 items-center gap-3">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                    <UserRound className="h-3.5 w-3.5" />
                  </div>
                  <div className="min-w-0">
                    <h3 className="truncate text-[15px] font-semibold tracking-tight text-(--text-strong)">
                      {profile?.user.display_name || profile?.user.username || "--"}
                    </h3>
                    <p className="mt-1 truncate text-[12px] leading-5 text-(--text-soft)">
                      {profile?.user.username || "--"}
                    </p>
                  </div>
                </div>
                <div className="grid gap-2 text-[11px] text-(--text-soft) sm:grid-cols-2 lg:min-w-[260px]">
                  <span className="rounded-xl border border-(--divider-subtle-color) bg-(--surface-inset-background) px-3 py-2">
                    {t("settings.personal.role")}: {profile?.user.role || "--"}
                  </span>
                  <span className="rounded-xl border border-(--divider-subtle-color) bg-(--surface-inset-background) px-3 py-2">
                    {t("settings.personal.auth_method")}: {auth_method_label(profile?.user.auth_method ?? "", t)}
                  </span>
                </div>
              </div>
            </section>

            <section className="order-last overflow-hidden rounded-[18px] border border-(--divider-subtle-color) bg-(--surface-card-background)">
              <div className="grid gap-3 px-3 py-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
                <div className="flex min-w-0 items-start gap-3">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
                    <Gauge className="h-3.5 w-3.5" />
                  </div>
                  <div className="min-w-0">
                    <h3 className="text-[15px] font-semibold tracking-tight text-(--text-strong)">
                      {t("settings.personal.token_usage_title")}
                    </h3>
                    <p className="mt-1 text-[12px] leading-5 text-(--text-soft)">
                      {t("settings.personal.updated_at", {
                        value: usage ? format_updated_at(usage.updated_at, locale) : "--",
                      })}
                    </p>
                  </div>
                </div>
                <div className="text-left lg:text-right">
                  <div className="text-[24px] font-semibold tracking-tight text-(--text-strong)">
                    {format_tokens(usage?.total_tokens ?? 0)}
                  </div>
                  <div className="mt-1 text-[11px] font-medium text-(--text-soft)">
                    {t("settings.personal.total_tokens")}
                  </div>
                </div>
              </div>

              <div className="mx-3 border-t border-(--divider-subtle-color)" />

              <div className="grid gap-2 px-3 py-3 sm:grid-cols-2">
                <UsageMetric
                  icon={<ShieldCheck className="h-3.5 w-3.5" />}
                  label={t("settings.personal.quota_limit")}
                  value={quota_text}
                />
                <UsageMetric
                  icon={<KeyRound className="h-3.5 w-3.5" />}
                  label={t("settings.personal.input_tokens")}
                  value={format_tokens(usage?.input_tokens ?? 0)}
                />
                <UsageMetric
                  icon={<LockKeyhole className="h-3.5 w-3.5" />}
                  label={t("settings.personal.output_tokens")}
                  value={format_tokens(usage?.output_tokens ?? 0)}
                />
                <UsageMetric
                  icon={<Database className="h-3.5 w-3.5" />}
                  label={t("settings.personal.cache_tokens")}
                  value={format_tokens(
                    (usage?.cache_creation_input_tokens ?? 0) + (usage?.cache_read_input_tokens ?? 0),
                  )}
                />
              </div>

              <div className="mx-3 border-t border-(--divider-subtle-color)" />

              <TokenUsageChart
                usage={usage}
                labels={{
                  input: t("settings.personal.input_tokens"),
                  output: t("settings.personal.output_tokens"),
                  cache: t("settings.personal.cache_tokens"),
                }}
              />

              <div className="mx-3 border-t border-(--divider-subtle-color)" />

              <div className="grid gap-2 px-3 py-2.5 text-[11px] text-(--text-soft) sm:grid-cols-2">
                <span>{t("settings.personal.session_count", { count: usage?.session_count ?? 0 })}</span>
                <span>{t("settings.personal.message_count", { count: usage?.message_count ?? 0 })}</span>
              </div>
            </section>

            <section className="overflow-hidden rounded-[18px] border border-(--divider-subtle-color) bg-(--surface-card-background)">
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

function UsageMetric({
  icon,
  label,
  value,
}: {
  icon: ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="flex min-w-0 items-center gap-3 rounded-2xl border border-(--divider-subtle-color) bg-(--surface-inset-background) px-3 py-3">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[16px] bg-(--surface-card-background) text-primary">
        {icon}
      </div>
      <div className="min-w-0">
        <div className="truncate text-[11px] font-medium text-(--text-soft)">
          {label}
        </div>
        <div className="mt-1 truncate text-[14px] font-semibold text-(--text-strong)">
          {value}
        </div>
      </div>
    </div>
  );
}

function TokenUsageChart({
  usage,
  labels,
}: {
  usage: TokenUsageSummary | undefined;
  labels: {
    input: string;
    output: string;
    cache: string;
  };
}) {
  const input_tokens = usage?.input_tokens ?? 0;
  const output_tokens = usage?.output_tokens ?? 0;
  const cache_tokens = (usage?.cache_creation_input_tokens ?? 0) + (usage?.cache_read_input_tokens ?? 0);
  const total = Math.max(input_tokens + output_tokens + cache_tokens, 1);
  const items = [
    {
      key: "input",
      label: labels.input,
      value: input_tokens,
      class_name: "bg-primary",
    },
    {
      key: "output",
      label: labels.output,
      value: output_tokens,
      class_name: "bg-sky-500",
    },
    {
      key: "cache",
      label: labels.cache,
      value: cache_tokens,
      class_name: "bg-amber-500",
    },
  ];

  return (
    <div className="px-3 py-3">
      <div className="flex h-2 overflow-hidden rounded-full bg-(--surface-inset-background)">
        {items.map((item) => (
          <div
            className={cn(item.value > 0 ? "min-w-[2px]" : "", item.class_name)}
            key={item.key}
            style={{ width: `${(item.value / total) * 100}%` }}
          />
        ))}
      </div>
      <div className="mt-3 grid gap-2 sm:grid-cols-3">
        {items.map((item) => (
          <div className="flex min-w-0 items-center gap-2 text-[11px] text-(--text-soft)" key={item.key}>
            <span className={cn("h-2 w-2 shrink-0 rounded-full", item.class_name)} />
            <span className="min-w-0 flex-1 truncate">{item.label}</span>
            <span className="font-semibold text-(--text-strong)">{format_tokens(item.value)}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
