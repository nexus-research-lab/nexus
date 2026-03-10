"use client";

import { useEffect, useState } from "react";
import { FileCode2, Minimize2, Save } from "lucide-react";

import { getWorkspaceFileContentApi, updateWorkspaceFileContentApi } from "@/lib/agent-manage-api";
import { cn } from "@/lib/utils";

interface WorkspaceEditorPaneProps {
  agentId: string;
  path: string | null;
  isOpen: boolean;
  onClose: () => void;
}

export function WorkspaceEditorPane({
  agentId,
  path,
  isOpen,
  onClose,
}: WorkspaceEditorPaneProps) {
  const [draftContent, setDraftContent] = useState("");
  const [savedContent, setSavedContent] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isDirty = draftContent !== savedContent;

  useEffect(() => {
    if (!isOpen || !path) {
      return;
    }

    let cancelled = false;
    const loadContent = async () => {
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
    };

    void loadContent();
    return () => {
      cancelled = true;
    };
  }, [agentId, isOpen, path]);

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
        "flex min-h-0 flex-col overflow-hidden border-r border-border/80 bg-secondary/78 shadow-[12px_0_32px_rgba(17,24,39,0.08)] transition-[width,opacity,transform] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
        isOpen ? "w-1/2 opacity-100 translate-x-0" : "w-0 opacity-0 -translate-x-6",
      )}
    >
      {isOpen && path && (
        <>
          <div className="flex items-center justify-between border-b border-border/80 px-4 py-3">
            <div className="min-w-0">
              <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground">
                <FileCode2 className="h-3.5 w-3.5" />
                <span>{path}</span>
              </div>
            </div>

            <div className="flex items-center gap-2">
              <button
                className={cn(
                  "inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium transition-colors",
                  isDirty ? "bg-primary text-primary-foreground" : "bg-muted/80 text-muted-foreground",
                )}
                disabled={!isDirty || isSaving}
                onClick={() => void handleSave()}
                type="button"
              >
                <Save className="h-4 w-4" />
                {isSaving ? "保存中" : "保存"}
              </button>
              <button
                className="rounded-xl border border-border/80 bg-secondary p-2 text-muted-foreground transition-colors hover:border-primary/20 hover:text-primary"
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
              disabled={isLoading}
              onChange={(event) => setDraftContent(event.target.value)}
              value={isLoading ? "加载中..." : draftContent}
            />
          </div>
        </>
      )}
    </section>
  );
}
