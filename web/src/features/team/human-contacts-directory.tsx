// INPUT: 当前组织身份及 Control 真人目录。
// OUTPUT: 可搜索的真人联系人，打开 Relay 唯一双人会话。
// POS: 真人私聊入口；不创建本地 Agent DM 或发送占位消息。
import { type ReactNode, useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { MessageCircle } from "lucide-react";
import { listControlMemberDirectoryApi, type ControlMemberDirectoryEntry } from "@/lib/api/account/control-api";
import { createTeamRoom } from "@/lib/api/conversation/team-api";
import { hasOrganizationAccess, useAuth } from "@/shared/auth/auth-context";
import { captureAuthOwnerScopeGeneration, isAuthOwnerScopeGenerationCurrent } from "@/shared/auth/auth-owner-generation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { SidebarSearchField } from "@/shared/ui/form/sidebar-search-field";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiListRow } from "@/shared/ui/list/list-row";

export function HumanContactsDirectory({sidebar = false, afterSearch}: {sidebar?: boolean; afterSearch?: ReactNode} = {}) {
  const { status } = useAuth();
  if (!hasOrganizationAccess(status)) return null;
  return <HumanContactsContent key={`${status?.organization_id}:${status?.control_user_id}`} currentUserId={status?.control_user_id ?? ""} sidebar={sidebar} afterSearch={afterSearch} />;
}

function HumanContactsContent({currentUserId, sidebar, afterSearch}: {currentUserId: string; sidebar: boolean; afterSearch?: ReactNode}) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const selectedId = params.get("member");
  const [people, setPeople] = useState<ControlMemberDirectoryEntry[]>([]);
  const [query, setQuery] = useState("");
  const [failed, setFailed] = useState(false);
  const [loading, setLoading] = useState(true);
  const [reload, setReload] = useState(0);
  const [opening, setOpening] = useState<string | null>(null);
  const [openFailed, setOpenFailed] = useState(false);
  const busy = useRef(false);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setFailed(false);
    void listControlMemberDirectoryApi().then((items) => { if (!cancelled) setPeople(items.filter((person) => person.user_id !== currentUserId)); })
      .catch(() => { if (!cancelled) setFailed(true); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [currentUserId, reload]);
  const open = async (userId: string) => {
    if (busy.current) return;
    busy.current = true;
    const generation = captureAuthOwnerScopeGeneration();
    setOpening(userId);
    setOpenFailed(false);
    try {
      const result = await createTeamRoom({direct_user_id: userId, name: "", member_user_ids: [], agent_ids: [], private_messages_enabled: false, skill_names: []}, crypto.randomUUID());
      if (mounted.current && isAuthOwnerScopeGenerationCurrent(generation)) { navigate(`/team?room_id=${encodeURIComponent(result.room.id)}`); }
    } catch { if (mounted.current && isAuthOwnerScopeGenerationCurrent(generation)) setOpenFailed(true); }
    finally { busy.current = false; if (mounted.current) setOpening(null); }
  };
  const visible = people.filter((person) => `${person.display_name} ${person.username}`.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));
  const selected = people.find((person) => person.user_id === selectedId);
  const feedback = <>
    {failed ? <div role="alert">{t("team.directory_failed")} <UiButton size="sm" variant="text" onClick={() => setReload((value) => value + 1)}>{t("state.retry")}</UiButton></div> : null}
    {openFailed ? <p role="alert">{t("team.direct_failed")}</p> : null}
    {loading ? <p role="status">{t("team.members_loading")}</p> : null}
  </>;
  if (sidebar) return <>
    <SidebarSearchField label={t("team.search_people")} value={query} onChange={setQuery} />
    {afterSearch}
    <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-2 max-[559px]:px-3">
      {feedback}
      {!loading && !failed && !visible.length ? <p>{t("team.no_people")}</p> : null}
      {visible.map((person) => <UiListRow key={person.user_id} aria-label={`${person.display_name || person.username} @${person.username}`} density="sidebarCompact" activeTone="sidebar" inactiveTone="muted"
        active={selectedId === person.user_id} title={person.display_name || person.username} description={`@${person.username}`}
        leading={<UiAgentAvatar avatar={person.avatar} name={person.display_name || person.username} size="md" />}
        right={<UiListActionButton title={t("sidebar.start_chat")} size="md" type="button" visibility="hover"
          disabled={opening !== null || failed} aria-busy={opening === person.user_id}
          onClick={(event) => { event.stopPropagation(); void open(person.user_id); }}>
          <MessageCircle aria-hidden="true" className="h-[18px] w-[18px]" />
        </UiListActionButton>}
        onClick={() => navigate(`/contacts?view=members&member=${encodeURIComponent(person.user_id)}`)} />)}
    </div>
  </>;
  return <div className="flex min-h-0 min-w-0 flex-1 flex-col">
    <div className="soft-scrollbar scrollbar-stable-gutter min-h-0 flex-1 overflow-y-auto">
    <section aria-label={t("team.human_contacts")} className={WORKSPACE_CONTENT_PAGE_CLASS_NAME}>
    <WorkspaceContentHeader title={t("team.human_contacts")} actions={selectedId
      ? <UiButton variant="ghost" onClick={() => navigate("/contacts?view=members")}>{t("team.back_to_members")}</UiButton>
      : <UiSearchInput className="w-full sm:w-[240px]" aria-label={t("team.search_people")} placeholder={t("team.search_people")} value={query} onChange={setQuery} />} />
    {feedback}
    {selected && !failed ? <div className="flex flex-wrap items-center gap-4 py-6">
      <UiAgentAvatar avatar={selected.avatar} name={selected.display_name || selected.username} size="xl" />
      <div className="min-w-0 flex-1"><h2 className="dialog-label">{selected.display_name || selected.username}</h2><p className="text-(--text-muted)">@{selected.username}</p></div>
      <UiButton aria-label={`${t("team.start_direct")}: ${selected.display_name || selected.username}`} aria-busy={opening !== null} disabled={opening !== null} variant="ghost" onClick={() => { void open(selected.user_id); }}>
        <MessageCircle aria-hidden="true" className="h-4 w-4" />{t("team.start_direct")}
      </UiButton>
    </div> : selectedId ? (!loading && !failed ? <UiResourceState state="empty" variant="plain" title={t("team.member_unavailable")} /> : null) : <>
      <div className="mb-3 flex min-h-8 items-center">
        <span className={getUiTypographyClassName({role: "metadata", tone: "muted"})}>{t("team.member_result_count", {count: visible.length, total: people.length})}</span>
      </div>
      {!loading && !failed ? <UiPanel padding="none" radius="md" variant="card" className="divide-y divide-(--divider-subtle-color) overflow-hidden">
        {visible.map((person) => <UiListRow key={person.user_id} variant="flush" title={person.display_name || person.username} description={`@${person.username}`}
          leading={<UiAgentAvatar avatar={person.avatar} name={person.display_name || person.username} size="md" />}
          onClick={() => navigate(`/contacts?view=members&member=${encodeURIComponent(person.user_id)}`)}
          right={<UiButton variant="ghost" size="sm" disabled={opening !== null} aria-busy={opening === person.user_id}
            aria-label={`${t("team.start_direct")}: ${person.display_name || person.username}`}
            onClick={(event) => { event.stopPropagation(); void open(person.user_id); }}>
            <MessageCircle aria-hidden="true" className="h-4 w-4" />{t("team.start_direct")}
          </UiButton>} />)}
        {!visible.length ? <UiResourceState size="sm" state="empty" variant="plain" title={t("team.no_people")} /> : null}
      </UiPanel> : null}
    </>}
  </section></div></div>;
}
