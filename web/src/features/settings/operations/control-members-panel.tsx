// INPUT: 当前 Organization 成员、当前身份权限和成员更新命令。
// OUTPUT: 可嵌入组织页的成员目录与互斥写入；未知写结果保持禁写，直至清单读取成功。
// POS: Operations 组织治理页的成员子视图；不拥有页面标题、邀请或账号创建。
"use client";

import { RefreshCw } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
  SETTINGS_GROUP_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import {
  listControlMembersApi,
  updateControlMemberApi,
  type ControlDeploymentMember,
  type ControlMemberRole,
} from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

type Feedback = { tone: "success" | "error"; message: string } | null;

export function OrganizationMembersPanel() {
  const { t } = useI18n();
  const { status } = useAuth();
  const [members, setMembers] = useState<ControlDeploymentMember[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [pendingKey, setPendingKey] = useState<string | null>(null);
  const mutationPendingRef = useRef(false);
  const loadingRef = useRef(false);
  const [mutationsBlocked, setMutationsBlocked] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const roleOptions = [
    { value: "member", label: t("settings.personal.role_member") },
    { value: "admin", label: t("settings.personal.role_admin") },
    { value: "owner", label: t("settings.personal.role_owner") },
  ];
  const visibleMembers = useMemo(() => {
    const matcher = createUiSearchMatcher(query);
    return members.filter((member) => matcher.matches([
      member.display_name,
      member.username,
    ]));
  }, [members, query]);

  const loadMembers = useCallback(async (reconcileMutation = false) => {
    if (loadingRef.current || (mutationPendingRef.current && !reconcileMutation)) return;
    loadingRef.current = true;
    setLoading(true);
    setLoadFailed(false);
    try {
      setMembers(await listControlMembersApi());
      setMutationsBlocked(false);
      if (!reconcileMutation) {
        setFeedback((current) => current?.tone === "error" ? null : current);
      }
    } catch {
      setLoadFailed(true);
      setFeedback({ tone: "error", message: t("members.load_failed") });
    } finally {
      loadingRef.current = false;
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadMembers();
  }, [loadMembers]);

  const updateMember = async (
    member: ControlDeploymentMember,
    change: { role?: ControlMemberRole; status?: "active" | "revoked" },
  ) => {
    if (loadingRef.current || mutationPendingRef.current || mutationsBlocked) return;
    mutationPendingRef.current = true;
    const key = `update:${member.user_id}`;
    setPendingKey(key);
    setFeedback(null);
    try {
      const updated = await updateControlMemberApi(member.user_id, change);
      setMembers((current) => current.map((item) => (
        item.user_id === updated.user_id ? updated : item
      )));
      setFeedback({ tone: "success", message: t("members.update_success") });
    } catch {
      setFeedback({ tone: "error", message: t("members.update_unknown") });
      setMutationsBlocked(true);
      await loadMembers(true);
    } finally {
      mutationPendingRef.current = false;
      setPendingKey(null);
    }
  };

  const writesDisabled = loading || pendingKey !== null || mutationsBlocked;

  return (
    <div className="@container/members grid min-w-0 gap-3">
      {feedback ? (
        <p
          className={cn(
            "py-3",
            getUiTypographyClassName({
              role: "supporting",
              tone: feedback.tone === "success" ? "muted" : "danger",
            }),
          )}
          role="status"
        >
          {feedback.message}
        </p>
      ) : null}

      <section aria-label={t("members.list_label")} className={SETTINGS_GROUP_CLASS_NAME}>
        <div className="flex flex-wrap items-center gap-3 border-b border-(--divider-subtle-color) p-3">
          <UiSearchInput
            aria-label={t("members.search_placeholder")}
            className="min-w-52 flex-1 sm:max-w-80"
            onChange={setQuery}
            placeholder={t("members.search_placeholder")}
            value={query}
          />
          <span className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>
            {t("members.results", { count: visibleMembers.length })}
          </span>
          <UiButton disabled={loading || pendingKey !== null} onClick={() => void loadMembers()} size="sm" variant="text">
            <RefreshCw className={loading ? getUiSpinnerClassName({ size: "sm" }) : "h-3.5 w-3.5"} />
            {t("members.refresh")}
          </UiButton>
        </div>
        {members.length > 0 ? (
          <div className="hidden grid-cols-[minmax(0,1fr)_160px_160px] gap-3 border-b border-(--divider-subtle-color) px-4 py-2 @min-[560px]/members:grid">
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>{t("members.column_member")}</span>
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>{t("members.column_role")}</span>
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>{t("members.column_access")}</span>
          </div>
        ) : null}
        <div className="divide-y divide-(--divider-subtle-color)">
          {loading && members.length === 0 ? (
            <UiResourceState
              size="sm"
              state="loading"
              title={t("members.loading")}
              variant="plain"
            />
          ) : null}
          {!loading && !loadFailed && members.length === 0 ? (
            <UiResourceState
              size="sm"
              state="empty"
              title={t("members.empty")}
              variant="plain"
            />
          ) : null}
          {!loading && !loadFailed && members.length > 0 && visibleMembers.length === 0 ? (
            <UiResourceState
              description={t("members.search_empty_description")}
              size="sm"
              state="empty"
              title={t("members.search_empty")}
              variant="plain"
            />
          ) : null}
          {visibleMembers.map((member) => {
            const isSelf = member.user_id === status?.user_id;
            const isPending = pendingKey === `update:${member.user_id}`;
            const canEditRole = status?.role === "owner" && !isSelf;
            const canToggle = !isSelf && (status?.role === "owner" || member.role === "member");
            return (
              <article className="grid items-center gap-3 px-4 py-3 @min-[560px]/members:grid-cols-[minmax(0,1fr)_160px_160px]" key={member.user_id}>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <p className={cn("min-w-0 wrap-anywhere", SETTINGS_ITEM_TITLE_CLASS_NAME)}>
                    {member.display_name || member.username}
                  </p>
                  {isSelf ? (
                    <span className={getUiTypographyClassName({ role: "caption", tone: "soft" })}>
                      {t("members.current")}
                    </span>
                  ) : null}
                </div>
                <p className={cn(
                  "mt-1 wrap-anywhere",
                  getUiTypographyClassName({ role: "caption", tone: "muted" }),
                )}>
                  @{member.username}
                </p>
              </div>
              <UiSelectMenu
                ariaLabel={`${t("members.role")}: ${member.display_name || member.username}`}
                disabled={!canEditRole || writesDisabled}
                onChange={(value) => void updateMember(member, { role: value as ControlMemberRole })}
                options={roleOptions}
                size="sm"
                value={member.role}
              />
              <div className="flex min-w-0 items-center justify-between gap-2">
                <UiBadge size="xs" tone={member.membership_status === "active" ? "default" : "warning"}>
                  {t(member.membership_status === "active" ? "members.active" : "members.revoked")}
                </UiBadge>
                <UiButton
                  disabled={!canToggle || writesDisabled}
                  onClick={() => void updateMember(member, { status: member.membership_status === "active" ? "revoked" : "active" })}
                  size="sm"
                  tone={member.membership_status === "active" ? "danger" : "primary"}
                  variant="text"
                >
                  {isPending
                    ? t("members.updating")
                    : member.membership_status === "active"
                      ? t("members.suspend")
                      : t("members.restore")}
                </UiButton>
              </div>
              </article>
            );
          })}
        </div>
      </section>

    </div>
  );
}
