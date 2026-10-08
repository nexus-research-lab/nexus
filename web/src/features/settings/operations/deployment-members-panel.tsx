// INPUT: 平台管理员身份与独立部署用户 API。
// OUTPUT: 部署用户目录与创建弹窗；未知写入只按原用户名核对，不自动重试。
// POS: 运营的部署用户页面；不读取组织身份或替用户加入组织。
"use client";

import { Plus, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useId, useRef, useState, type FormEvent } from "react";
import {
  createControlDeploymentMemberApi,
  listControlDeploymentMembersApi,
  type ControlDeploymentMember,
} from "@/lib/api/account/control-api";
import { ApiRequestError, UnauthorizedError } from "@/lib/api/core/http-error";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDialogBackdrop, UiDialogBody, UiDialogCloseButton, UiDialogFooter, UiDialogFormShell, UiDialogHeader, UiDialogPortal } from "@/shared/ui/dialog/dialog";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput, UiNativeSelect, UiSearchInput } from "@/shared/ui/form/form-control";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

const DIRECTORY_COLUMNS = "@min-[560px]/deployment-users:grid-cols-[minmax(0,1fr)_120px_140px]";

export function DeploymentMembersPanel() {
  const { t } = useI18n();
  const fieldId = useId();
  const { status } = useAuth();
  const [members, setMembers] = useState<ControlDeploymentMember[]>([]);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(true);
  const [loaded, setLoaded] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [blocked, setBlocked] = useState(false);
  const [open, setOpen] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [passwordMismatch, setPasswordMismatch] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const transaction = useRef(false);
  const unknownUsername = useRef<string | null>(null);
  const usernameInput = useRef<HTMLInputElement>(null);
  const confirmInput = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    if (transaction.current) return;
    transaction.current = true;
    setBusy(true);
    setLoadFailed(false);
    try {
      const values = await listControlDeploymentMembersApi();
      setMembers(values);
      setLoaded(true);
      if (unknownUsername.current && values.some((member) => member.username === unknownUsername.current)) {
        setQuery(unknownUsername.current);
        unknownUsername.current = null;
        setBlocked(false);
        setFeedback(t("deployment_members.reconciled"));
      }
    } catch {
      setLoadFailed(true);
    } finally {
      transaction.current = false;
      setBusy(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  const close = () => {
    if (!transaction.current) setOpen(false);
  };
  const openCreate = () => {
    setFormError(null);
    setPasswordMismatch(false);
    setOpen(true);
  };
  const create = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (transaction.current || unknownUsername.current || !loaded) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const username = String(data.get("username")).trim().toLowerCase();
    const password = String(data.get("password"));
    if (password !== data.get("confirm_password")) {
      setPasswordMismatch(true);
      confirmInput.current?.focus();
      return;
    }
    transaction.current = true;
    setBusy(true);
    setFormError(null);
    setFeedback(null);
    try {
      const member = await createControlDeploymentMemberApi({
        username,
        display_name: String(data.get("display_name")).trim(),
        password,
        role: data.get("role") === "admin" && status?.role === "owner" ? "admin" : "member",
      });
      setMembers((current) => [...current, member]);
      setQuery("");
      setOpen(false);
      setFeedback(t("deployment_members.created"));
    } catch (error) {
      // 明确拒绝留在表单修正；未知结果退出敏感表单，目录保留原用户名核对入口。
      const rejected = error instanceof UnauthorizedError || (error instanceof ApiRequestError && [400, 403, 409].includes(error.status));
      if (rejected) {
        setFormError(t("deployment_members.create_failed"));
      } else {
        unknownUsername.current = username;
        setBlocked(true);
        setOpen(false);
      }
    } finally {
      form.querySelectorAll<HTMLInputElement>('input[type="password"]').forEach((input) => { input.value = ""; });
      transaction.current = false;
      setBusy(false);
    }
  };

  const matcher = createUiSearchMatcher(query);
  const visibleMembers = members.filter((member) => matcher.matches([member.username, member.display_name]));
  const refreshAction = {
    label: t("members.refresh"),
    icon: <RefreshCw className="h-4 w-4" />,
    pending: busy,
    disabled: busy,
    onClick: () => void load(),
  };

  return (
    <div className="@container/deployment-users grid min-w-0 gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-(--divider-subtle-color) pb-3">
        <UiButton size="sm" variant="solid" tone="primary" disabled={busy || blocked || !loaded || loadFailed} onClick={openCreate}>
          <Plus className="h-4 w-4" />{t("deployment_members.create")}
        </UiButton>
        <UiSearchInput className="min-w-0 w-full @min-[560px]/deployment-users:w-60" value={query} onChange={setQuery} placeholder={t("deployment_members.search")} aria-label={t("deployment_members.search")} />
      </div>
      {blocked ? (
        <UiInlineNotice tone="warning" title={`@${unknownUsername.current}`} message={[loadFailed ? t("deployment_members.load_failed") : null, t("deployment_members.unknown")].filter(Boolean).join(" ")} action={refreshAction} />
      ) : feedback ? <UiInlineNotice message={feedback} /> : null}
      {loadFailed && !blocked ? <UiInlineNotice tone="warning" message={t("deployment_members.load_failed")} action={refreshAction} /> : null}
      {busy && !loaded ? <UiResourceState state="loading" size="sm" variant="plain" title={t("common.loading")} /> : null}
      {loaded && !loadFailed && members.length === 0 ? (
        <UiResourceState state="empty" size="sm" variant="plain" title={t("deployment_members.empty")} description={t("deployment_members.description")} />
      ) : null}
      {loaded && members.length > 0 && visibleMembers.length === 0 ? (
        <UiResourceState state="empty" size="sm" variant="plain" title={t("members.search_empty")} description={t("members.search_empty_description")} />
      ) : null}
      {visibleMembers.length > 0 ? <section aria-label={t("operations.tabs.deployment_members")} className="min-w-0">
        <div aria-hidden="true" className={cn("hidden items-center gap-4 border-b border-(--divider-subtle-color) px-3 pb-2 @min-[560px]/deployment-users:grid", DIRECTORY_COLUMNS, getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
          <span>{t("deployment_members.column_user")}</span>
          <span>{t("deployment_members.role")}</span>
          <span>{t("members.column_access")}</span>
        </div>
        <ul className="divide-y divide-(--divider-subtle-color)">
          {visibleMembers.map((member) => <li key={member.user_id} className={cn("grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-2 px-3 py-4 hover:bg-(--surface-interactive-hover-background)", DIRECTORY_COLUMNS)}>
            <div className="col-span-2 min-w-0 @min-[560px]/deployment-users:col-span-1">
              <div className="flex flex-wrap items-center gap-2">
                <p className={cn("min-w-0 wrap-anywhere", getUiTypographyClassName({ role: "body", weight: "medium" }))}>{member.display_name || member.username}</p>
                {member.user_id === (status?.control_user_id ?? status?.user_id) ? <span className={getUiTypographyClassName({ role: "caption", tone: "muted" })}>{t("members.current")}</span> : null}
              </div>
              <p className={cn("mt-1 wrap-anywhere", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>@{member.username}</p>
            </div>
            <span className={getUiTypographyClassName({ role: "supporting" })}>{t(`settings.personal.role_${member.role}`)}</span>
            <div className="min-w-0">
              <UiBadge size="xs" tone={member.membership_status === "active" && !member.web_access_disabled ? "default" : "warning"}>
                {t(member.membership_status !== "active" ? "deployment_members.revoked" : member.web_access_disabled ? "deployment_members.web_disabled" : "deployment_members.web_enabled")}
              </UiBadge>
            </div>
          </li>)}
        </ul>
      </section> : null}
      {open ? <UiDialogPortal>
        <UiDialogBackdrop initialFocusRef={usernameInput} onClose={close} closeOnBackdrop={!busy}>
          <UiDialogFormShell size="md" viewport="adaptiveMax" onSubmit={(event) => void create(event)} aria-busy={busy}>
            <UiDialogHeader title={t("deployment_members.create")} actions={<UiDialogCloseButton disabled={busy} onClose={close} />} />
            <UiDialogBody scrollable className="space-y-4">
              <fieldset disabled={busy} className="grid min-w-0 gap-4">
                <UiField htmlFor={`${fieldId}-username`} label={t("members.username")} description={t("deployment_members.username_hint")} required>
                  <UiInput ref={usernameInput} id={`${fieldId}-username`} name="username" autoComplete="off" autoCapitalize="none" spellCheck={false} required minLength={3} maxLength={64} pattern="[a-zA-Z0-9_.\-]+" />
                </UiField>
                <UiField htmlFor={`${fieldId}-display_name`} label={t("members.display_name")}>
                  <UiInput id={`${fieldId}-display_name`} name="display_name" autoComplete="off" maxLength={128} />
                </UiField>
                <UiField htmlFor={`${fieldId}-password`} label={t("members.password")} description={t("deployment_members.password_hint")} required>
                  <UiInput id={`${fieldId}-password`} name="password" type="password" autoComplete="new-password" required minLength={8} onChange={() => setPasswordMismatch(false)} />
                </UiField>
                <UiField htmlFor={`${fieldId}-confirm_password`} label={t("members.confirm_password")} error={passwordMismatch ? t("members.validation_confirm") : undefined} required>
                  <UiInput ref={confirmInput} id={`${fieldId}-confirm_password`} name="confirm_password" type="password" autoComplete="new-password" required minLength={8} onChange={() => setPasswordMismatch(false)} />
                </UiField>
                <UiField htmlFor={`${fieldId}-role`} label={t("deployment_members.role")}>
                  <UiNativeSelect id={`${fieldId}-role`} name="role" defaultValue="member">
                    <option value="member">{t("settings.personal.role_member")}</option>
                    {status?.role === "owner" ? <option value="admin">{t("settings.personal.role_admin")}</option> : null}
                  </UiNativeSelect>
                </UiField>
              </fieldset>
              {formError ? <UiInlineNotice role="alert" tone="warning" message={formError} /> : null}
            </UiDialogBody>
            <UiDialogFooter>
              <UiButton type="button" variant="surface" disabled={busy} onClick={close}>{t("common.cancel")}</UiButton>
              <UiButton type="submit" variant="solid" tone="primary" disabled={busy}>
                {busy ? <span className={getUiSpinnerClassName({ size: "sm" })} /> : null}
                {t(busy ? "deployment_members.creating" : "deployment_members.create")}
              </UiButton>
            </UiDialogFooter>
          </UiDialogFormShell>
        </UiDialogBackdrop>
      </UiDialogPortal> : null}
    </div>
  );
}
