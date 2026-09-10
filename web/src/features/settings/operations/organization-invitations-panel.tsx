// INPUT: 当前 Organization 管理角色与 Control 邀请 API。
// OUTPUT: 一次性邀请链接、邀请审计列表与撤销动作。
// POS: 组织成员管理页中的拉人入口；不创建账号或发送邮件。
"use client";

import { Copy, Link2 } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import {
  createControlOrganizationInvitationApi,
  listControlOrganizationInvitationsApi,
  revokeControlOrganizationInvitationApi,
  type ControlOrganizationInvitation,
} from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

export function OrganizationInvitationsPanel() {
  const { t } = useI18n();
  const { status } = useAuth();
  const [invitations, setInvitations] = useState<ControlOrganizationInvitation[]>([]);
  const [role, setRole] = useState<"admin" | "member">("member");
  const [link, setLink] = useState("");
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  const pendingRef = useRef(false);
  const load = useCallback(async () => {
    setInvitations(await listControlOrganizationInvitationsApi());
    setFailed(false);
  }, []);
  useEffect(() => { void load().catch(() => undefined); }, [load]);

  const create = async () => {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPending(true);
    setFailed(false);
    try {
      const invitation = await createControlOrganizationInvitationApi(role);
      setInvitations((current) => [invitation, ...current]);
      setLink(invitation.join_url);
    } catch {
      setFailed(true);
    } finally {
      pendingRef.current = false;
      setPending(false);
    }
  };
  const revoke = async (invitationID: string) => {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPending(true);
    setFailed(false);
    try {
      await revokeControlOrganizationInvitationApi(invitationID);
      await load();
    } catch {
      setFailed(true);
    } finally {
      pendingRef.current = false;
      setPending(false);
    }
  };

  return (
    <UiDisclosure label={t("members.invite_title")} variant="panel">
      <div className="grid gap-4">
        <div className="grid items-end gap-3 sm:grid-cols-[180px_auto]">
          <UiField label={t("members.role")}>
            <UiSelectMenu
              ariaLabel={t("members.role")}
              disabled={pending}
              onChange={(value) => setRole(value as "admin" | "member")}
              options={status?.role === "owner" ? [
                { value: "member", label: t("settings.personal.role_member") },
                { value: "admin", label: t("settings.personal.role_admin") },
              ] : [{ value: "member", label: t("settings.personal.role_member") }]}
              value={role}
            />
          </UiField>
          <UiButton disabled={pending} onClick={() => void create()} tone="primary" variant="solid">
            <Link2 className="h-4 w-4" />
            {pending ? t("members.invite_creating") : t("members.invite_create")}
          </UiButton>
        </div>
        {link ? (
          <div className="flex gap-2">
            <UiInput aria-label={t("members.invite_link")} readOnly value={link} />
            <UiButton onClick={() => void navigator.clipboard.writeText(link).catch(() => setFailed(true))} variant="outline">
              <Copy className="h-4 w-4" />{t("members.invite_copy")}
            </UiButton>
          </div>
        ) : null}
        {failed ? <UiInlineNotice message={t("members.invite_failed")} tone="danger" /> : null}
        <div className="divide-y divide-(--divider-subtle-color)">
          {invitations.map((invitation) => {
            const active = !invitation.accepted_at && !invitation.revoked_at && new Date(invitation.expires_at).getTime() > Date.now();
            return (
              <div className="flex items-center justify-between gap-3 py-3" key={invitation.invitation_id}>
                <div className="flex min-w-0 items-center gap-2">
                  <span>{t(invitation.role === "admin" ? "settings.personal.role_admin" : "settings.personal.role_member")}</span>
                  <UiBadge size="xs" tone={active ? "default" : "warning"}>
                    {t(active ? "members.invite_pending" : invitation.accepted_at ? "members.invite_accepted" : "members.invite_inactive")}
                  </UiBadge>
                </div>
                {active ? (
                  <UiButton disabled={pending} onClick={() => void revoke(invitation.invitation_id)} size="sm" tone="danger" variant="text">
                    {t("members.invite_revoke")}
                  </UiButton>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    </UiDisclosure>
  );
}
