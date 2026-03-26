"use client";

import { Home, MessageCircleMore, Sparkles, Users, Waypoints } from "lucide-react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { cn } from "@/lib/utils";

export type AppGlobalRailKey = "home" | "nexus" | "dms" | "rooms" | "contacts";

interface AppGlobalRailProps {
  active_item: AppGlobalRailKey;
  dm_href?: string;
  room_href?: string;
}

interface RailItem {
  key: AppGlobalRailKey;
  label: string;
  to: string;
  icon: typeof Home;
}

export function AppGlobalRail({
  active_item,
  dm_href,
  room_href,
}: AppGlobalRailProps) {
  const items: RailItem[] = [
    {
      key: "home",
      label: "Home",
      to: AppRouteBuilders.launcher(),
      icon: Home,
    },
    {
      key: "nexus",
      label: "Nexus",
      to: AppRouteBuilders.launcher_app(),
      icon: Sparkles,
    },
    {
      key: "dms",
      label: "DMs",
      to: dm_href || AppRouteBuilders.launcher(),
      icon: MessageCircleMore,
    },
    {
      key: "rooms",
      label: "Rooms",
      to: room_href || AppRouteBuilders.launcher(),
      icon: Waypoints,
    },
    {
      key: "contacts",
      label: "Contacts",
      to: AppRouteBuilders.contacts(),
      icon: Users,
    },
  ];

  return (
    <aside className="hidden h-full w-[76px] shrink-0 flex-col items-center border-r border-white/18 bg-[linear-gradient(180deg,rgba(42,56,92,0.9),rgba(28,36,62,0.82))] px-3 py-5 shadow-[inset_-1px_0_0_rgba(255,255,255,0.08)] lg:flex">
      <Link
        className="mb-6 flex h-12 w-12 items-center justify-center rounded-[18px] border border-white/14 bg-white/12 text-lg font-black tracking-[-0.06em] text-white/92 shadow-[0_14px_26px_rgba(8,12,24,0.22)]"
        to={AppRouteBuilders.launcher()}
      >
        N
      </Link>

      <nav className="flex w-full flex-1 flex-col items-center gap-2">
        {items.map((item) => {
          const Icon = item.icon;
          const is_active = item.key === active_item;

          return (
            <Link
              key={item.key}
              aria-current={is_active ? "page" : undefined}
              className={cn(
                "group flex w-full flex-col items-center gap-1 rounded-[20px] px-2 py-3 text-[11px] font-semibold tracking-[0.01em] transition-all duration-300",
                is_active
                  ? "bg-white/16 text-white shadow-[0_16px_30px_rgba(8,12,24,0.24)]"
                  : "text-white/64 hover:bg-white/10 hover:text-white/90",
              )}
              to={item.to}
            >
              <div
                className={cn(
                  "flex h-9 w-9 items-center justify-center rounded-[14px] transition-all duration-300",
                  is_active ? "bg-white/18" : "bg-white/8 group-hover:bg-white/12",
                )}
              >
                <Icon className="h-4.5 w-4.5" />
              </div>
              <span>{item.label}</span>
            </Link>
          );
        })}
      </nav>

      <div className="mt-4 flex h-11 w-11 items-center justify-center rounded-full border border-white/12 bg-white/8 text-sm font-bold text-white/80">
        AG
      </div>
    </aside>
  );
}
