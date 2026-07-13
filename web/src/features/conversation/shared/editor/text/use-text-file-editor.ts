import {
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import {
  getWorkspaceFileContentApi,
  updateWorkspaceFileContentApi,
} from "@/lib/api/agent/agent-api";
import { useWorkspaceLiveStore } from "@/store/workspace-live";

interface UseTextFileEditorParams {
  agentId: string;
  path: string;
}

/**
 * 文本控制器只维护 API 草稿与实时文件投影。
 * 请求编号隔离切换文件和实时更新触发的过期响应。
 */
export function useTextFileEditor({
  agentId,
  path,
}: UseTextFileEditorParams) {
  const [draftContent, setDraftContent] = useState("");
  const [savedContent, setSavedContent] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isEditing, setIsEditing] = useResettableState(false, path);
  const [error, setError] = useState<string | null>(null);
  const loadRequestIdRef = useRef(0);
  const liveState = useWorkspaceLiveStore(
    (state) => state.file_states[`${agentId}:${path}`],
  );
  const isExternalWriting = Boolean(
    liveState && liveState.source !== "api" && liveState.status === "writing",
  );
  const isDirty = draftContent !== savedContent;

  const loadContent = useCallback(async (): Promise<void> => {
    const requestId = loadRequestIdRef.current + 1;
    loadRequestIdRef.current = requestId;
    setIsLoading(true);
    setError(null);
    try {
      const response = await getWorkspaceFileContentApi(agentId, path);
      if (loadRequestIdRef.current === requestId) {
        setDraftContent(response.content);
        setSavedContent(response.content);
      }
    } catch (loadError) {
      if (loadRequestIdRef.current === requestId) {
        setError(
          loadError instanceof Error ? loadError.message : "读取文件失败",
        );
      }
    } finally {
      if (loadRequestIdRef.current === requestId) {
        setIsLoading(false);
      }
    }
  }, [agentId, path]);

  useEffect(() => {
    void loadContent();
    return () => {
      loadRequestIdRef.current += 1;
    };
  }, [loadContent]);

  useEffect(() => {
    if (!liveState || typeof liveState.live_content !== "string") {
      return;
    }
    if (liveState.source === "api" && isSaving) {
      return;
    }
    setDraftContent(liveState.live_content);
    if (liveState.status === "updated") {
      setSavedContent(liveState.live_content);
    }
  }, [isSaving, liveState]);

  useEffect(() => {
    if (
      liveState?.status === "updated" &&
      typeof liveState.live_content !== "string"
    ) {
      void loadContent();
    }
  }, [liveState, loadContent]);

  const save = useCallback(async (): Promise<void> => {
    if (!isDirty || isSaving) {
      return;
    }
    setIsSaving(true);
    setError(null);
    try {
      const response = await updateWorkspaceFileContentApi(
        agentId,
        path,
        draftContent,
      );
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (saveError) {
      setError(
        saveError instanceof Error ? saveError.message : "保存文件失败",
      );
    } finally {
      setIsSaving(false);
    }
  }, [agentId, draftContent, isDirty, isSaving, path]);

  const toggleEditing = useCallback((): void => {
    if (isEditing) {
      setIsEditing(false);
      return;
    }
    if (!isExternalWriting) {
      setIsEditing(true);
    }
  }, [isEditing, isExternalWriting, setIsEditing]);

  return {
    draftContent,
    error,
    isDirty,
    isEditing,
    isExternalWriting,
    isLoading,
    isSaving,
    liveState,
    save,
    setDraftContent,
    setIsEditing,
    toggleEditing,
  };
}
