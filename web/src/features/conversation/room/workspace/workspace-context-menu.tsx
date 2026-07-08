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
import { Download, FilePlus, FolderOpen, FolderPlus, Pencil, Trash2, Upload } from "lucide-react";

import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { DIALOG_POPOVER_CLASS_NAME } from "@/shared/ui/dialog/dialog-styles";
import { WorkspaceFileEntry } from "@/types/agent/agent";

interface WorkspaceContextMenuProps {
  position: { x: number; y: number } | null;
  entry: WorkspaceFileEntry | null;
  canCreateChildren: boolean;
  onUpload: () => void;
  onCreateFile: () => void;
  onCreateFolder: () => void;
  onDownload: () => void;
  onRename: () => void;
  onDelete: () => void;
  onClose: () => void;
}

export function WorkspaceContextMenu({
  position,
  entry,
  canCreateChildren: canCreateChildren,
  onUpload: onUpload,
  onCreateFile: onCreateFile,
  onCreateFolder: onCreateFolder,
  onDownload: onDownload,
  onRename: onRename,
  onDelete: onDelete,
  onClose: onClose,
}: WorkspaceContextMenuProps) {
  const { t } = useI18n();
  const menuRef = useRef<HTMLDivElement>(null);
  const fileActionCopy = getWorkspaceFileExternalActionCopy(entry?.name);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        onClose();
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [onClose]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [onClose]);

  if (!position) {
    return null;
  }

  return createPortal(
    <div
      ref={menuRef}
      className={DIALOG_POPOVER_CLASS_NAME}
      style={{
        top: `${position.y}px`,
        left: `${position.x}px`,
        minWidth: "180px",
      }}
    >
      <div className="py-1">
        {canCreateChildren ? (
          <>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onUpload(); onClose(); }}
            >
              <Upload className="h-4 w-4" />
              <span>{t("room.workspace_action_upload")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onCreateFile(); onClose(); }}
            >
              <FilePlus className="h-4 w-4" />
              <span>{t("room.workspace_action_new_file")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onCreateFolder(); onClose(); }}
            >
              <FolderPlus className="h-4 w-4" />
              <span>{t("room.workspace_action_new_folder")}</span>
            </button>
            {entry ? <div className="my-1 h-px bg-(--divider-subtle-color)" /> : null}
          </>
        ) : null}

        {entry ? (
          <>
            {!entry.is_dir ? (
              <button
                aria-label={fileActionCopy.ariaLabel}
                type="button"
                className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
                onClick={() => { onDownload(); onClose(); }}
                title={fileActionCopy.title}
              >
                {fileActionCopy.mode === "reveal" ? (
                  <FolderOpen className="h-4 w-4" />
                ) : (
                  <Download className="h-4 w-4" />
                )}
                <span>{fileActionCopy.label}</span>
              </button>
            ) : null}
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => { onRename(); onClose(); }}
            >
              <Pencil className="h-4 w-4" />
              <span>{t("home.rename")}</span>
            </button>
            <button
              type="button"
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-destructive"
              onClick={() => { onDelete(); onClose(); }}
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
