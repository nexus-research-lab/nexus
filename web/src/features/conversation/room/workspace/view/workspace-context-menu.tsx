"use client";

import { useEffect, useRef, type RefObject } from "react";
import { createPortal } from "react-dom";
import {
  Download,
  FilePlus,
  FolderOpen,
  FolderPlus,
  Pencil,
  Trash2,
  Upload,
  type LucideIcon,
} from "lucide-react";

import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import {
  getMenuItemStateClassName,
  MENU_ITEM_BASE_CLASS_NAME,
} from "@/shared/ui/menu/menu-styles";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

interface WorkspaceContextMenuProps {
  canCreateChildren: boolean;
  entry: WorkspaceFileEntry | null;
  onClose: () => void;
  onCreateFile: () => void;
  onCreateFolder: () => void;
  onDelete: () => void;
  onDownload: () => void;
  onRename: () => void;
  onUpload: () => void;
  position: { x: number; y: number } | null;
}

interface WorkspaceMenuAction {
  ariaLabel?: string;
  Icon: LucideIcon;
  id: string;
  label: string;
  onSelect: () => void;
  title?: string;
  tone?: "danger";
}

export function WorkspaceContextMenu({
  canCreateChildren,
  entry,
  onClose,
  onCreateFile,
  onCreateFolder,
  onDelete,
  onDownload,
  onRename,
  onUpload,
  position,
}: WorkspaceContextMenuProps) {
  const { t } = useI18n();
  const menuRef = useRef<HTMLDivElement>(null);
  useWorkspaceContextMenuDismiss(menuRef, position !== null, onClose);

  if (!position) {
    return null;
  }

  const createActions: WorkspaceMenuAction[] = canCreateChildren ? [
    {
      Icon: Upload,
      id: "upload",
      label: t("room.workspace_action_upload"),
      onSelect: onUpload,
    },
    {
      Icon: FilePlus,
      id: "create-file",
      label: t("room.workspace_action_new_file"),
      onSelect: onCreateFile,
    },
    {
      Icon: FolderPlus,
      id: "create-folder",
      label: t("room.workspace_action_new_folder"),
      onSelect: onCreateFolder,
    },
  ] : [];
  const entryActions = buildEntryActions({
    deleteLabel: t("common.delete"),
    entry,
    onDelete,
    onDownload,
    onRename,
    renameLabel: t("home.rename"),
    translate: t,
  });

  return createPortal(
    <div
      className={cn(
        "fixed z-[130] overflow-hidden",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      ref={menuRef}
      role="menu"
      style={{
        left: `${position.x}px`,
        minWidth: "180px",
        top: `${position.y}px`,
      }}
    >
      <div className="py-1">
        <WorkspaceContextMenuActions actions={createActions} onClose={onClose} />
        {createActions.length > 0 && entryActions.length > 0 ? (
          <div className="my-1 h-px bg-(--divider-subtle-color)" />
        ) : null}
        <WorkspaceContextMenuActions actions={entryActions} onClose={onClose} />
      </div>
    </div>,
    document.body,
  );
}

function buildEntryActions({
  deleteLabel,
  entry,
  onDelete,
  onDownload,
  onRename,
  renameLabel,
  translate,
}: {
  deleteLabel: string;
  entry: WorkspaceFileEntry | null;
  onDelete: () => void;
  onDownload: () => void;
  onRename: () => void;
  renameLabel: string;
  translate: ReturnType<typeof useI18n>["t"];
}): WorkspaceMenuAction[] {
  if (!entry) {
    return [];
  }
  const actions: WorkspaceMenuAction[] = [];
  if (!entry.is_dir) {
    const copy = getWorkspaceFileExternalActionCopy(translate, entry.name);
    actions.push({
      ariaLabel: copy.ariaLabel,
      Icon: copy.mode === "reveal" ? FolderOpen : Download,
      id: "external-file",
      label: copy.label,
      onSelect: onDownload,
      title: copy.title,
    });
  }
  actions.push(
    { Icon: Pencil, id: "rename", label: renameLabel, onSelect: onRename },
    {
      Icon: Trash2,
      id: "delete",
      label: deleteLabel,
      onSelect: onDelete,
      tone: "danger",
    },
  );
  return actions;
}

function WorkspaceContextMenuActions({
  actions,
  onClose,
}: {
  actions: WorkspaceMenuAction[];
  onClose: () => void;
}) {
  return actions.map(({ ariaLabel, Icon, id, label, onSelect, title, tone }) => (
    <button
      aria-label={ariaLabel}
      className={cn(
        MENU_ITEM_BASE_CLASS_NAME,
        "flex min-h-9 items-center gap-2 px-2.5 py-1.5 text-sm",
        getMenuItemStateClassName({ tone: tone ?? "default" }),
      )}
      key={id}
      onClick={() => {
        onSelect();
        onClose();
      }}
      role="menuitem"
      title={title}
      type="button"
    >
      <Icon className="h-4 w-4" />
      <span>{label}</span>
    </button>
  ));
}

function useWorkspaceContextMenuDismiss(
  menuRef: RefObject<HTMLDivElement | null>,
  isOpen: boolean,
  onClose: () => void,
): void {
  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        onClose();
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };
    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, menuRef, onClose]);
}
