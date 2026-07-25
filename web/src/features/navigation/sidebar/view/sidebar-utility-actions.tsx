import {
  Compass,
  LogOut,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  type LucideIcon,
} from "lucide-react";
import type { CSSProperties } from "react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { cn } from "@/shared/ui/class-name";

import type { SidebarUtilityLabels } from "./sidebar-wide-panel-types";

interface SidebarUtilityActionsProps {
  collapsed: boolean;
  guideOpen: boolean;
  labels: SidebarUtilityLabels;
  onCollapse: () => void;
  onExpand: () => void;
  onLogout: () => void;
  onOpenGuide: () => void;
  settingsActive: boolean;
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

const FOOTER_ACTION_SIZE = 32;
const FOOTER_ACTION_GAP = 6;
const FOOTER_ACTION_STRIDE = FOOTER_ACTION_SIZE + FOOTER_ACTION_GAP;
const FOOTER_HORIZONTAL_GAP = 10;
const FOOTER_PADDING = 12;
const FOOTER_LEFT = 8;

export function SidebarPanelToggleAction(
  props: SidebarPanelToggleActionProps,
) {
  if (!props.showPanelToggle) {
    return null;
  }
  return (
    <UtilityButton
      icon={props.variant === "rail" ? PanelLeftOpen : PanelLeftClose}
      label={
        props.variant === "rail" ? props.labels.expand : props.labels.collapse
      }
      onClick={props.variant === "rail" ? props.onExpand : props.onCollapse}
    />
  );
}

export function SidebarFooterActions(props: SidebarUtilityActionsProps) {
  const navigationActionCount = props.showSettings ? 2 : 1;
  const actionRowCount = navigationActionCount + (props.showLogout ? 1 : 0);
  const guideIndex = props.showSettings ? 1 : 0;
  const collapsedHeight =
    FOOTER_PADDING * 2
    + actionRowCount * FOOTER_ACTION_SIZE
    + Math.max(0, actionRowCount - 1) * FOOTER_ACTION_GAP;

  return (
    <div
      className="sidebar-panel-footer shell-region-footer relative -mr-1.5 h-14 shrink-0 overflow-hidden max-lg:h-16"
      style={{ height: props.collapsed ? collapsedHeight : undefined }}
    >
      {props.showSettings ? (
        <div
          className="sidebar-panel-footer-action"
          style={getNavigationActionStyle({
            collapsed: props.collapsed,
            index: 0,
            rowCount: actionRowCount,
          })}
        >
          <UtilityLink
            active={props.settingsActive}
            icon={Settings}
            label={props.labels.settings}
            to={AppRouteBuilders.settings()}
          />
        </div>
      ) : null}
      <div
        className="sidebar-panel-footer-action"
        style={getNavigationActionStyle({
          collapsed: props.collapsed,
          index: guideIndex,
          rowCount: actionRowCount,
        })}
      >
        <UtilityButton
          active={props.guideOpen}
          anchor={SIDEBAR_TOUR_ANCHORS.restart}
          icon={Compass}
          label={props.labels.guide}
          onClick={props.onOpenGuide}
        />
      </div>
      {props.showLogout ? (
        <div
          className="sidebar-panel-footer-action"
          style={{ bottom: FOOTER_PADDING, right: FOOTER_LEFT }}
        >
          <UtilityButton
            icon={LogOut}
            label={props.labels.logout}
            onClick={props.onLogout}
          />
        </div>
      ) : null}
    </div>
  );
}

function getNavigationActionStyle({
  collapsed,
  index,
  rowCount,
}: {
  collapsed: boolean;
  index: number;
  rowCount: number;
}): CSSProperties {
  return {
    bottom: collapsed
      ? FOOTER_PADDING + (rowCount - index - 1) * FOOTER_ACTION_STRIDE
      : FOOTER_PADDING,
    left: collapsed
      ? FOOTER_LEFT
      : FOOTER_LEFT + index * (FOOTER_ACTION_SIZE + FOOTER_HORIZONTAL_GAP),
  };
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
      <Icon className="h-[18px] w-[18px]" />
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
      <Icon className="h-[18px] w-[18px]" />
    </button>
  );
}

function utilityActionClassName(active: boolean): string {
  return cn(
    "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-[background,color] duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    active &&
      "bg-(--surface-interactive-active-background) text-(--text-strong)",
  );
}
