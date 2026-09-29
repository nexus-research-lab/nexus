// INPUT: 平台管理员身份与独立部署用户 API。
// OUTPUT: Web 用户创建与目录；未知写入只按原用户名核对，不自动重试。
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
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { UiBadge } from "@/shared/ui/display/badge";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput, UiNativeSelect, UiSearchInput } from "@/shared/ui/form/form-control";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function DeploymentMembersPanel() {
  const { t } = useI18n();
  const fieldId = useId();
  const { status } = useAuth();
  const [members, setMembers] = useState<ControlDeploymentMember[]>([]);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [blocked, setBlocked] = useState(false);
  const [feedback, setFeedback] = useState<{ message: string; error: boolean } | null>(null);
  const transaction = useRef(false);
  const unknownUsername = useRef<string | null>(null);
  const form = useRef<HTMLFormElement>(null);

  const load = useCallback(async () => {
    if (transaction.current) return;
    transaction.current = true;
    setBusy(true);
    try {
      const values = await listControlDeploymentMembersApi();
      setMembers(values);
      setLoaded(true);
      if (unknownUsername.current && values.some((member) => member.username === unknownUsername.current)) {
        unknownUsername.current = null;
        setBlocked(false);
        form.current?.reset();
        setFeedback({ message: t("deployment_members.reconciled"), error: false });
      } else {
        setFeedback(unknownUsername.current ? { message: t("deployment_members.unknown"), error: true } : null);
      }
    } catch {
      setFeedback({ message: t("deployment_members.load_failed"), error: true });
    } finally {
      transaction.current = false;
      setBusy(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  const create = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (transaction.current || unknownUsername.current || !loaded) return;
    const data = new FormData(event.currentTarget);
    const username = String(data.get("username")).trim().toLowerCase();
    const password = String(data.get("password"));
    if (password !== data.get("confirm_password")) {
      setFeedback({ message: t("members.validation_confirm"), error: true });
      return;
    }
    transaction.current = true;
    setBusy(true);
    setFeedback(null);
    try {
      const member = await createControlDeploymentMemberApi({
        username,
        display_name: String(data.get("display_name")).trim(),
        password,
        role: data.get("role") === "admin" && status?.role === "owner" ? "admin" : "member",
      });
      setMembers((current) => [...current, member]);
      form.current?.reset();
      setFeedback({ message: t("deployment_members.created"), error: false });
    } catch (error) {
      // 明确拒绝才可修改表单；网络或服务端未知结果必须先核对唯一用户名。
      const rejected = error instanceof UnauthorizedError || (error instanceof ApiRequestError && [400, 403, 409].includes(error.status));
      if (!rejected) {
        unknownUsername.current = username;
        setBlocked(true);
      }
      setFeedback({ message: t(rejected ? "deployment_members.create_failed" : "deployment_members.unknown"), error: true });
    } finally {
      form.current?.querySelectorAll<HTMLInputElement>('input[type="password"]').forEach((input) => { input.value = ""; });
      transaction.current = false;
      setBusy(false);
    }
  };

  const matcher = createUiSearchMatcher(query);
  const visibleMembers = members.filter((member) => matcher.matches([member.username, member.display_name]));
  return (
    <div className="grid min-w-0 gap-4">
      <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t("deployment_members.description")}</p>
      <UiDisclosure label={t("deployment_members.create")} leading={<Plus className="h-4 w-4" />} variant="inline">
        <form ref={form} onSubmit={(event) => void create(event)}>
          <fieldset disabled={busy || blocked || !loaded} className="grid min-w-0 gap-4 py-4 sm:grid-cols-2">
            <UiField htmlFor={`${fieldId}-username`} label={t("members.username")} required><UiInput id={`${fieldId}-username`} name="username" autoComplete="off" required minLength={3} maxLength={64} pattern="[a-zA-Z0-9_.\-]+" /></UiField>
            <UiField htmlFor={`${fieldId}-display_name`} label={t("members.display_name")}><UiInput id={`${fieldId}-display_name`} name="display_name" autoComplete="off" maxLength={128} /></UiField>
            <UiField htmlFor={`${fieldId}-password`} label={t("members.password")} required><UiInput id={`${fieldId}-password`} name="password" type="password" autoComplete="new-password" required minLength={8} /></UiField>
            <UiField htmlFor={`${fieldId}-confirm_password`} label={t("members.confirm_password")} required><UiInput id={`${fieldId}-confirm_password`} name="confirm_password" type="password" autoComplete="new-password" required minLength={8} /></UiField>
            <UiField htmlFor={`${fieldId}-role`} label={t("deployment_members.role")}><UiNativeSelect id={`${fieldId}-role`} name="role" defaultValue="member">
              <option value="member">{t("settings.personal.role_member")}</option>
              {status?.role === "owner" ? <option value="admin">{t("settings.personal.role_admin")}</option> : null}
            </UiNativeSelect></UiField>
            <div className="flex items-end"><UiButton type="submit" variant="solid" tone="primary">{t("deployment_members.create")}</UiButton></div>
          </fieldset>
        </form>
      </UiDisclosure>
      {feedback ? <UiInlineNotice message={feedback.message} tone={feedback.error ? "warning" : "neutral"} /> : null}
      <div className="flex flex-wrap items-center gap-3">
        <UiSearchInput className="min-w-0 flex-1" value={query} onChange={setQuery} placeholder={t("deployment_members.search")} aria-label={t("deployment_members.search")} />
        <UiButton size="sm" variant="text" disabled={busy} onClick={() => void load()}>
          {busy ? <span className={getUiSpinnerClassName({ size: "sm" })} /> : <RefreshCw className="h-4 w-4" />}{t("members.refresh")}
        </UiButton>
      </div>
      <ul aria-label={t("operations.tabs.deployment_members")} className="min-w-0 divide-y divide-(--divider-subtle-color)">
        {visibleMembers.map((member) => <li key={member.user_id} className="flex min-w-0 flex-wrap items-center gap-3 py-3">
          <div className="min-w-0 flex-1 break-words">
            <p className={getUiTypographyClassName({ role: "body" })}>{member.display_name || member.username}</p>
            <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>@{member.username}</p>
          </div>
          <span className={getUiTypographyClassName({ role: "supporting" })}>{t(`settings.personal.role_${member.role}`)}</span>
          <UiBadge size="xs" tone={member.membership_status === "active" && !member.web_access_disabled ? "default" : "warning"}>
            {t(member.membership_status !== "active" ? "deployment_members.revoked" : member.web_access_disabled ? "deployment_members.web_disabled" : "deployment_members.web_enabled")}
          </UiBadge>
        </li>)}
      </ul>
      {loaded && visibleMembers.length === 0 ? <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t("members.search_empty")}</p> : null}
    </div>
  );
}
