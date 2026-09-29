// INPUT: 当前已加入的在线群；仅使用 Relay 持久共享文件，不读取 Agent 私有目录。
// OUTPUT: 共用文件树、上传和下载；重复上传同名同内容沿用原命令。
// POS: 群共享工作区与本机 Agent 工作区的边界。
import { useEffect, useRef, useState } from "react";
import { Upload } from "lucide-react";
import { listTeamFiles, uploadTeamFile, saveTeamFile, type TeamFile } from "@/lib/api/conversation/team-files-api";
import { useTeamRefresh } from "./use-team-refresh";
import { WorkspaceFileTree } from "@/shared/ui/workspace/tree/workspace-file-tree";
import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";

export function TeamWorkspace({roomId}: {roomId: string}) {
  const {t} = useI18n();
  const [files, setFiles] = useState<TeamFile[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  const input = useRef<HTMLInputElement>(null);
  const transfer = useRef<AbortController | null>(null);
  useEffect(() => () => { transfer.current?.abort(); }, []);
  const refresh = useTeamRefresh(roomId, async (signal) => {
    try { const result = await listTeamFiles(roomId, signal); if (!signal.aborted) { setFiles(result); setLoaded(true); if (!pending) setError(null); } }
    catch { if (!signal.aborted) { setFiles([]); setError(t("team.files_error")); } }
  });
  const upload = async (file: File) => {
    if (lock.current) return;
    if (file.size > 32 * 1024 * 1024) { setError(t("team.files_limit")); return; }
    lock.current = true; setBusy(true); setPending(file); setError(null);
    const controller = new AbortController(); transfer.current = controller;
    try {
      await uploadTeamFile(roomId, file, controller.signal);
      if (!controller.signal.aborted) { setPending(null); refresh(); }
    } catch { if (!controller.signal.aborted) setError(t("team.files_upload_error")); }
    finally {lock.current = false; setBusy(false);}
  };
  const download = async (id: string) => {
    const file = files.find((entry) => entry.id === id); if (!file || lock.current) return;
    lock.current = true; setBusy(true);
    const controller = new AbortController(); transfer.current = controller;
    try {
      await saveTeamFile(roomId, file, controller.signal);
    } catch { if (!controller.signal.aborted) setError(t("team.files_error")); }
    finally {lock.current = false; setBusy(false);}
  };
  return <div className="flex min-h-0 flex-1 flex-col overflow-auto p-3">
    <div className="flex items-center justify-between gap-2 pb-3">
      <span className="text-xs text-muted">{t("team.files_shared")}</span>
      <UiButton size="sm" variant="surface" disabled={busy || !!pending} onClick={() => input.current?.click()}><Upload className="h-4 w-4" />{t("room.workspace_action_upload")}</UiButton>
      <input ref={input} type="file" className="hidden" aria-label={t("room.workspace_action_upload")} onChange={(event) => {const file = event.target.files?.[0]; event.target.value=""; if(file) void upload(file);}} />
    </div>
    {error ? <div role="alert" className="py-2 text-sm">{error}<UiButton size="sm" variant="text" disabled={busy} onClick={() => pending ? void upload(pending) : refresh()}>{t("state.retry")}</UiButton></div> : null}
    {pending && !busy ? <UiButton size="sm" variant="text" onClick={() => {setPending(null); setError(null); refresh();}}>{t("common.cancel")}</UiButton> : null}
    {!loaded && !error ? <p role="status">{t("common.loading")}</p> : null}
    {loaded && !files.length && !error ? <p className="p-3 text-sm text-muted">{t("room.no_files")}</p> : null}
    <WorkspaceFileTree activePath={null} focusedDirectoryPath={null} entries={files.map((file) => ({path:file.id,name:file.name,size:file.size,is_dir:false,modified_at:file.created_at,depth:0}))} onClickDirectory={() => {}} onClickFile={(id) => {void download(id);}} />
  </div>;
}
