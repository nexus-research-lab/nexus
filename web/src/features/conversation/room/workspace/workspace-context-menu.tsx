/**
 * =====================================================
 * @File   : workspace-context-menu.tsx
 * @Date   : 2026-04-15 17:42
 * @Author : leemysw
 * 2026-04-15 17:42   Create
 * =====================================================
 */

"use client";

import { createPortal } from "react-dom";
import { useEffect, useRef } from "react";
import { FilePlus, FolderPlus, Pencil, Trash2, Upload } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { DIALOG_POPOVER_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";
import { WorkspaceFileEntry } from "@/types/agent/agent";

interface WorkspaceContextMenuProps {
  position: { x: number; y: number } | null;
  entry: WorkspaceFileEntry | null;
  can_create_children: boolean;
  on_upload: () => void;
  on_create_file: () => void;
  on_create_folder: () => void;
  on_rename: () => void;
  on_delete: () => void;
  on_close: () => void;
}

export function WorkspaceContextMenu({
  position,
  entry,
  can_create_children,
  on_upload,
  on_create_file,
  on_create_folder,
  on_rename,
  on_delete,
  on_close,
}: WorkspaceContextMenuProps) {
  const { t } = useI18n();
  const menu_ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handle_click_outside = (event: MouseEvent) => {
      if (menu_ref.current && !menu_ref.current.contains(event.target as Node)) {
        on_close();
      }
    };

    document.addEventListener("mousedown", handle_click_outside);
    return () => {
      document.removeEventListener("mousedown", handle_click_outside);
    };
  }, [on_close]);

  useEffect(() => {
    const handle_key_down = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        on_close();
      }
    };

    document.addEventListener("keydown", handle_key_down);
    return () => {
      document.removeEventListener("keydown", handle_key_down);
    };
  }, [on_close]);

  if (!position) {
    return null;
  }

  return createPortal(
    <div
      ref={menu_ref}
      className={DIALOG_POPOVER_CLASS_NAME}
      style={{
        top: `${position.y}px`,
        left: `${position.x}px`,
        minWidth: "180px",
      }}
    >
      <div className="py-1">
        {can_create_children ? (
          <>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { on_upload(); on_close(); }}
            >
              <Upload className="h-4 w-4" />
              <span>{t("room.workspace_action_upload")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { on_create_file(); on_close(); }}
            >
              <FilePlus className="h-4 w-4" />
              <span>{t("room.workspace_action_new_file")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { on_create_folder(); on_close(); }}
            >
              <FolderPlus className="h-4 w-4" />
              <span>{t("room.workspace_action_new_folder")}</span>
            </button>
            {entry ? <div className="my-1 h-px bg-(--divider-subtle-color)" /> : null}
          </>
        ) : null}

        {entry ? (
          <>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { on_rename(); on_close(); }}
            >
              <Pencil className="h-4 w-4" />
              <span>{t("home.rename")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-destructive"
              onClick={() => { on_delete(); on_close(); }}
            >
              <Trash2 className="h-4 w-4" />
              <span>{t("common.delete")}</span>
            </button>
          </>
        ) : null}
      </div>
    </div>,
    document.body,
  );
}
