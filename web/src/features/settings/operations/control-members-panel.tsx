// INPUT: 当前 Organization 成员、当前身份权限和成员更新命令。
// OUTPUT: 可嵌入组织页的成员目录与互斥写入；未知写结果保持禁写，直至清单读取成功。
// POS: Operations 组织治理页的成员子视图；不拥有页面标题、邀请或账号创建。
"use client";

import { RefreshCw } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import {
  SETTINGS_CONTROL_LABEL_CLASS_NAME,
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

export function OrganizationMembersPanel({ toolbarActions }: { toolbarActions?: ReactNode }) {
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
      setMembers((await listControlMembersApi()).filter((member) => member.membership_status === "active"));
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
      )).filter((item) => item.membership_status === "active"));
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
  const canRemoveMember = (member: ControlDeploymentMember) => (
    member.user_id !== (status?.control_user_id ?? status?.user_id)
    && member.membership_status === "active"
    && member.role !== "owner"
    && (status?.organization_role === "owner" || (status?.organization_role === "admin" && member.role === "member"))
  );
  const showActions = visibleMembers.some(canRemoveMember);
  const columns = showActions
    ? "@min-[560px]/members:grid-cols-[minmax(0,1fr)_104px_90px_72px]"
    : "@min-[560px]/members:grid-cols-[minmax(0,1fr)_104px_90px]";

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

      {loadFailed || mutationsBlocked ? (
          <UiButton disabled={loading || pendingKey !== null} onClick={() => void loadMembers()} size="sm" variant="text">
            <RefreshCw className={loading ? getUiSpinnerClassName({ size: "sm" }) : "h-3.5 w-3.5"} />
            {t("members.refresh")}
          </UiButton>
      ) : null}

      <section aria-label={t("members.list_label")} className="min-w-0">
        <div className="grid grid-cols-1 items-center justify-between gap-3 border-b border-(--divider-subtle-color) pb-3 @min-[560px]/members:flex">
          {toolbarActions}
          <UiSearchInput
            aria-label={t("members.search_placeholder")}
            className="order-last min-w-0 w-full @min-[560px]/members:order-none @min-[560px]/members:w-60"
            onChange={setQuery}
            placeholder={t("members.search_placeholder")}
            value={query}
          />

        </div>
        {members.length > 0 ? (
          <div className={cn("hidden gap-3 border-b border-(--divider-subtle-color) px-4 py-2 @min-[560px]/members:grid", columns)}>
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>{t("members.column_member")}</span>
            <span className={cn("px-3", SETTINGS_CONTROL_LABEL_CLASS_NAME)}>{t("members.column_role")}</span>
            <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>{t("members.column_access")}</span>
            {showActions ? <span className={SETTINGS_CONTROL_LABEL_CLASS_NAME}>{t("members.column_actions")}</span> : null}
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
            const isSelf = member.user_id === (status?.control_user_id ?? status?.user_id);
            const canEditRole = status?.organization_role === "owner" && !isSelf && member.role !== "owner" && member.membership_status === "active";
            const canRemove = canRemoveMember(member);
            return (
              <article className={cn("grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 px-4 py-3 hover:bg-(--surface-interactive-hover-background)", columns)} key={member.user_id}>
              <div className="col-span-3 min-w-0 @min-[560px]/members:col-span-1">
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
              {canEditRole ? <UiSelectMenu
                className="w-[104px] max-w-full"
                allowLabelWrap
                surface="plain"
                ariaLabel={`${t("members.role")}: ${member.display_name || member.username}`}
                disabled={writesDisabled}
                onChange={(value) => void updateMember(member, { role: value as ControlMemberRole })}
                options={roleOptions.filter((option) => option.value !== "owner")}
                size="sm"
                value={member.role}
              /> : <span className={cn("px-3", getUiTypographyClassName({ role: "supporting" }))}>{roleOptions.find((option) => option.value === member.role)?.label}</span>}
              <div className="min-w-0">
                <UiBadge size="xs" tone={member.membership_status === "active" ? "default" : "warning"}>
                  {t(member.membership_status === "active" ? "members.active" : "members.revoked")}
                </UiBadge>
              </div>
              {showActions ? <div className="flex justify-end">
                {canRemove ? <UiButton
                  aria-label={`${t("members.remove")}: ${member.display_name || member.username}`}
                  disabled={writesDisabled}
                  size="sm"
                  variant="text"
                  onClick={() => void updateMember(member, { status: "revoked" })}
                >{t("members.remove")}</UiButton> : null}
              </div> : null}
              </article>
            );
          })}
        </div>
      </section>

    </div>
  );
}
