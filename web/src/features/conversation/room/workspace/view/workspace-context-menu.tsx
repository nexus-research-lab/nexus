"use client";

import {
  useEffect,
  useRef,
  useState,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";
import {
  AppWindow,
  ChevronRight,
  Copy,
  Download,
  ExternalLink,
  FilePlus,
  FolderOpen,
  FolderPlus,
  LoaderCircle,
  MessageSquarePlus,
  Pencil,
  Trash2,
  Upload,
  type LucideIcon,
} from "lucide-react";

import {
  getDesktopRuntimeConfig,
  isDesktopRuntime,
} from "@/config/desktop-runtime";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import type {
  DesktopFileApplicationsResult,
  DesktopWorkspaceFileOpenTarget,
} from "@/lib/desktop-bridge/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiMenuActionRow } from "@/shared/ui/menu/menu-action-row";
import { MENU_LIST_CLASS_NAME } from "@/shared/ui/menu/menu-styles";
import { OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

interface WorkspaceContextMenuProps {
  canCreateChildren: boolean;
  entry: WorkspaceFileEntry | null;
  isLoadingOpenApplications: boolean;
  onAddToChat: () => void;
  onClose: () => void;
  onCopyPath: () => void;
  onCreateFile: () => void;
  onCreateFolder: () => void;
  onDelete: () => void;
  onDownload: () => void;
  onOpen: (
    target: DesktopWorkspaceFileOpenTarget,
    applicationPath?: string,
  ) => void;
  onRename: () => void;
  onUpload: () => void;
  position: { x: number; y: number } | null;
  openApplications: DesktopFileApplicationsResult | null;
}

interface WorkspaceMenuAction {
  ariaLabel?: string;
  disabled?: boolean;
  Icon?: LucideIcon;
  id: string;
  label: string;
  onSelect?: () => void;
  submenu?: WorkspaceMenuAction[];
  title?: string;
  tone?: "danger";
}

export function WorkspaceContextMenu({
  canCreateChildren,
  entry,
  isLoadingOpenApplications,
  onAddToChat,
  onClose,
  onCopyPath,
  onCreateFile,
  onCreateFolder,
  onDelete,
  onDownload,
  onOpen,
  onRename,
  onUpload,
  openApplications,
  position,
}: WorkspaceContextMenuProps) {
  const { t } = useI18n();
  const menuRef = useRef<HTMLDivElement>(null);
  const [openSubmenuId, setOpenSubmenuId] = useState<string | null>(null);
  useWorkspaceContextMenuDismiss(menuRef, position !== null, onClose);

  useEffect(() => {
    setOpenSubmenuId(null);
  }, [entry?.path, position?.x, position?.y]);

  if (!position) {
    return null;
  }

  const isDesktopFile = isDesktopRuntime() && Boolean(entry && !entry.is_dir);
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
  const actionGroups = [
    createActions,
    ...buildEntryActionGroups({
      deleteLabel: t("common.delete"),
      entry,
      isLoadingOpenApplications,
      onAddToChat,
      onCopyPath,
      onDelete,
      onDownload,
      onOpen,
      onRename,
      openApplications,
      renameLabel: t("home.rename"),
      translate: t,
    }),
  ].filter((group) => group.length > 0);

  return createPortal(
    <div
      className={cn(
        "fixed ui-layer-action-menu overflow-visible",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      ref={menuRef}
      role="menu"
      style={{
        left: `${position.x}px`,
        minWidth: isDesktopFile ? "200px" : "180px",
        top: `${position.y}px`,
      }}
    >
      <div className="p-1">
        {actionGroups.map((actions, index) => (
          <div key={actions[0]?.id}>
            {index > 0 ? (
              <div className="mx-1 my-1 h-px bg-(--divider-subtle-color)" />
            ) : null}
            <WorkspaceContextMenuActions
              actions={actions}
              onClose={onClose}
              openSubmenuId={openSubmenuId}
              position={position}
              setOpenSubmenuId={setOpenSubmenuId}
            />
          </div>
        ))}
      </div>
    </div>,
    document.body,
  );
}

function buildEntryActionGroups({
  deleteLabel,
  entry,
  isLoadingOpenApplications,
  onAddToChat,
  onCopyPath,
  onDelete,
  onDownload,
  onOpen,
  onRename,
  openApplications,
  renameLabel,
  translate,
}: {
  deleteLabel: string;
  entry: WorkspaceFileEntry | null;
  isLoadingOpenApplications: boolean;
  onAddToChat: () => void;
  onCopyPath: () => void;
  onDelete: () => void;
  onDownload: () => void;
  onOpen: (
    target: DesktopWorkspaceFileOpenTarget,
    applicationPath?: string,
  ) => void;
  onRename: () => void;
  openApplications: DesktopFileApplicationsResult | null;
  renameLabel: string;
  translate: ReturnType<typeof useI18n>["t"];
}): WorkspaceMenuAction[][] {
  if (!entry) {
    return [];
  }

  const groups: WorkspaceMenuAction[][] = [];
  const editActions: WorkspaceMenuAction[] = [
    { Icon: Pencil, id: "rename", label: renameLabel, onSelect: onRename },
    {
      Icon: Trash2,
      id: "delete",
      label: deleteLabel,
      onSelect: onDelete,
      tone: "danger",
    },
  ];
  if (!entry.is_dir) {
    if (isDesktopRuntime()) {
      groups.push(
        [
          {
            Icon: ExternalLink,
            id: "open-default",
            label: translate("room.workspace_open_in_app", {
              name: openApplications?.default_application?.name
                || translate("room.workspace_default_app"),
            }),
            onSelect: () => onOpen("default"),
          },
          {
            Icon: AppWindow,
            id: "open-with",
            label: translate("room.workspace_open_with"),
            submenu: buildOpenWithActions({
              isLoading: isLoadingOpenApplications,
              onOpen,
              openApplications,
              translate,
            }),
          },
        ],
        [
          {
            Icon: Copy,
            id: "copy-path",
            label: translate("room.workspace_copy_path"),
            onSelect: onCopyPath,
          },
          {
            Icon: MessageSquarePlus,
            id: "add-to-chat",
            label: translate("room.workspace_add_to_chat"),
            onSelect: onAddToChat,
          },
        ],
      );
    } else {
      const copy = getWorkspaceFileExternalActionCopy(translate, entry.name);
      return [[
        {
          ariaLabel: copy.ariaLabel,
          Icon: copy.mode === "reveal" ? FolderOpen : Download,
          id: "external-file",
          label: copy.label,
          onSelect: onDownload,
          title: copy.title,
        },
        ...editActions,
      ]];
    }
  }
  groups.push(editActions);
  return groups;
}

function buildOpenWithActions({
  isLoading,
  onOpen,
  openApplications,
  translate,
}: {
  isLoading: boolean;
  onOpen: (
    target: DesktopWorkspaceFileOpenTarget,
    applicationPath?: string,
  ) => void;
  openApplications: DesktopFileApplicationsResult | null;
  translate: ReturnType<typeof useI18n>["t"];
}): WorkspaceMenuAction[] {
  const platform = getDesktopRuntimeConfig()?.platform;
  const fileManagerLabel = platform === "macos" ? "Finder" : "File Explorer";
  const defaultApplication = openApplications?.default_application;
  const actions: WorkspaceMenuAction[] = [
    {
      id: "open-with-default",
      label: defaultApplication?.name || translate("room.workspace_default_app"),
      onSelect: () => onOpen("default"),
    },
    {
      id: "open-with-file-manager",
      label: fileManagerLabel,
      onSelect: () => onOpen("file_manager"),
    },
  ];
  if (platform === "macos") {
    actions.push({
      id: "open-with-terminal",
      label: "Terminal",
      onSelect: () => onOpen("terminal"),
    });
  }
  if (isLoading) {
    actions.push({
      disabled: true,
      Icon: LoaderCircle,
      id: "open-with-loading",
      label: translate("room.workspace_loading_applications"),
    });
    return actions;
  }
  const fixedApplicationPaths = new Set([
    defaultApplication?.path,
  ].filter((path): path is string => Boolean(path)));
  const fixedApplicationNames = new Set(
    platform === "macos"
      ? [fileManagerLabel, "Terminal"]
      : [fileManagerLabel],
  );
  return actions.concat(
    (openApplications?.applications ?? [])
      .filter((application) => (
        !fixedApplicationPaths.has(application.path)
        && !fixedApplicationNames.has(application.name)
      ))
      .map((application) => ({
        id: `open-with-${application.path}`,
        label: application.name,
        onSelect: () => onOpen("application", application.path),
      })),
  );
}

function WorkspaceContextMenuActions({
  actions,
  onClose,
  openSubmenuId,
  position,
  setOpenSubmenuId,
}: {
  actions: WorkspaceMenuAction[];
  onClose: () => void;
  openSubmenuId: string | null;
  position: { x: number; y: number };
  setOpenSubmenuId: (value: string | null) => void;
}) {
  return (
    <div className={MENU_LIST_CLASS_NAME} role="none">
      {actions.map((action) => {
        const {
          ariaLabel,
          disabled,
          Icon,
          id,
          label,
          onSelect,
          submenu,
          title,
          tone,
        } = action;
        const isSubmenuOpen = openSubmenuId === id;
        return (
          <div
            className="relative"
            key={id}
            onPointerLeave={() => submenu && setOpenSubmenuId(null)}
          >
            <UiMenuActionRow
              active={isSubmenuOpen}
              aria-expanded={submenu ? isSubmenuOpen : undefined}
              aria-haspopup={submenu ? "menu" : undefined}
              aria-label={ariaLabel}
              disabled={disabled}
              onClick={() => {
                if (disabled) {
                  return;
                }
                if (submenu) {
                  setOpenSubmenuId(id);
                  return;
                }
                onSelect?.();
                onClose();
              }}
              onKeyDown={(event) => {
                if (submenu && event.key === "ArrowRight") {
                  event.preventDefault();
                  setOpenSubmenuId(id);
                }
              }}
              onPointerEnter={() => setOpenSubmenuId(submenu ? id : null)}
              title={title}
              tone={tone}
            >
              {Icon ? <Icon className="h-4 w-4" /> : null}
              <span className="min-w-0 flex-1 truncate">{label}</span>
              {submenu ? <ChevronRight className="h-4 w-4 shrink-0" /> : null}
            </UiMenuActionRow>

            {submenu && isSubmenuOpen ? (
              <WorkspaceContextSubmenu
                actions={submenu}
                maxHeight={window.innerHeight - position.y - 8}
                onClose={onClose}
                openOnLeft={position.x + 384 > window.innerWidth}
              />
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

function WorkspaceContextSubmenu({
  actions,
  maxHeight,
  onClose,
  openOnLeft,
}: {
  actions: WorkspaceMenuAction[];
  maxHeight: number;
  onClose: () => void;
  openOnLeft: boolean;
}) {
  return (
    <div
      className={cn(
        MENU_LIST_CLASS_NAME,
        "absolute -top-9 w-[180px] overflow-y-auto p-1",
        openOnLeft ? "right-[calc(100%+4px)]" : "left-[calc(100%+4px)]",
        OVERLAY_SURFACE_CLASS_NAME,
      )}
      role="menu"
      style={{maxHeight: `${Math.max(36, maxHeight)}px`}}
    >
      {actions.map((action) => (
        <UiMenuActionRow
          key={action.id}
          onClick={() => {
            if (action.disabled) {
              return;
            }
            action.onSelect?.();
            onClose();
          }}
          title={action.label}
          disabled={action.disabled}
        >
          {action.Icon ? (
            <action.Icon
              className={action.id === "open-with-loading"
                ? getUiSpinnerClassName(
                    { size: "md", tone: "muted" },
                    "mr-2",
                  )
                : "mr-2 h-4 w-4 shrink-0"}
            />
          ) : null}
          <span className="truncate">{action.label}</span>
        </UiMenuActionRow>
      ))}
    </div>
  );
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
