import {
  APP_ROUTE_PATHS,
  AppRouteBuilders,
} from "@/app/router/route-paths";
import type { TranslationKey } from "@/shared/i18n/messages";

export type MobileAppRoutePresentation =
  | { mode: "conversation" }
  | { mode: "directory" }
  | {
      backPath: string;
      mode: "detail";
      titleKey: TranslationKey;
    };

interface MobileCapabilityRoute {
  detailPrefix?: string;
  rootPath: string;
  titleKey: TranslationKey;
}

const MOBILE_CAPABILITY_ROUTES: MobileCapabilityRoute[] = [
  {
    detailPrefix: `${APP_ROUTE_PATHS.skills}/`,
    rootPath: APP_ROUTE_PATHS.skills,
    titleKey: "capability.skills",
  },
  {
    detailPrefix: `${APP_ROUTE_PATHS.connectors}/`,
    rootPath: APP_ROUTE_PATHS.connectors,
    titleKey: "capability.connectors",
  },
  {
    detailPrefix: `${APP_ROUTE_PATHS.loops}/`,
    rootPath: APP_ROUTE_PATHS.loops,
    titleKey: "capability.loops",
  },
  {
    rootPath: APP_ROUTE_PATHS.scheduledTasks,
    titleKey: "capability.scheduled",
  },
  {
    rootPath: APP_ROUTE_PATHS.channels,
    titleKey: "capability.channels",
  },
  {
    rootPath: APP_ROUTE_PATHS.pairings,
    titleKey: "capability.pairings",
  },
];

export function resolveMobileAppRoute({
  pathname,
  search,
}: {
  pathname: string;
  search: string;
}): MobileAppRoutePresentation {
  if (pathname.startsWith("/rooms/")) {
    return { mode: "conversation" };
  }
  if (pathname === APP_ROUTE_PATHS.home) {
    return { mode: "directory" };
  }
  if (pathname === APP_ROUTE_PATHS.contacts) {
    const searchParams = new URLSearchParams(search);
    const opensContactContent = (
      searchParams.has("agent")
      || searchParams.get("view") === "manage"
    );
    return opensContactContent
      ? {
          backPath: AppRouteBuilders.contacts(),
          mode: "detail",
          titleKey: "sidebar.tab_contacts",
        }
      : { mode: "directory" };
  }
  if (pathname === APP_ROUTE_PATHS.capability) {
    return { mode: "directory" };
  }

  const capabilityRoute = MOBILE_CAPABILITY_ROUTES.find((route) => (
    pathname === route.rootPath
    || Boolean(route.detailPrefix && pathname.startsWith(route.detailPrefix))
  ));
  if (capabilityRoute) {
    const opensCapabilityDetail = (
      capabilityRoute.detailPrefix !== undefined
      && pathname.startsWith(capabilityRoute.detailPrefix)
    );
    return {
      backPath: opensCapabilityDetail
        ? capabilityRoute.rootPath
        : AppRouteBuilders.capability(),
      mode: "detail",
      titleKey: capabilityRoute.titleKey,
    };
  }
  if (pathname === APP_ROUTE_PATHS.settings) {
    return {
      backPath: AppRouteBuilders.home(),
      mode: "detail",
      titleKey: "settings.title",
    };
  }
  if (pathname === APP_ROUTE_PATHS.operations) {
    return {
      backPath: AppRouteBuilders.home(),
      mode: "detail",
      titleKey: "operations.title",
    };
  }
  return {
    backPath: AppRouteBuilders.home(),
    mode: "detail",
    titleKey: "sidebar.tab_chat",
  };
}
