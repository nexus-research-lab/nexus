// INPUT: 侧栏展开状态、可见系统动作、路由状态与动作命令。
// OUTPUT: 紧凑账号菜单集中提供引导与有效退出，设置与升级提示收进账号菜单。
// POS: 宽侧栏底部/折叠动作视图；权限与更新状态由上层和专属 hook 决定。

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import {
  CircleHelp,
  Download,
  LogIn,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  type LucideIcon,
} from "lucide-react";
import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiActionMenu, type UiActionMenuItem } from "@/shared/ui/menu/action-menu";

import { startDesktopUpdate } from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { SidebarUtilityLabels } from "./sidebar-wide-panel-types";
import { useSidebarUpdateVersion } from "./use-sidebar-update-version";

export interface SidebarUtilityActionsProps {
  accountName: string;
  accountAvatar?: string | null;
  guideOpen: boolean;
  labels: SidebarUtilityLabels;
  onCollapse: () => void;
  onExpand: () => void;
  onLogin: () => void;
  onLogout: () => void;
  onOpenGuide: () => void;
  settingsActive: boolean;
  showLogin: boolean;
  showLogout: boolean;
  showPanelToggle: boolean;
  showSettings: boolean;
}

interface SidebarPanelToggleActionProps {
  labels: Pick<SidebarUtilityLabels, "collapse" | "expand">;
  onCollapse: () => void;
  onExpand: () => void;
  showPanelToggle: boolean;
  variant: "rail" | "panel";
}

export function SidebarPanelToggleAction(
  props: SidebarPanelToggleActionProps,
) {
  if (!props.showPanelToggle) {
    return null;
  }
  return (
    <UtilityButton
      icon={props.variant === "rail" ? PanelLeftOpen : PanelLeftClose}
      iconClassName="h-5 w-5"
      label={
        props.variant === "rail" ? props.labels.expand : props.labels.collapse
      }
      onClick={props.variant === "rail" ? props.onExpand : props.onCollapse}
    />
  );
}

export function SidebarFooterActions(props: SidebarUtilityActionsProps) {
  const updateVersion = useSidebarUpdateVersion();
  const { t } = useI18n();
  const navigate = useNavigate();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState(260);
  const [updateStarting, setUpdateStarting] = useState(false);
  const startUpdate = async () => {
    if (updateStarting) return;
    setUpdateStarting(true);
    try {
      const result = await startDesktopUpdate();
      if (result.status === "disabled" || result.status === "unavailable") {
        throw new Error(`Desktop update is ${result.status}`);
      }
    } catch (error) {
      console.error("[DesktopUpdate] Failed to start native update:", error);
    } finally {
      setUpdateStarting(false);
    }
  };
  const items: UiActionMenuItem[] = [{
    value: "guide",
    label: props.labels.guide,
    icon: <CircleHelp aria-hidden="true" className="h-4 w-4" />,
    active: props.guideOpen,
  }, ...(props.showSettings ? [{
    value: "settings",
    label: props.labels.settings,
    icon: <Settings aria-hidden="true" className="h-4 w-4" />,
    active: props.settingsActive,
    trailing: updateVersion ? <span className="text-xs text-(--primary)">{updateVersion}</span> : undefined,
  }] : []), ...(updateVersion ? [{
    value: "update",
    label: updateStarting ? t("sidebar.update_starting") : t("sidebar.update_available", { version: updateVersion }),
    disabled: updateStarting,
    icon: <Download aria-hidden="true" className="h-4 w-4" />,
  }] : []), ...(props.showLogin ? [{
    value: "login",
    label: props.labels.login,
    icon: <LogIn aria-hidden="true" className="h-4 w-4" />,
  }] : []), ...(props.showLogout ? [{
    value: "logout",
    label: props.labels.logout,
    icon: <LogOut aria-hidden="true" className="h-4 w-4" />,
  }] : [])];

  return (
    <div className="sidebar-panel-footer shell-region-footer relative -mr-1.5 flex h-12 shrink-0 items-center justify-end gap-2 px-2">
      <UiButton
        ref={anchorRef}
        data-tour-anchor={SIDEBAR_TOUR_ANCHORS.restart}
        aria-label={props.accountName}
        aria-expanded={menuOpen}
        aria-haspopup="menu"
        className="h-9 min-w-0 flex-1 justify-start gap-2 px-1"
        onClick={() => {
          setMenuWidth(Math.max(260, anchorRef.current?.parentElement?.getBoundingClientRect().width ?? 0));
          setMenuOpen(!menuOpen);
        }}
        size="md"
        title={props.accountName}
        variant="ghost"
      >
        <UiAgentAvatar aria-hidden="true" avatar={props.accountAvatar} className="rounded-full" name={props.accountName} size="sm" />
        <span className="min-w-0 truncate text-left font-normal">{props.accountName}</span>
      </UiButton>
      <UiActionMenu
        anchorRef={anchorRef}
        ariaLabel={props.accountName}
        isOpen={menuOpen}
        items={[{
          value: "personal",
          label: <span className="flex min-w-0 items-center gap-2.5">
            <UiAgentAvatar aria-hidden="true" avatar={props.accountAvatar} className="rounded-full" name={props.accountName} size="sm" />
            <UiTooltip label={props.accountName}><span className="ui-type-control truncate text-(--text-strong)" >{props.accountName}</span></UiTooltip>
          </span>,
        }]}
        footerItems={items}
        minWidth={menuWidth}
        placement="top"
        onClose={() => setMenuOpen(false)}
        onSelect={(value) => {
          if (value === "settings") navigate(AppRouteBuilders.settings());
          if (value === "update") void startUpdate();
          if (value === "personal") navigate(AppRouteBuilders.settings("personal"));
          if (value === "login") props.onLogin();
          if (value === "logout") props.onLogout();
          if (value === "guide") props.onOpenGuide();
        }}
      />
      {props.showSettings ? (
        <UtilityButton
          active={props.settingsActive}
          icon={Settings}
          label={props.labels.settings}
          onClick={() => navigate(AppRouteBuilders.settings())}
        />
      ) : null}
    </div>
  );
}

function UtilityButton({
  active,
  icon: Icon,
  iconClassName = "h-[18px] w-[18px]",
  label,
  onClick,
}: {
  active?: boolean;
  icon: LucideIcon;
  iconClassName?: string;
  label: string;
  onClick: () => void;
}) {
  return (
    <UiIconButton
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
      shape="round"
      size="md"
      tooltip={label}
    >
      <Icon aria-hidden="true" className={iconClassName} />
    </UiIconButton>
  );
}
