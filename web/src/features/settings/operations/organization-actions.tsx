// INPUT: 远程账号的组织身份和 Control 组织事务。
// OUTPUT: 无组织创建入口及改名、退出、移交、解散的显式确认。
// POS: 组织生命周期动作；不授予平台运营权限。
import { useRef, useState } from "react";
import { listControlMembersApi, mutateControlOrganizationApi, type ControlDeploymentMember } from "@/lib/api/account/control-api";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiDialogBackdrop, UiDialogBody, UiDialogFooter, UiDialogHeader, UiDialogPortal, UiDialogShell } from "@/shared/ui/dialog/dialog";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";

type Action = "create" | "rename" | "leave" | "transfer" | "dissolve";

export function OrganizationActions() {
  const { status, refreshStatus } = useAuth();
  const { t } = useI18n();
  const [action, setAction] = useState<Action | null>(null);
  const [name, setName] = useState("");
  const [target, setTarget] = useState("");
  const [members, setMembers] = useState<ControlDeploymentMember[]>([]);
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  const locked = useRef(false);
  const actionsByRole: Record<string, Action[]> = {
    owner: ["rename", "transfer", "dissolve"],
    admin: ["rename", "leave"],
    member: ["leave"],
  };
  const actions = !status?.organization_id ? ["create" as const] : actionsByRole[status.organization_role ?? ""] ?? [];

  const open = async (next: Action) => {
    setAction(next);
    setName(status?.organization_name ?? "");
    setTarget("");
    setMembers([]);
    setFailed(false);
    if (next !== "transfer") return;
    locked.current = true;
    setPending(true);
    try {
      const result = await listControlMembersApi();
      setMembers(result.filter((member) => member.membership_status === "active" && member.user_id !== (status?.control_user_id ?? status?.user_id)));
    } catch { setFailed(true); }
    finally { locked.current = false; setPending(false); }
  };
  const submit = async () => {
    if (!action || locked.current) return;
    locked.current = true;
    setPending(true);
    setFailed(false);
    try {
      const input = action === "transfer" ? { target_user_id: target } : action === "create" || action === "rename" ? { name: name.trim() } : {};
      await mutateControlOrganizationApi(action, input);
      await refreshStatus();
      setAction(null);
    } catch {
      setFailed(true);
      // 响应丢失时先对账，不自动重放退出、移交或解散。
      await refreshStatus().catch(() => undefined);
    } finally { locked.current = false; setPending(false); }
  };
  return <>
    {status?.organization_id && actions.length === 0 ? <UiInlineNotice tone="danger" message={t("organization.role_unavailable")} /> : null}
    <div className="flex flex-wrap gap-2">
      {actions.map((item) => <UiButton key={item} disabled={pending} variant="outline" size="sm" onClick={() => void open(item)}>{t(`organization.action_${item}`)}</UiButton>)}
    </div>
    {action ? <UiDialogPortal><UiDialogBackdrop onClose={() => { if (!pending) setAction(null); }}>
      <UiDialogShell size="md" viewport="adaptiveMax">
        <UiDialogHeader title={t(`organization.action_${action}`)} onClose={() => { if (!pending) setAction(null); }} />
        <UiDialogBody className="space-y-4 px-5" scrollable>
          <p>{t(`organization.hint_${action}`)}</p>
          {action === "create" || action === "rename" ? <UiField label={t("organization.name")}><UiInput aria-label={t("organization.name")} maxLength={128} disabled={pending} value={name} onChange={(event) => setName(event.target.value)} /></UiField> : null}
          {action === "transfer" ? <UiSelectMenu ariaLabel={t("organization.action_transfer")} disabled={pending} value={target} onChange={setTarget} options={members.map((member) => ({ value: member.user_id, label: member.display_name || member.username }))} /> : null}
          {failed ? <UiInlineNotice tone="danger" message={t("organization.action_failed")} /> : null}
        </UiDialogBody>
        <UiDialogFooter>
          <UiButton disabled={pending} variant="surface" onClick={() => setAction(null)}>{t("common.cancel")}</UiButton>
          <UiButton disabled={pending || ((action === "create" || action === "rename") && !name.trim()) || (action === "transfer" && !target)} tone="primary" variant="solid" onClick={() => void submit()}>{t(`organization.action_${action}`)}</UiButton>
        </UiDialogFooter>
      </UiDialogShell>
    </UiDialogBackdrop></UiDialogPortal> : null}
  </>;
}
