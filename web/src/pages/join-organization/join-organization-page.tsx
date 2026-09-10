// INPUT: URL 中的一次性组织邀请 token 与用户自设账号资料。
// OUTPUT: 邀请预览、注册加入和成功后的统一登录态。
// POS: 无需预先登录的 Organization 加入入口。
"use client";

import { UserPlus } from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState, type FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";

import { AccessPageFrame, AccessPageIntroduction } from "@/features/access/access-page-frame";
import {
  acceptControlOrganizationInvitationApi,
  previewControlOrganizationInvitationApi,
  type ControlOrganizationInvitationPreview,
} from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { APP_ROUTE_PATHS } from "@/shared/navigation/route-paths";
import { UiButton } from "@/shared/ui/button/button";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { AppLoadingState } from "@/shared/ui/layout/app-loading-screen";
import { UiPanel } from "@/shared/ui/panel";

interface JoinDraft {
  username: string;
  displayName: string;
  password: string;
  confirmPassword: string;
}

const INITIAL_DRAFT: JoinDraft = { username: "", displayName: "", password: "", confirmPassword: "" };

export function JoinOrganizationPage() {
  const { t } = useI18n();
  const { token = "" } = useParams();
  const navigate = useNavigate();
  const { refreshStatus } = useAuth();
  const fieldID = useId();
  const pendingRef = useRef(false);
  const [preview, setPreview] = useState<ControlOrganizationInvitationPreview | null>(null);
  const [invalid, setInvalid] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [failed, setFailed] = useState(false);
  const [draft, setDraft] = useState(INITIAL_DRAFT);
  const validation = useMemo(() => {
    if (draft.username.trim().length < 3) return "members.validation_username";
    if (draft.password.length < 8) return "members.validation_password";
    if (draft.password !== draft.confirmPassword) return "members.validation_confirm";
    return null;
  }, [draft]);

  useEffect(() => {
    let active = true;
    void previewControlOrganizationInvitationApi(token)
      .then((value) => { if (active) setPreview(value); })
      .catch(() => { if (active) setInvalid(true); });
    return () => { active = false; };
  }, [token]);

  const setField = (field: keyof JoinDraft, value: string) => {
    setDraft((current) => ({ ...current, [field]: value }));
    setFailed(false);
  };
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (validation || pendingRef.current || !preview) return;
    pendingRef.current = true;
    setSubmitting(true);
    setFailed(false);
    try {
      await acceptControlOrganizationInvitationApi(token, {
        username: draft.username.trim(), display_name: draft.displayName.trim(), password: draft.password,
      });
      await refreshStatus();
      navigate(APP_ROUTE_PATHS.launcher, { replace: true });
    } catch {
      setFailed(true);
    } finally {
      pendingRef.current = false;
      setSubmitting(false);
    }
  };

  return (
    <AccessPageFrame introduction={(
      <AccessPageIntroduction
        backHomeLabel={t("join.back_home")}
        eyebrow={t("join.eyebrow")}
        title={t("join.headline")}
        description={preview ? t("join.description").replace("{organization}", preview.organization_name) : t("join.loading_description")}
      />
    )}>
      <UiPanel padding="lg" radius="lg" variant="filled">
        {!preview && !invalid ? <AppLoadingState message={t("join.loading")} /> : null}
        {invalid ? <UiInlineNotice message={t("join.invalid_description")} title={t("join.invalid_title")} tone="danger" /> : null}
        {preview ? (
          <form className="grid gap-4" onSubmit={submit}>
            <UserPlus className="h-8 w-8 text-(--brand)" />
            <UiField htmlFor={`${fieldID}-username`} label={t("members.username")} required>
              <UiInput id={`${fieldID}-username`} minLength={3} onChange={(event) => setField("username", event.target.value)} pattern="[a-z0-9._-]+" required value={draft.username} />
            </UiField>
            <UiField htmlFor={`${fieldID}-display-name`} label={t("members.display_name")}>
              <UiInput id={`${fieldID}-display-name`} onChange={(event) => setField("displayName", event.target.value)} value={draft.displayName} />
            </UiField>
            <UiField htmlFor={`${fieldID}-password`} label={t("members.password")} required>
              <UiInput autoComplete="new-password" id={`${fieldID}-password`} minLength={8} onChange={(event) => setField("password", event.target.value)} required type="password" value={draft.password} />
            </UiField>
            <UiField htmlFor={`${fieldID}-confirm`} label={t("members.confirm_password")} required>
              <UiInput autoComplete="new-password" id={`${fieldID}-confirm`} minLength={8} onChange={(event) => setField("confirmPassword", event.target.value)} required type="password" value={draft.confirmPassword} />
            </UiField>
            {failed ? <UiInlineNotice message={t("join.failed_description")} title={t("join.failed_title")} tone="danger" /> : null}
            <UiButton disabled={Boolean(validation) || submitting} tone="primary" type="submit" variant="solid">
              {submitting ? t("join.submitting") : t("join.submit")}
            </UiButton>
          </form>
        ) : null}
      </UiPanel>
    </AccessPageFrame>
  );
}
