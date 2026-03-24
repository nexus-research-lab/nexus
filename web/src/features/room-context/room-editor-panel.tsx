"use client";

import { useCallback, useEffect, useState } from "react";
import { GripVertical, LoaderCircle, Minimize2, Save } from "lucide-react";

import { getWorkspaceFileContentApi, updateWorkspaceFileContentApi } from "@/lib/agent-manage-api";
import { cn } from "@/lib/utils";
import { useWorkspaceLiveStore } from "@/store/workspace-live";

export interface RoomEditorPanelProps {
  agent_id: string;
  path: string | null;
  is_open: boolean;
  width_percent: number;
  embedded?: boolean;
  class_name?: string;
  on_close: () => void;
  on_resize_start: () => void;
}

export function RoomEditorPanel({
  agent_id,
  path,
  is_open,
  width_percent,
  embedded = false,
  class_name,
  on_close,
  on_resize_start,
}: RoomEditorPanelProps) {
  const [draft_content, setDraftContent] = useState("");
  const [saved_content, setSavedContent] = useState("");
  const [is_loading, setIsLoading] = useState(false);
  const [is_saving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const file_states = useWorkspaceLiveStore((state) => state.fileStates);

  const live_state = path ? file_states[`${agent_id}:${path}`] : undefined;
  const is_external_writing = !!live_state && live_state.source !== "api" && live_state.status === "writing";
  const has_live_content = typeof live_state?.live_content === "string";
  const is_dirty = draft_content !== saved_content;

  const load_content = useCallback(async () => {
    if (!is_open || !path) {
      return;
    }

    setIsLoading(true);
    setError(null);
    try {
      const response = await getWorkspaceFileContentApi(agent_id, path);
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (load_error) {
      setError(load_error instanceof Error ? load_error.message : "读取文件失败");
    } finally {
      setIsLoading(false);
    }
  }, [agent_id, is_open, path]);

  useEffect(() => {
    if (!is_open || !path) {
      return;
    }

    let cancelled = false;
    void (async () => {
      setIsLoading(true);
      setError(null);
      try {
        const response = await getWorkspaceFileContentApi(agent_id, path);
        if (cancelled) {
          return;
        }
        setDraftContent(response.content);
        setSavedContent(response.content);
      } catch (load_error) {
        if (!cancelled) {
          setError(load_error instanceof Error ? load_error.message : "读取文件失败");
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
  }, [agent_id, is_open, path]);

  useEffect(() => {
    if (!is_open || !path || !live_state || !has_live_content) {
      return;
    }

    if (live_state.source === "api" && is_saving) {
      return;
    }

    setDraftContent(live_state.live_content || "");
    if (live_state.status === "updated") {
      setSavedContent(live_state.live_content || "");
    }
  }, [has_live_content, is_open, is_saving, live_state, path]);

  useEffect(() => {
    if (!is_open || !path || !live_state) {
      return;
    }

    if (live_state.status !== "updated" || typeof live_state.live_content === "string") {
      return;
    }

    void load_content();
  }, [is_open, live_state, load_content, path]);

  const handle_save = async () => {
    if (!path || !is_dirty || is_saving) {
      return;
    }

    setIsSaving(true);
    setError(null);
    try {
      const response = await updateWorkspaceFileContentApi(agent_id, path, draft_content);
      setDraftContent(response.content);
      setSavedContent(response.content);
    } catch (save_error) {
      setError(save_error instanceof Error ? save_error.message : "保存文件失败");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <section
      className={cn(
        "relative flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden transition-[width,opacity,transform,border-color] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] will-change-[width,opacity,transform]",
        embedded ? "border-l workspace-divider shadow-none" : "workspace-card radius-shell-lg",
        is_open ? "translate-x-0 opacity-100" : "pointer-events-none -translate-x-3 opacity-0",
        embedded && !is_open && "border-l-transparent",
        class_name,
      )}
      style={
        embedded
          ? { width: is_open ? "calc(100% - 280px)" : "0px" }
          : { width: is_open ? `${width_percent}%` : "0px" }
      }
    >
      {embedded && (!is_open || !path) ? (
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
      ) : is_open && path ? (
        <>
          {!embedded ? (
            <button
              aria-label="调整编辑器宽度"
              className="absolute -right-3 top-0 z-20 flex h-full w-6 cursor-col-resize items-center justify-center text-muted-foreground/60 transition-colors hover:text-primary"
              onMouseDown={on_resize_start}
              type="button"
            >
              <GripVertical className="h-4 w-4" />
            </button>
          ) : null}

          <div className="flex h-14 min-w-0 items-center justify-between overflow-hidden border-b workspace-divider px-4">
            <div className="min-w-0 flex-1 overflow-hidden pr-3">
              <p
                className="w-full truncate text-xs font-semibold uppercase tracking-[0.16em] text-muted-foreground"
                title={path}
              >
                {path.split("/").at(-1)}
              </p>
              {live_state && live_state.source !== "api" ? (
                <div className="mt-1 flex min-w-0 items-center gap-2 text-[10px] text-muted-foreground">
                  {is_external_writing ? (
                    <>
                      <LoaderCircle className="h-3 w-3 shrink-0 animate-spin text-primary" />
                      <span className="truncate">模型正在实时写入该文件</span>
                    </>
                  ) : (
                    <>
                      <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500" />
                      <span className="truncate">
                        已同步最新内容
                        {live_state.diff_stats
                          ? ` · +${live_state.diff_stats.additions} -${live_state.diff_stats.deletions}`
                          : ""}
                      </span>
                    </>
                  )}
                </div>
              ) : null}
            </div>

            <div className="flex shrink-0 items-center gap-2">
              <button
                className={cn(
                  "inline-flex h-10 items-center gap-2 rounded-full px-4 text-sm font-semibold transition-all duration-300",
                  is_dirty
                    ? "bg-[linear-gradient(135deg,rgba(166,255,194,0.92),rgba(102,217,143,0.88))] text-[#18653a] shadow-[0_14px_24px_rgba(102,217,143,0.22)]"
                    : "workspace-chip text-slate-700/58",
                )}
                disabled={!is_dirty || is_saving || is_external_writing}
                onClick={() => void handle_save()}
                type="button"
              >
                <Save className="h-4 w-4" />
                {is_saving ? "保存中" : "保存"}
              </button>
              <button
                className="workspace-chip flex h-10 w-10 items-center justify-center rounded-2xl text-slate-700/58 transition-colors hover:text-slate-950"
                onClick={on_close}
                type="button"
              >
                <Minimize2 className="h-4 w-4" />
              </button>
            </div>
          </div>

          {error ? (
            <div className="px-4 py-3 text-sm text-destructive">{error}</div>
          ) : null}

          <div className="flex-1 p-3">
            <textarea
              className="soft-scrollbar workspace-card h-full w-full resize-none rounded-[28px] p-5 font-mono text-sm leading-6 text-slate-900/82 outline-none disabled:opacity-70"
              disabled={is_loading || is_external_writing}
              onChange={(event) => setDraftContent(event.target.value)}
              value={is_loading ? "加载中..." : draft_content}
            />
          </div>
        </>
      ) : null}
    </section>
  );
}
