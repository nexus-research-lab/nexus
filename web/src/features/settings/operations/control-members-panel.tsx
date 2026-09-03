// INPUT: Control 部署成员、当前身份权限和创建/更新成员命令。
// OUTPUT: 成员目录、创建表单、角色/状态控制与操作反馈。
// POS: Operations 成员管理用例；不拥有认证资源或通用表单视觉。
"use client";

import { RefreshCw, UserPlus, UsersRound } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";

import {
  createControlMemberApi,
  listControlMembersApi,
  updateControlMemberApi,
  type ControlDeploymentMember,
  type ControlMemberRole,
} from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import {
  UiField,
  UiInput,
  UiNativeSelect,
} from "@/shared/ui/form/form-control";

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
    <div className="mx-auto w-full max-w-[980px] pb-8">
      <header className="flex flex-col gap-3 border-b border-(--divider-subtle-color) pb-5 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex items-center gap-2 text-(--text-strong)">
            <UsersRound className="h-4 w-4" />
            <h2 className="text-base font-semibold">{t("members.title")}</h2>
          </div>
          <p className="mt-1.5 max-w-[680px] text-sm leading-6 text-(--text-muted)">
            {t("members.description")}
          </p>
        </div>
        <UiButton className="self-start" disabled={loading} onClick={() => void loadMembers()} size="sm" variant="text">
          <RefreshCw className={loading ? "h-3.5 w-3.5 animate-spin" : "h-3.5 w-3.5"} />
          {t("members.refresh")}
        </UiButton>
      </header>

      <form className="border-b border-(--divider-subtle-color) py-5" onSubmit={createMember}>
        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-(--text-strong)">
          <UserPlus className="h-4 w-4" />
          {t("members.create_title")}
        </div>
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
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
            <UiNativeSelect
              id="member-role"
              onChange={(event) => setDraft((current) => ({ ...current, role: event.target.value as ControlMemberRole }))}
              value={draft.role}
              variant="surface"
            >
              <option value="member">{t("settings.personal.role_member")}</option>
              {canCreateElevatedRole ? <option value="admin">{t("settings.personal.role_admin")}</option> : null}
              {canCreateElevatedRole ? <option value="owner">{t("settings.personal.role_owner")}</option> : null}
            </UiNativeSelect>
          </UiField>
        </div>
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
          <p className={draftError ? "text-xs text-(--destructive)" : "text-xs text-(--text-muted)"}>
            {draftError ?? t("members.create_hint")}
          </p>
          <UiButton disabled={Boolean(draftError) || pendingKey !== null} size="sm" tone="primary" type="submit" variant="solid">
            {pendingKey === "create" ? t("members.creating") : t("members.create")}
          </UiButton>
        </div>
      </form>

      {feedback ? (
        <p className={feedback.tone === "success" ? "py-3 text-sm text-(--text-muted)" : "py-3 text-sm text-(--destructive)"} role="status">
          {feedback.message}
        </p>
      ) : null}

      <section aria-label={t("members.list_label")} className="divide-y divide-(--divider-subtle-color)">
        {loading && members.length === 0 ? (
          <p className="py-8 text-sm text-(--text-muted)">{t("members.loading")}</p>
        ) : null}
        {!loading && members.length === 0 ? (
          <p className="py-8 text-sm text-(--text-muted)">{t("members.empty")}</p>
        ) : null}
        {members.map((member) => {
          const isSelf = member.user_id === status?.user_id;
          const isPending = pendingKey === `update:${member.user_id}`;
          const canEditRole = status?.role === "owner" && !isSelf;
          const canToggle = !isSelf && (status?.role === "owner" || member.role === "member");
          return (
            <article className="grid items-center gap-4 py-4 md:grid-cols-[minmax(0,1fr)_150px_120px]" key={member.user_id}>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <p className="truncate text-sm font-semibold text-(--text-strong)">{member.display_name || member.username}</p>
                  {isSelf ? <span className="text-xs text-(--text-soft)">{t("members.current")}</span> : null}
                </div>
                <p className="mt-1 truncate text-xs text-(--text-muted)">@{member.username}</p>
              </div>
              <UiNativeSelect
                aria-label={t("members.role")}
                controlSize="sm"
                disabled={!canEditRole || isPending}
                onChange={(event) => void updateMember(member, { role: event.target.value as ControlMemberRole })}
                value={member.role}
                variant="surface"
              >
                <option value="member">{t("settings.personal.role_member")}</option>
                <option value="admin">{t("settings.personal.role_admin")}</option>
                <option value="owner">{t("settings.personal.role_owner")}</option>
              </UiNativeSelect>
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
