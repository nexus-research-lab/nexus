// INPUT: 侧栏展开状态、可见系统动作、路由状态与动作命令。
// OUTPUT: 账号入口、向上展开的设置/退出菜单与右侧帮助入口。
// POS: 宽侧栏底部/折叠动作视图；权限与更新状态由上层和专属 hook 决定。

import {
  CircleHelp,
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
import { cn } from "@/shared/ui/class-name";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { UiActionMenu, type UiActionMenuItem } from "@/shared/ui/menu/action-menu";

import { SidebarUpdateIndicator } from "./sidebar-update-indicator";
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
  const navigate = useNavigate();
  const anchorRef = useRef<HTMLButtonElement>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuWidth, setMenuWidth] = useState(220);
  const items: UiActionMenuItem[] = props.showSettings ? [{
    value: "settings",
    label: props.labels.settings,
    icon: <Settings className="h-4 w-4" />,
    active: props.settingsActive,
  }] : [];
  const footerItems: UiActionMenuItem[] = [
    ...(props.showLogin ? [{
      value: "login",
      label: props.labels.login,
      icon: <LogIn className="h-4 w-4" />,
    }] : []),
    ...(props.showLogout ? [{
      value: "logout",
      label: props.labels.logout,
      icon: <LogOut className="h-4 w-4" />,
    }] : []),
  ];

  return (
    <div className={cn(
      "sidebar-panel-footer shell-region-footer relative -mr-1.5 flex shrink-0 items-center gap-2 px-2",
      "h-12",
    )}>
      <UiButton
        ref={anchorRef}
        aria-label={props.accountName}
        aria-expanded={menuOpen}
        aria-haspopup="menu"
        className="min-w-0 flex-1 justify-start gap-2 px-1 font-normal"
        onClick={() => {
          const footer = anchorRef.current?.parentElement;
          if (footer) {
            const style = getComputedStyle(footer);
            setMenuWidth(footer.getBoundingClientRect().width - (parseFloat(style.paddingLeft) || 0) - (parseFloat(style.paddingRight) || 0));
          }
          setMenuOpen(!menuOpen);
        }}
        variant="ghost"
      >
        <UiAgentAvatar avatar={props.accountAvatar} className="rounded-full" name={props.accountName} size="sm" />
        <span className="truncate">{props.accountName}</span>
      </UiButton>
      <UiActionMenu
        anchorRef={anchorRef}
        ariaLabel={props.accountName}
        isOpen={menuOpen}
        header={
          <div className="flex min-w-0 items-center gap-2.5">
            <UiAgentAvatar avatar={props.accountAvatar} className="rounded-full" name={props.accountName} size="sm" />
            <span className="ui-type-control truncate text-(--text-strong)">{props.accountName}</span>
          </div>
        }
        items={[...items, ...footerItems]}
        minWidth={menuWidth}
        placement="top"
        onClose={() => setMenuOpen(false)}
        onSelect={(value) => {
          if (value === "login") props.onLogin();
          if (value === "logout") props.onLogout();
          if (value === "settings") navigate(AppRouteBuilders.settings());
        }}
      />
      {updateVersion ? <SidebarUpdateIndicator version={updateVersion} /> : null}
      <UtilityButton
        active={props.guideOpen}
        anchor={SIDEBAR_TOUR_ANCHORS.restart}
        icon={CircleHelp}
        label={props.labels.guide}
        onClick={props.onOpenGuide}
      />
    </div>
  );
}

function UtilityButton({
  active,
  anchor,
  icon: Icon,
  iconClassName = "h-[18px] w-[18px]",
  label,
  onClick,
}: {
  active?: boolean;
  anchor?: string;
  icon: LucideIcon;
  iconClassName?: string;
  label: string;
  onClick: () => void;
}) {
  return (
    <UiIconButton
      aria-label={label}
      aria-pressed={active}
      data-tour-anchor={anchor}
      onClick={onClick}
      shape="round"
      size="md"
      tooltip={label}
    >
      <Icon className={iconClassName} />
    </UiIconButton>
  );
}
