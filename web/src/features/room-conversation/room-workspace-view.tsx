"use client";

import { FileText, FolderTree } from "lucide-react";
import { useMemo } from "react";

import { WorkspaceSurfaceView } from "@/shared/ui/workspace/workspace-surface-view";
import { useWorkspaceFilesStore } from "@/store/workspace-files";

interface RoomWorkspaceViewProps {
  active_workspace_path: string | null;
  agent_id: string;
  on_open_workspace_file: (path: string | null) => void;
}

export function RoomWorkspaceView({
  active_workspace_path,
  agent_id,
  on_open_workspace_file,
}: RoomWorkspaceViewProps) {
  const files_by_agent = useWorkspaceFilesStore((state) => state.files_by_agent);
  const visible_files = useMemo(
    () => (files_by_agent[agent_id] ?? []).filter((file) => !file.is_dir).slice(0, 12),
    [agent_id, files_by_agent],
  );

  return (
    <WorkspaceSurfaceView eyebrow="Workspace" title="共享资源">
      {visible_files.length ? (
        <div className="space-y-2">
          {visible_files.map((file) => {
            const is_active = file.path === active_workspace_path;
            return (
              <button
                key={file.path}
                className={`flex items-center gap-3 rounded-[14px] border px-4 py-2.5 text-left transition-all duration-300 ${is_active
                    ? "border-white/30 bg-white/20 shadow-[0_10px_18px_rgba(111,126,162,0.08)]"
                    : "border-white/14 bg-white/8 hover:bg-white/12"
                  }`}
                onClick={() => on_open_workspace_file(file.path)}
                type="button"
              >
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-white/24 bg-white/18 text-slate-700/60">
                  <FileText className="h-4 w-4" />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[13px] font-semibold text-slate-950/84">{file.name}</p>
                  <p className="truncate text-[11px] text-slate-700/52">{file.path}</p>
                </div>
              </button>
            );
          })}
        </div>
      ) : (
        <div className="rounded-2xl border border-white/20 bg-white/10 px-5 py-5 text-sm leading-7 text-slate-700/60">
          <div className="mb-3 flex items-center gap-2 text-slate-900/74">
            <FolderTree className="h-4 w-4" />
            当前还没有共享资源。
          </div>
          在左侧 Context 中创建或打开文件后，这里会成为当前协作的资源视图。
        </div>
      )}
    </WorkspaceSurfaceView>
  );
}
