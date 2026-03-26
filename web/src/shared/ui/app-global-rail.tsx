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
      to: dm_href || AppRouteBuilders.dm_directory(),
      icon: MessageCircleMore,
    },
    {
      key: "rooms",
      label: "Rooms",
      to: room_href || AppRouteBuilders.room_directory(),
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
    <aside className="hidden h-full w-[88px] shrink-0 px-3 py-4 lg:flex">
      <div className="home-glass-panel radius-shell-xl flex h-full w-full flex-col items-center px-2 py-4">
        <Link
          className="mb-6 flex h-12 w-12 items-center justify-center rounded-[18px] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(233,229,223,0.92))] text-lg font-black tracking-[-0.06em] text-slate-900 shadow-[0_14px_26px_rgba(102,112,145,0.14)]"
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
                    ? "workspace-card-strong text-slate-950 shadow-[0_16px_30px_rgba(102,112,145,0.14)]"
                    : "text-slate-600 hover:bg-white/30 hover:text-slate-900",
                )}
                to={item.to}
              >
                <div
                  className={cn(
                    "flex h-9 w-9 items-center justify-center rounded-[14px] transition-all duration-300",
                    is_active ? "bg-white/70 text-slate-900" : "bg-white/30 text-slate-600 group-hover:bg-white/48 group-hover:text-slate-900",
                  )}
                >
                  <Icon className="h-4.5 w-4.5" />
                </div>
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>

        <div className="mt-4 flex h-11 w-11 items-center justify-center rounded-full bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(231,228,223,0.94))] text-sm font-bold text-slate-800 shadow-[0_12px_24px_rgba(102,112,145,0.12)]">
          AG
        </div>
      </div>
    </aside>
  );
}
