// INPUT: 当前 Organization 身份、成员目录、管理角色与 Control 邀请 API。
// OUTPUT: 统一组织治理页、邀请弹窗、成员管理、角色说明与邀请记录。
// POS: Operations 组织治理页；组织成员身份是页面的信息架构根。
"use client";

import { Copy, UserPlus } from "lucide-react";
import { useCallback, useEffect, useId, useRef, useState } from "react";

import {
  SETTINGS_GROUP_CLASS_NAME,
  SETTINGS_ITEM_TITLE_CLASS_NAME,
  SETTINGS_SECTION_TITLE_CLASS_NAME,
} from "@/features/settings/shared/settings-panel-ui";
import {
  createControlOrganizationInvitationApi,
  listControlOrganizationInvitationsApi,
  revokeControlOrganizationInvitationApi,
  type ControlOrganizationInvitation,
} from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { OrganizationMembersPanel } from "./control-members-panel";

export function OrganizationPanel() {
  const { locale, t } = useI18n();
  const { status } = useAuth();
  const titleId = useId();
  const [invitations, setInvitations] = useState<ControlOrganizationInvitation[]>([]);
  const [role, setRole] = useState<"admin" | "member">("member");
  const [link, setLink] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [actionFailed, setActionFailed] = useState(false);
  const [copied, setCopied] = useState(false);
  const [pending, setPending] = useState(false);
  const pendingRef = useRef(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadFailed(false);
    try {
      setInvitations(await listControlOrganizationInvitationsApi());
    } catch {
      setLoadFailed(true);
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => { void load(); }, [load]);

  const openInviteDialog = () => {
    setRole("member");
    setLink("");
    setActionFailed(false);
    setCopied(false);
    setDialogOpen(true);
  };
  const closeInviteDialog = () => {
    if (!pending) setDialogOpen(false);
  };
  const create = async () => {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPending(true);
    setActionFailed(false);
    try {
      const invitation = await createControlOrganizationInvitationApi(role);
      setInvitations((current) => [invitation, ...current]);
      setLink(invitation.join_url);
    } catch {
      setActionFailed(true);
    } finally {
      pendingRef.current = false;
      setPending(false);
    }
  };
  const revoke = async (invitationID: string) => {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPending(true);
    setActionFailed(false);
    try {
      await revokeControlOrganizationInvitationApi(invitationID);
      await load();
    } catch {
      setActionFailed(true);
    } finally {
      pendingRef.current = false;
      setPending(false);
    }
  };
  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
    } catch {
      setActionFailed(true);
    }
  };

  const roleLabel = (value: string | null | undefined) => t(
    value === "owner"
      ? "settings.personal.role_owner"
      : value === "admin"
        ? "settings.personal.role_admin"
        : "settings.personal.role_member",
  );

  return (
    <div className="@container/organization grid w-full min-w-0 gap-6 pb-8">
      <WorkspaceContentHeader
        className="mb-0 max-sm:[&_h1]:hidden"
        title={status?.organization_name || t("organization.name_unavailable")}
        description={t("organization.header_description", { role: roleLabel(status?.role) })}
        actions={(
          <UiButton onClick={openInviteDialog} size="sm" tone="primary" variant="solid">
            <UserPlus className="h-4 w-4" />
            {t("organization.invite_action")}
          </UiButton>
        )}
      />

      <section aria-labelledby={`${titleId}-members`}>
        <div className="mb-3">
          <h2 className={SETTINGS_SECTION_TITLE_CLASS_NAME} id={`${titleId}-members`}>{t("members.title")}</h2>
          <p className={cn("mt-1", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>{t("members.description")}</p>
        </div>
        <div className="grid items-start gap-4 @min-[960px]/organization:grid-cols-[minmax(0,1fr)_320px]">
          <OrganizationMembersPanel />
          <aside aria-labelledby={`${titleId}-roles`} className={SETTINGS_GROUP_CLASS_NAME}>
            <h3 className={cn("border-b border-(--divider-subtle-color) px-4 py-3", SETTINGS_SECTION_TITLE_CLASS_NAME)} id={`${titleId}-roles`}>
              {t("organization.roles_title")}
            </h3>
            <div className="divide-y divide-(--divider-subtle-color)">
              {(["owner", "admin", "member"] as const).map((memberRole) => (
                <div className="px-4 py-3" key={memberRole}>
                  <p className={SETTINGS_ITEM_TITLE_CLASS_NAME}>{roleLabel(memberRole)}</p>
                  <p className={cn("mt-1", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
                    {t(`organization.role_${memberRole}_description`)}
                  </p>
                </div>
              ))}
            </div>
          </aside>
        </div>
      </section>

      <section aria-labelledby={`${titleId}-invitations`}>
        <UiDisclosure
          contentClassName="p-0"
          label={<span id={`${titleId}-invitations`}>{t("organization.invitations_title")}</span>}
          meta={<UiBadge size="xs">{invitations.length}</UiBadge>}
          surfaceTone="subtle"
          variant="panel"
        >
          <div className="divide-y divide-(--divider-subtle-color)">
            {loading && invitations.length === 0 ? (
              <UiResourceState size="sm" state="loading" title={t("organization.invitations_loading")} variant="plain" />
            ) : null}
            {!loading && loadFailed ? (
              <UiResourceState
                impact={t("organization.invitations_load_failed")}
                primaryAction={{ label: t("members.refresh"), onClick: () => void load() }}
                size="sm"
                state="error"
                title={t("organization.invitations_load_failed_title")}
                variant="plain"
              />
            ) : null}
            {!loading && !loadFailed && invitations.length === 0 ? (
              <UiResourceState description={t("organization.invitations_empty_description")} size="sm" state="empty" title={t("organization.invitations_empty")} variant="plain" />
            ) : null}
            {invitations.map((invitation) => {
              const active = !invitation.accepted_at
                && !invitation.revoked_at
                && new Date(invitation.expires_at).getTime() > Date.now();
              return (
                <article className="flex flex-wrap items-center justify-between gap-3 px-4 py-3" key={invitation.invitation_id}>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={SETTINGS_ITEM_TITLE_CLASS_NAME}>{roleLabel(invitation.role)}</span>
                    <UiBadge size="xs" tone={active ? "default" : "warning"}>
                      {t(active ? "members.invite_pending" : invitation.accepted_at ? "members.invite_accepted" : "members.invite_inactive")}
                    </UiBadge>
                  </div>
                  <p className={cn("mt-1", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
                    {t("organization.invite_created_at", { date: new Date(invitation.created_at).toLocaleDateString(locale) })}
                  </p>
                </div>
                {active ? (
                  <UiButton disabled={pending} onClick={() => void revoke(invitation.invitation_id)} size="sm" tone="danger" variant="text">
                    {t("members.invite_revoke")}
                  </UiButton>
                ) : null}
                </article>
              );
            })}
          </div>
        </UiDisclosure>
        {actionFailed && !dialogOpen ? <UiInlineNotice className="mt-3" message={t("members.invite_failed")} tone="danger" /> : null}
      </section>

      {dialogOpen ? (
        <UiDialogPortal>
          <UiDialogBackdrop labelledBy={`${titleId}-dialog`} onClose={closeInviteDialog}>
            <UiDialogShell size="md" viewport="adaptiveMax">
              <UiDialogHeader appearance="plain" onClose={closeInviteDialog} title={t("organization.invite_title")} titleId={`${titleId}-dialog`} />
              <UiDialogBody className="space-y-4 px-5" scrollable>
                {link ? (
                  <UiField description={t("organization.invite_link_hint")} label={t("members.invite_link")}>
                    <div className="flex min-w-0 gap-2">
                      <UiInput aria-label={t("members.invite_link")} className="min-w-0 flex-1" readOnly value={link} />
                      <UiButton className="shrink-0 whitespace-nowrap" onClick={() => void copyLink()} type="button" variant="outline">
                        <Copy className="h-4 w-4" />
                        {copied ? t("organization.invite_copied") : t("members.invite_copy")}
                      </UiButton>
                    </div>
                  </UiField>
                ) : (
                  <UiField description={t("organization.invite_role_hint")} label={t("members.role")}>
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
                )}
                {actionFailed ? <UiInlineNotice message={t("members.invite_failed")} tone="danger" /> : null}
              </UiDialogBody>
              <UiDialogFooter appearance="plain">
                {link ? (
                  <UiButton onClick={closeInviteDialog} tone="primary" type="button" variant="solid">{t("common.close")}</UiButton>
                ) : (
                  <>
                    <UiButton disabled={pending} onClick={closeInviteDialog} type="button" variant="surface">{t("common.cancel")}</UiButton>
                    <UiButton disabled={pending} onClick={() => void create()} tone="primary" type="button" variant="solid">
                      {pending ? t("members.invite_creating") : t("members.invite_create")}
                    </UiButton>
                  </>
                )}
              </UiDialogFooter>
            </UiDialogShell>
          </UiDialogBackdrop>
        </UiDialogPortal>
      ) : null}
    </div>
  );
}
