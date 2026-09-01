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
import { cn } from "@/shared/ui/class-name";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";

import { SidebarUpdateIndicator } from "./sidebar-update-indicator";
import type { SidebarUtilityLabels } from "./sidebar-wide-panel-types";
import { useSidebarUpdateVersion } from "./use-sidebar-update-version";

interface SidebarUtilityActionsProps {
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
const FOOTER_LEFT = 16;

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

  return (
    <div className="sidebar-panel-footer shell-region-footer relative -mr-1.5 h-14 shrink-0 overflow-hidden max-lg:h-16">
      {props.showSettings ? (
        <div
          className="sidebar-panel-footer-action"
          style={{ bottom: FOOTER_PADDING, left: FOOTER_LEFT }}
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
        style={{
          bottom: FOOTER_PADDING,
          left: FOOTER_LEFT + (props.showSettings
            ? FOOTER_ACTION_SIZE + FOOTER_HORIZONTAL_GAP
            : 0),
        }}
      >
        <UtilityButton
          active={props.guideOpen}
          anchor={SIDEBAR_TOUR_ANCHORS.restart}
          icon={Compass}
          label={props.labels.guide}
          onClick={props.onOpenGuide}
        />
      </div>
      {updateVersion ? (
        <div
          className="sidebar-panel-footer-action"
          style={{
            bottom: FOOTER_PADDING,
            right: props.showLogout
              ? FOOTER_LEFT + FOOTER_ACTION_STRIDE
              : FOOTER_LEFT,
          }}
        >
          <SidebarUpdateIndicator
            className={utilityActionClassName(false)}
            version={updateVersion}
          />
        </div>
      ) : null}
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
    <UiTooltip label={label}>
      <Link
        aria-label={label}
        className={utilityActionClassName(active)}
        to={to}
      >
        <Icon className="h-[18px] w-[18px]" />
      </Link>
    </UiTooltip>
  );
}

function UtilityButton({
  active = false,
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
    <UiTooltip label={label}>
      <button
        aria-label={label}
        className={utilityActionClassName(active)}
        data-tour-anchor={anchor}
        onClick={onClick}
        type="button"
      >
        <Icon className={iconClassName} />
      </button>
    </UiTooltip>
  );
}

function utilityActionClassName(active: boolean): string {
  return cn(
    "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-[background,color] duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    active &&
      "bg-(--surface-interactive-active-background) text-(--text-strong)",
  );
}
