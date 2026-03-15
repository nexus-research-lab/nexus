"use client";

import { useCallback, useEffect, useState } from "react";
import { GripVertical, LoaderCircle, Minimize2, Save } from "lucide-react";

import { getWorkspaceFileContentApi, updateWorkspaceFileContentApi } from "@/lib/agent-manage-api";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import { cn } from "@/lib/utils";

interface WorkspaceEditorPaneProps {
  agentId: string;
  path: string | null;
  isOpen: boolean;
  widthPercent: number;
  embedded?: boolean;
  onClose: () => void;
  onResizeStart: () => void;
}

export function WorkspaceEditorPane({
  agentId,
  path,
  isOpen,
  widthPercent,
  embedded = false,
  onClose,
  onResizeStart,
}: WorkspaceEditorPaneProps) {
  const [draftContent, setDraftContent] = useState("");
  const [savedContent, setSavedContent] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileStates = useWorkspaceLiveStore((state) => state.fileStates);

  const liveState = path ? fileStates[`${agentId}:${path}`] : undefined;
  const isExternalWriting = !!liveState && liveState.source !== "api" && liveState.status === "writing";
  const hasLiveContent = typeof liveState?.liveContent === "string";

  const isDirty = draftContent !== savedContent;

  const loadContent = useCallback(async () => {
    if (!isOpen || !path) {
      return;
    }

    setIsLoading(true);
    setError(null);
    try {
      const response = await getWorkspaceFileContentApi(agentId, path);
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "读取文件失败");
    } finally {
      setIsLoading(false);
    }
  }, [agentId, isOpen, path]);

  useEffect(() => {
    if (!isOpen || !path) {
      return;
    }

    let cancelled = false;
    void (async () => {
      setIsLoading(true);
      setError(null);
      try {
        const response = await getWorkspaceFileContentApi(agentId, path);
        if (cancelled) {
          return;
        }
        setDraftContent(response.content);
        setSavedContent(response.content);
      } catch (loadError) {
        if (!cancelled) {
          setError(loadError instanceof Error ? loadError.message : "读取文件失败");
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [agentId, isOpen, path]);

  useEffect(() => {
    if (!isOpen || !path || !liveState || !hasLiveContent) {
      return;
    }

    if (liveState.source === "api" && isSaving) {
      return;
    }

    setDraftContent(liveState.liveContent || "");
    if (liveState.status === "updated") {
      setSavedContent(liveState.liveContent || "");
    }
  }, [hasLiveContent, isOpen, isSaving, liveState, path]);

  useEffect(() => {
    if (!isOpen || !path || !liveState) {
      return;
    }

    if (liveState.status !== "updated" || typeof liveState.liveContent === "string") {
      return;
    }

    void loadContent();
  }, [isOpen, liveState, loadContent, path]);

  const handleSave = async () => {
    if (!path || !isDirty || isSaving) {
      return;
    }

    setIsSaving(true);
    setError(null);
    try {
      const response = await updateWorkspaceFileContentApi(agentId, path, draftContent);
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "保存文件失败");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <section
      className={cn(
        "relative flex min-h-0 shrink-0 flex-col overflow-hidden bg-secondary/78 transition-[width,opacity,transform,border-color] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] will-change-[width,opacity,transform]",
        embedded
          ? "border-l border-border/70 shadow-none"
          : "rounded-[20px] border border-border/70 shadow-[0_18px_44px_rgba(17,24,39,0.08)]",
        isOpen ? "opacity-100 translate-x-0" : "pointer-events-none opacity-0 -translate-x-3",
        embedded && !isOpen && "border-l-transparent",
      )}
      style={
        embedded
          ? { width: isOpen ? "calc(100% - 280px)" : "0px" }
          : { width: isOpen ? `${widthPercent}%` : "0px" }
      }
    >
      {embedded && (!isOpen || !path) ? (
        <div className="flex h-full flex-1 items-center justify-center px-8 text-center">
          <div className="max-w-sm">
            <p className="text-sm font-semibold uppercase tracking-[0.18em] text-muted-foreground">
              Workspace Preview
            </p>
            <p className="mt-3 text-sm leading-6 text-muted-foreground">
              从左侧选择一个文件，这里会显示对应内容。模型写入时，也会在这里实时同步。
            </p>
          </div>
        </div>
      ) : isOpen && path ? (
        <>
          {!embedded && (
            <button
              aria-label="调整编辑器宽度"
              className="absolute -right-3 top-0 z-20 flex h-full w-6 cursor-col-resize items-center justify-center text-muted-foreground/60 transition-colors hover:text-primary"
              onMouseDown={onResizeStart}
              type="button"
            >
              <GripVertical className="h-4 w-4" />
            </button>
          )}

          <div className="flex h-12 items-center justify-between border-b border-border/80 px-4">
            <div className="min-w-0 pr-3">
              <p
                className="truncate text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground"
                title={path}
              >
                {path}
              </p>
              {liveState && liveState.source !== "api" && (
                <div className="mt-1 flex items-center gap-2 text-[10px] text-muted-foreground">
                  {isExternalWriting ? (
                    <>
                      <LoaderCircle className="h-3 w-3 animate-spin text-primary" />
                      <span>模型正在实时写入该文件</span>
                    </>
                  ) : (
                    <>
                      <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                      <span>
                        已同步最新内容
                        {liveState.diffStats
                          ? ` · +${liveState.diffStats.additions} -${liveState.diffStats.deletions}`
                          : ""}
                      </span>
                    </>
                  )}
                </div>
              )}
            </div>

            <div className="flex items-center gap-2">
              <button
                className={cn(
                  "inline-flex h-9 items-center gap-2 rounded-xl px-3 text-sm font-medium transition-colors",
                  isDirty ? "bg-primary text-primary-foreground" : "bg-muted/80 text-muted-foreground",
                )}
                disabled={!isDirty || isSaving || isExternalWriting}
                onClick={() => void handleSave()}
                type="button"
              >
                <Save className="h-4 w-4" />
                {isSaving ? "保存中" : "保存"}
              </button>
              <button
                className="flex h-9 w-9 items-center justify-center rounded-xl border border-border/80 bg-secondary text-muted-foreground transition-colors hover:border-primary/20 hover:text-primary"
                onClick={onClose}
                type="button"
              >
                <Minimize2 className="h-4 w-4" />
              </button>
            </div>
          </div>

          {error && (
            <div className="px-4 py-3 text-sm text-destructive">{error}</div>
          )}

          <div className="flex-1 p-3">
            <textarea
              className="soft-scrollbar h-full w-full resize-none rounded-2xl border border-border/80 bg-background p-4 font-mono text-sm leading-6 text-foreground outline-none focus:border-primary/20 disabled:opacity-70"
              disabled={isLoading || isExternalWriting}
              onChange={(event) => setDraftContent(event.target.value)}
              value={isLoading ? "加载中..." : draftContent}
            />
          </div>
        </>
      ) : null}
    </section>
  );
}
