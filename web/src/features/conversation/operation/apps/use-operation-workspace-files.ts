/**
 * INPUT: Agent identity and the operation revision currently shown by the Files app.
 * OUTPUT: Cached workspace entries plus truthful loading, ready, or unavailable state.
 * POS: Files resource controller; API loading stays outside the visual surface.
 */
import { useCallback, useEffect, useState } from "react";

import { useWorkspaceFilesStore } from "@/store/workspace-files";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

const EMPTY_FILES: WorkspaceFileEntry[] = [];

export type OperationWorkspaceFilesStatus = "loading" | "ready" | "unavailable";

export function useOperationWorkspaceFiles(agentId: string, revision: string) {
  const files = useWorkspaceFilesStore((state) => (
    state.files_by_agent[agentId] ?? EMPTY_FILES
  ));
  const refresh_files = useWorkspaceFilesStore((state) => state.refresh_files);
  const [status, set_status] = useState<OperationWorkspaceFilesStatus>(
    files.length ? "ready" : "loading",
  );

  const reload = useCallback(async () => {
    set_status("loading");
    try {
      await refresh_files(agentId);
      set_status("ready");
    } catch {
      set_status("unavailable");
    }
  }, [agentId, refresh_files]);

  useEffect(() => {
    void reload();
  }, [reload, revision]);

  return { files, reload, status };
}
