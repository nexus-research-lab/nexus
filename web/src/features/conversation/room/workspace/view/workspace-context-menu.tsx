// INPUT: Workspace 调用元素/原始指针位置、文件、桌面应用目录与外部动作/关闭命令。
// OUTPUT: 复用公共视口定位/模态仲裁的分组与打开方式菜单；超长列表内部滚动，显式退出归还原焦点。
// POS: Workspace 领域菜单；业务动作归调用方，几何/关闭/行节奏/键盘分别由 shared owner 负责。
"use client";

import {
  useCallback,
  useEffect,
  useId,
  useMemo,
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
import { isImeKeyboardEvent } from "@/shared/lib/browser/ime-keyboard-event";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiMenuActionRow } from "@/shared/ui/menu/menu-action-row";
import { focusFirstMenuItem, handleMenuKeyDown } from "@/shared/ui/menu/menu-keyboard";
import { getMenuContentHeight, getMenuItemLayout, MENU_LIST_CLASS_NAME, MENU_SEPARATOR_CLASS_NAME } from "@/shared/ui/menu/menu-styles";
import { useAnchoredOverlayLayer } from "@/shared/ui/overlay/anchored-overlay-layer";
import { resolveUiPointOverlayPosition, resolveUiSideOverlayPosition } from "@/shared/ui/overlay/anchored-overlay-layout";
import { OPEN_OVERLAY_DATA_ATTRIBUTES } from "@/shared/ui/overlay/overlay-contract";
import { ANCHORED_OVERLAY_MOTION_CLASS_NAME, OVERLAY_SURFACE_CLASS_NAME } from "@/shared/ui/overlay/overlay-styles";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

interface WorkspaceContextMenuProps {
  anchor: HTMLElement | null;
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
  anchor,
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
  const anchorRef = useMemo(() => ({ current: anchor }), [anchor]);
  const previousFocusRef = useRef<HTMLElement | null>(null);
  const [openSubmenuId, setOpenSubmenuId] = useState<string | null>(null);
  const isOpen = position !== null && anchor !== null;
  const restoreFocus = useCallback(() => { previousFocusRef.current?.focus(); }, []);
  const closeAndRestoreFocus = () => { onClose(); restoreFocus(); };

  useEffect(() => {
    setOpenSubmenuId(null);
  }, [anchor, entry?.path, position?.x, position?.y]);
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

  const contentHeight = getMenuContentHeight(
    actionGroups.flatMap((group) => group.map(() => getMenuItemLayout().height)),
    Math.max(0, actionGroups.length - 1),
  );
  const estimatePosition = useCallback(() => resolveUiPointOverlayPosition({
    point: position ?? { x: 0, y: 0 },
    preset: "cascade-menu",
    estimatedContentHeight: contentHeight,
  }), [contentHeight, position]);
  const { overlayRef, overlayPosition, overlayStyle, portalContainer } = useAnchoredOverlayLayer({
    anchorRef,
    anchorPress: "outside",
    disabled: false,
    estimatePosition,
    isOpen,
    onClose,
    restoreFocus,
  });
  const isPositioned = overlayPosition !== null;
  useEffect(() => {
    if (!isOpen || !isPositioned) return;
    // StrictMode 重放不会覆盖第一次保存的外部返回位置。
    if (!overlayRef.current?.contains(document.activeElement)) {
      previousFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    }
    focusFirstMenuItem(overlayRef.current);
  }, [anchor, isOpen, isPositioned, overlayRef]);

  if (!isOpen || !portalContainer) return null;
  return createPortal(
    <div
      className={cn(
        MENU_LIST_CLASS_NAME,
        "soft-scrollbar fixed ui-layer-action-menu overflow-y-auto overscroll-contain p-1",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
      )}
      ref={overlayRef}
      aria-label={entry?.name ?? t("room.workspace")}
      data-placement={overlayPosition?.placement ?? "bottom"}
      onKeyDown={(event) => handleMenuKeyDown(event, closeAndRestoreFocus)}
      role="menu"
      tabIndex={-1}
      style={overlayStyle}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      {actionGroups.flatMap((actions, index) => [
        ...(index > 0 ? [<div key={`separator-${index}`} className={MENU_SEPARATOR_CLASS_NAME} role="separator" />] : []),
        ...actions.map((action) => (
          <WorkspaceContextMenuAction
            key={action.id}
            action={action}
            onClose={closeAndRestoreFocus}
            openSubmenuId={openSubmenuId}
            setOpenSubmenuId={setOpenSubmenuId}
          />
        )),
      ])}
    </div>,
    portalContainer,
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

function WorkspaceContextMenuAction({ action, onClose, openSubmenuId, setOpenSubmenuId }: {
  action: WorkspaceMenuAction;
  onClose: () => void;
  openSubmenuId: string | null;
  setOpenSubmenuId: (value: string | null) => void;
}) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const submenuId = useId();
  const [focusSubmenu, setFocusSubmenu] = useState(false);
  const { ariaLabel, disabled, Icon, id, label, onSelect, submenu, title, tone } = action;
  const isSubmenuOpen = openSubmenuId === id;
  const closeSubmenu = () => setOpenSubmenuId(null);
  const select = () => {
    if (disabled) return;
    if (submenu) {
      setFocusSubmenu(true);
      setOpenSubmenuId(id);
    } else {
      onSelect?.();
      onClose();
    }
  };
  return (
    <div>
      <UiMenuActionRow
        ref={triggerRef}
        active={isSubmenuOpen}
        aria-controls={submenu && isSubmenuOpen ? submenuId : undefined}
        aria-expanded={submenu ? isSubmenuOpen : undefined}
        aria-haspopup={submenu ? "menu" : undefined}
        aria-label={ariaLabel}
        disabled={disabled}
        onClick={select}
        onKeyDown={(event) => {
          if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent)) return;
          if (submenu && event.key === "ArrowRight") {
            event.preventDefault();
            select();
          }
        }}
        onPointerEnter={(event) => {
          if (disabled || event.pointerType !== "mouse") return;
          setFocusSubmenu(false);
          setOpenSubmenuId(submenu ? id : null);
        }}
        title={title ?? label}
        tone={tone}
      >
        {Icon ? <Icon className="h-4 w-4 shrink-0" /> : null}
        <span className="min-w-0 flex-1 truncate">{label}</span>
        {submenu ? <ChevronRight className="h-4 w-4 shrink-0" /> : null}
      </UiMenuActionRow>
      {submenu && isSubmenuOpen ? (
        <WorkspaceContextSubmenu
          actions={submenu}
          anchorRef={triggerRef}
          focusRequested={focusSubmenu}
          id={submenuId}
          label={label}
          onClose={closeSubmenu}
          onSelect={onClose}
        />
      ) : null}
    </div>
  );
}

function WorkspaceContextSubmenu({ actions, anchorRef, focusRequested, id, label, onClose, onSelect }: {
  actions: WorkspaceMenuAction[];
  anchorRef: RefObject<HTMLButtonElement | null>;
  focusRequested: boolean;
  id: string;
  label: string;
  onClose: () => void;
  onSelect: () => void;
}) {
  const contentHeight = getMenuContentHeight(actions.map(() => getMenuItemLayout().height));
  const estimatePosition = useCallback((anchor: HTMLButtonElement) => resolveUiSideOverlayPosition({
    anchor,
    preset: "cascade-menu",
    estimatedContentHeight: contentHeight,
  }), [contentHeight]);
  const { overlayRef, overlayPosition, overlayStyle, portalContainer } = useAnchoredOverlayLayer({
    anchorRef, disabled: false, estimatePosition, isOpen: true, onClose,
  });
  const isPositioned = overlayPosition !== null;
  useEffect(() => {
    if (focusRequested && isPositioned) focusFirstMenuItem(overlayRef.current);
  }, [focusRequested, isPositioned, overlayRef]);
  if (!portalContainer) return null;
  return createPortal(
    <div
      aria-label={label}
      className={cn(
        MENU_LIST_CLASS_NAME,
        "soft-scrollbar fixed ui-layer-action-menu overflow-y-auto overscroll-contain p-1",
        OVERLAY_SURFACE_CLASS_NAME,
        ANCHORED_OVERLAY_MOTION_CLASS_NAME,
      )}
      data-placement={overlayPosition?.placement ?? "bottom"}
      id={id}
      ref={overlayRef}
      role="menu"
      tabIndex={-1}
      onKeyDown={(event) => {
        if (event.defaultPrevented || isImeKeyboardEvent(event.nativeEvent)) return;
        if (event.key === "ArrowLeft") {
          event.preventDefault();
          event.stopPropagation();
          onClose();
          anchorRef.current?.focus();
          return;
        }
        handleMenuKeyDown(event, onSelect);
      }}
      style={overlayStyle}
      {...OPEN_OVERLAY_DATA_ATTRIBUTES}
    >
      {actions.map((action) => (
        <UiMenuActionRow
          key={action.id}
          onClick={() => {
            if (action.disabled) return;
            action.onSelect?.();
            onSelect();
          }}
          title={action.label}
          disabled={action.disabled}
        >
          {action.Icon ? (
            <action.Icon className={action.id === "open-with-loading"
              ? getUiSpinnerClassName({ size: "md", tone: "muted" })
              : "h-4 w-4 shrink-0"} />
          ) : null}
          <span className="min-w-0 flex-1 truncate">{action.label}</span>
        </UiMenuActionRow>
      ))}
    </div>,
    portalContainer,
  );
}
