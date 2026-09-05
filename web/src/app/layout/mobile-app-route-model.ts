/**
 * INPUT: 当前路径与查询参数。
 * OUTPUT: 移动端目录/详情布局、返回目标和与当前分区一致的页头标题。
 * POS: AppLayout 的纯路由展示模型，不维护导航状态。
 */
import {
  APP_ROUTE_PATHS,
  AppRouteBuilders,
} from "@/shared/navigation/route-paths";
import {
  getSettingsSectionLabelKey,
  parseSettingsSection,
} from "@/features/settings/settings-navigation-model";
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
    detailPrefix: `${APP_ROUTE_PATHS.workGraphDistillations}/`,
    rootPath: APP_ROUTE_PATHS.workGraphDistillations,
    titleKey: "capability.workgraph_distillations",
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
      || searchParams.get("view") === "create"
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
    const searchParams = new URLSearchParams(search);
    return {
      backPath: AppRouteBuilders.home(),
      mode: "detail",
      titleKey: getSettingsSectionLabelKey(parseSettingsSection(searchParams)),
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
