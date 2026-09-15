// INPUT: 当前远程登录与本机授权状态。
// OUTPUT: 已发布本机 Agent 的显式授权、执行开关与本机审批会话入口。
// POS: 在线群的宿主授权表面；旧授权不得静默升级为任务执行。
import { useEffect, useId, useRef, useState } from "react";
import { Link } from "react-router-dom";

import { authorizeTeamNode, getTeamNode, revokeTeamNode, type TeamNodeView } from "@/lib/api/conversation/team-node-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { UiButton } from "@/shared/ui/button/button";
import { UiDialogBackdrop, UiDialogBody, UiDialogFooter, UiDialogHeader, UiDialogPortal, UiDialogShell } from "@/shared/ui/dialog/dialog";
import { UiCheckboxRow } from "@/shared/ui/form/checkbox-row";
import { UiInput } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export function TeamNodeDialog({ onClose }: { onClose: () => void }) {
  const { t } = useI18n();
  const titleId = useId();
  const [view, setView] = useState<TeamNodeView | null>(null);
  const [name, setName] = useState("Nexus");
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(true);
  const [failed, setFailed] = useState(false);
  const generation = useRef(0);
  const inFlight = useRef(false);
  useEffect(() => {
    const current = ++generation.current;
    const controller = new AbortController();
    void getTeamNode(controller.signal)
      .then((result) => { if (generation.current === current) setView(result); })
      .catch(() => { if (generation.current === current) setFailed(true); })
      .finally(() => { if (generation.current === current) setBusy(false); });
    return () => { generation.current = current + 1; controller.abort(); };
  }, []);

  const locked = !!view && view.state !== "disconnected" && view.state !== "revoked";
  const agentIds = locked ? view.agent_ids : selected;
  const grantName = locked ? view.name ?? "Nexus" : name;
  const act = async (command?: () => Promise<unknown>) => {
    if (inFlight.current || busy) return;
    const current = generation.current;
    inFlight.current = true;
    setBusy(true);
    setFailed(false);
    try {
      await command?.();
    } catch {
      if (generation.current === current) setFailed(true);
    }
    // 未知响应也重读持久意图，不能让下一次点击生成另一份授权。
    try {
      const result = await getTeamNode();
      if (generation.current === current) setView(result);
    } catch {
      if (generation.current === current) { setFailed(true); setView(null); }
    } finally {
      inFlight.current = false;
      if (generation.current === current) setBusy(false);
    }
  };

  return (
    <UiDialogPortal>
      <UiDialogBackdrop labelledBy={titleId} onClose={onClose}>
        <UiDialogShell size="md" viewport="adaptiveMax">
          <UiDialogHeader appearance="plain" onClose={onClose} title={t("team.node_title")} titleId={titleId} />
          <UiDialogBody className="space-y-4 px-5" scrollable>
            <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t("team.node_scope")}</p>
            {view ? <p role="status" className={getUiTypographyClassName({ role: "body" })}>{t(`team.node_${view.state}`)}</p> : null}
            {!view?.execution_available ? <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t("team.node_runtime_pending")}</p> : null}
            {view?.execution_available ? <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t(view.execution_enabled ? "team.node_execution_enabled" : "team.node_execution_hint")}</p> : null}
            {failed ? <p role="alert" className={getUiTypographyClassName({ role: "supporting", tone: "danger" })}>{t("team.node_error")}</p> : null}
            <UiInput aria-label={t("team.node_name")} disabled={busy || locked || !view} maxLength={128} value={grantName} onChange={(event) => setName(event.target.value)} />
            <div className="space-y-2" aria-busy={busy}>
              {view?.candidates.map((agent) => <UiCheckboxRow key={agent.id} label={agent.name} checked={agentIds.includes(agent.id)}
                disabled={busy || locked || (!agentIds.includes(agent.id) && selected.length >= 32)}
                onChange={(checked) => setSelected((previous) => checked ? [...previous, agent.id] : previous.filter((id) => id !== agent.id))} />)}
              {view && view.candidates.length === 0 ? <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })}>{t("team.node_no_agents")}</p> : null}
            </div>
            {view?.jobs?.length ? <section className="space-y-2" aria-label={t("team.node_jobs")}>
              <h3 className={getUiTypographyClassName({ role: "sectionTitle" })}>{t("team.node_jobs")}</h3>
              {view.jobs.map((job) => <div key={job.id} className="flex flex-wrap items-center justify-between gap-2">
                <span className={getUiTypographyClassName({ role: "supporting" })}>{view.candidates.find((agent) => agent.id === job.agent_id)?.name ?? t("team.node_agent")} · {t(`team.node_job_${job.state}`)}</span>
                {job.room_id && job.conversation_id ? <Link className={getUiTypographyClassName({ role: "supporting", tone: "brand" })} to={AppRouteBuilders.roomConversation(job.room_id, job.conversation_id)} onClick={onClose}>{t("team.node_open_execution")}</Link> : null}
              </div>)}
            </section> : null}
          </UiDialogBody>
          <UiDialogFooter>
            <UiButton disabled={busy} onClick={() => { void act(); }} variant="surface">{t("team.node_refresh")}</UiButton>
            {view?.state === "authorized" && view.execution_available && !view.execution_enabled ? <UiButton disabled={busy} onClick={() => { void act(() => authorizeTeamNode(grantName, agentIds, true)); }} variant="solid">{t("team.node_enable_execution")}</UiButton> : null}
            {view?.state === "pending" ? <UiButton disabled={busy} onClick={() => { void act(revokeTeamNode); }} variant="surface">{t("team.node_revoke")}</UiButton> : null}
            {view?.state === "authorized" || view?.state === "revoking" ? (
              <UiButton disabled={busy} onClick={() => { void act(revokeTeamNode); }} variant="surface">{t("team.node_revoke")}</UiButton>
            ) : (
              <UiButton disabled={busy || !view || !grantName.trim() || agentIds.length === 0} onClick={() => { void act(() => authorizeTeamNode(grantName, agentIds)); }} variant="solid">
                {t(view?.state === "pending" ? "team.node_retry" : "team.node_authorize")}
              </UiButton>
            )}
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
