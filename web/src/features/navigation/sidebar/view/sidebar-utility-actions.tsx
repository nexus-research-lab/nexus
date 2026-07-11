import {
  Compass,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  type LucideIcon,
} from "lucide-react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { cn } from "@/lib/utils";

import type { SidebarUtilityLabels } from "./sidebar-wide-panel-types";

interface SidebarUtilityActionsProps {
  guideOpen: boolean;
  labels: SidebarUtilityLabels;
  onCollapse: () => void;
  onExpand: () => void;
  onLogout: () => void;
  onOpenGuide: () => void;
  settingsActive: boolean;
  showLogout: boolean;
  showSettings: boolean;
  variant: "rail" | "panel";
}

export function SidebarUtilityActions(props: SidebarUtilityActionsProps) {
  const primaryActions = (
    <>
      {props.showSettings ? (
        <UtilityLink
          active={props.settingsActive}
          icon={Settings}
          label={props.labels.settings}
          to={AppRouteBuilders.settings()}
        />
      ) : null}
      <UtilityButton
        active={props.guideOpen}
        anchor={SIDEBAR_TOUR_ANCHORS.restart}
        icon={Compass}
        label={props.labels.guide}
        onClick={props.onOpenGuide}
      />
    </>
  );
  const panelActions = (
    <>
      {props.showLogout ? (
        <UtilityButton
          icon={LogOut}
          label={props.labels.logout}
          onClick={props.onLogout}
        />
      ) : null}
      <UtilityButton
        icon={props.variant === "rail" ? PanelLeftOpen : PanelLeftClose}
        label={
          props.variant === "rail" ? props.labels.expand : props.labels.collapse
        }
        onClick={props.variant === "rail" ? props.onExpand : props.onCollapse}
      />
    </>
  );

  if (props.variant === "rail") {
    return (
      <div className="flex flex-col items-center gap-1.5 border-t divider-subtle py-3">
        {primaryActions}
        {panelActions}
      </div>
    );
  }
  return (
    <div className="relative flex items-center justify-between gap-2.5 border-t divider-subtle px-3 py-3">
      <div className="flex items-center gap-2.5">{primaryActions}</div>
      <div className="min-w-0 flex-1" />
      <div className="flex items-center gap-2.5">{panelActions}</div>
    </div>
  );
}

function UtilityLink({
  active,
  icon: Icon,
  label,
  to,
}: {
  active: boolean;
  icon: LucideIcon;
  label: string;
  to: string;
}) {
  return (
    <Link
      aria-label={label}
      className={utilityActionClassName(active)}
      title={label}
      to={to}
    >
      <Icon className="h-4 w-4" />
    </Link>
  );
}

function UtilityButton({
  active = false,
  anchor,
  icon: Icon,
  label,
  onClick,
}: {
  active?: boolean;
  anchor?: string;
  icon: LucideIcon;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      aria-label={label}
      className={utilityActionClassName(active)}
      data-tour-anchor={anchor}
      onClick={onClick}
      title={label}
      type="button"
    >
      <Icon className="h-4 w-4" />
    </button>
  );
}

function utilityActionClassName(active: boolean): string {
  return cn(
    "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    active &&
      "bg-(--surface-interactive-active-background) text-(--text-strong)",
  );
}
