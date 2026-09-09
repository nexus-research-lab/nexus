// INPUT: Control 部署成员、当前身份权限和创建/更新成员命令。
// OUTPUT: 成员目录、创建表单、角色/状态控制与操作反馈。
// POS: Operations 成员管理用例；不拥有认证资源或通用表单视觉。
"use client";

import { RefreshCw } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";

import {
  SETTINGS_ITEM_TITLE_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import {
  createControlMemberApi,
  listControlMembersApi,
  updateControlMemberApi,
  type ControlDeploymentMember,
  type ControlMemberRole,
} from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiField,
  UiInput,
} from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface MemberDraft {
  username: string;
  displayName: string;
  password: string;
  confirmPassword: string;
  role: ControlMemberRole;
}

const EMPTY_DRAFT: MemberDraft = {
  username: "",
  displayName: "",
  password: "",
  confirmPassword: "",
  role: "member",
};

type Feedback = { tone: "success" | "error"; message: string } | null;

export function ControlMembersPanel() {
  const { t } = useI18n();
  const { status } = useAuth();
  const [members, setMembers] = useState<ControlDeploymentMember[]>([]);
  const [draft, setDraft] = useState<MemberDraft>(EMPTY_DRAFT);
  const [loading, setLoading] = useState(true);
  const [pendingKey, setPendingKey] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const canCreateElevatedRole = status?.role === "owner";
  const roleOptions = [
    { value: "member", label: t("settings.personal.role_member") },
    { value: "admin", label: t("settings.personal.role_admin") },
    { value: "owner", label: t("settings.personal.role_owner") },
  ];
  const draftError = useMemo(() => {
    if (draft.username.trim().length < 3) return t("members.validation_username");
    if (draft.password.length < 8) return t("members.validation_password");
    if (draft.password !== draft.confirmPassword) return t("members.validation_confirm");
    return null;
  }, [draft, t]);

  const loadMembers = useCallback(async () => {
    setLoading(true);
    try {
      setMembers(await listControlMembersApi());
    } catch {
      setFeedback({ tone: "error", message: t("members.load_failed") });
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadMembers();
  }, [loadMembers]);

  const createMember = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (draftError) return;
    setPendingKey("create");
    setFeedback(null);
    try {
      const member = await createControlMemberApi({
        username: draft.username.trim(),
        display_name: draft.displayName.trim(),
        password: draft.password,
        role: draft.role,
      });
      setMembers((current) => [...current, member]);
      setDraft(EMPTY_DRAFT);
      setFeedback({ tone: "success", message: t("members.create_success") });
    } catch {
      setFeedback({ tone: "error", message: t("members.create_unknown") });
      await loadMembers();
    } finally {
      setPendingKey(null);
    }
  };

  const updateMember = async (
    member: ControlDeploymentMember,
    change: { role?: ControlMemberRole; status?: "active" | "revoked" },
  ) => {
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
      await loadMembers();
    } finally {
      setPendingKey(null);
    }
  };

  return (
    <div className="@container/members grid w-full min-w-0 gap-4 pb-8">
      <WorkspaceContentHeader
        className="mb-0 max-sm:[&_h1]:hidden"
        title={t("members.title")}
        description={t("members.description")}
        actions={(
          <UiButton disabled={loading} onClick={() => void loadMembers()} size="sm" variant="text">
            <RefreshCw className={loading ? getUiSpinnerClassName({ size: "sm" }) : "h-3.5 w-3.5"} />
            {t("members.refresh")}
          </UiButton>
        )}
      />

      <UiDisclosure label={t("members.create_title")} variant="panel">
        <form onSubmit={createMember}>
          <div className="grid items-end gap-4 @min-[480px]/members:grid-cols-2">
            <UiField htmlFor="member-username" label={t("members.username")} required>
              <UiInput id="member-username" maxLength={64} minLength={3} onChange={(event) => setDraft((current) => ({ ...current, username: event.target.value }))} pattern="[a-z0-9._-]+" required value={draft.username} variant="surface" />
            </UiField>
            <UiField htmlFor="member-display-name" label={t("members.display_name")}>
              <UiInput id="member-display-name" maxLength={128} onChange={(event) => setDraft((current) => ({ ...current, displayName: event.target.value }))} value={draft.displayName} variant="surface" />
            </UiField>
            <UiField htmlFor="member-password" label={t("members.password")} required>
              <UiInput autoComplete="new-password" id="member-password" minLength={8} onChange={(event) => setDraft((current) => ({ ...current, password: event.target.value }))} required type="password" value={draft.password} variant="surface" />
            </UiField>
            <UiField htmlFor="member-confirm-password" label={t("members.confirm_password")} required>
              <UiInput autoComplete="new-password" id="member-confirm-password" minLength={8} onChange={(event) => setDraft((current) => ({ ...current, confirmPassword: event.target.value }))} required type="password" value={draft.confirmPassword} variant="surface" />
            </UiField>
            <UiField htmlFor="member-role" label={t("members.role")} required>
              <UiSelectMenu
                ariaLabel={t("members.role")}
                id="member-role"
                onChange={(value) => setDraft((current) => ({ ...current, role: value as ControlMemberRole }))}
                options={canCreateElevatedRole ? roleOptions : roleOptions.filter((option) => option.value === "member")}
                size="md"
                value={draft.role}
              />
            </UiField>
          </div>
          <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
            <p className={getUiTypographyClassName({
              role: "caption",
              tone: draft.username && draft.password && draftError ? "danger" : "muted",
            })}>
              {draft.username && draft.password ? draftError ?? t("members.create_hint") : t("members.create_hint")}
            </p>
            <UiButton disabled={Boolean(draftError) || pendingKey !== null} size="md" tone="primary" type="submit" variant="solid">
              {pendingKey === "create" ? t("members.creating") : t("members.create")}
            </UiButton>
          </div>
        </form>
      </UiDisclosure>

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

      <section aria-label={t("members.list_label")} className="divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
        {loading && members.length === 0 ? (
          <UiResourceState
            size="sm"
            state="loading"
            title={t("members.loading")}
            variant="plain"
          />
        ) : null}
        {!loading && members.length === 0 ? (
          <UiResourceState
            size="sm"
            state="empty"
            title={t("members.empty")}
            variant="plain"
          />
        ) : null}
        {members.map((member) => {
          const isSelf = member.user_id === status?.user_id;
          const isPending = pendingKey === `update:${member.user_id}`;
          const canEditRole = status?.role === "owner" && !isSelf;
          const canToggle = !isSelf && (status?.role === "owner" || member.role === "member");
          return (
            <article className="grid items-center gap-3 py-4 @min-[560px]/members:grid-cols-[minmax(0,1fr)_160px_96px]" key={member.user_id}>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <p className={cn("min-w-0 wrap-anywhere", SETTINGS_ITEM_TITLE_CLASS_NAME)}>
                    {member.display_name || member.username}
                  </p>
                  <UiBadge size="xs" tone={member.membership_status === "active" ? "default" : "warning"}>
                    {t(member.membership_status === "active" ? "members.active" : "members.revoked")}
                  </UiBadge>
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
                disabled={!canEditRole || isPending}
                onChange={(value) => void updateMember(member, { role: value as ControlMemberRole })}
                options={roleOptions}
                size="sm"
                value={member.role}
              />
              <UiButton
                disabled={!canToggle || isPending}
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
            </article>
          );
        })}
      </section>
    </div>
  );
}
