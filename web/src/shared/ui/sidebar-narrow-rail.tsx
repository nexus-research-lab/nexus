/**
 * 侧边栏左侧窄栏（88px）
 *
 * 窄栏：Tab 选择器（工作模式切换）。
 * 每个 Tab 代表一种工作模式，点击切换右侧宽面板内容。
 * 底部 More 按钮弹出菜单提供 Files、Settings、Documentation、Feedback 入口。
 */

import { useCallback, useEffect, useRef, useState } from "react";
import {
  Bell,
  BookOpen,
  FolderOpen,
  Home,
  MessageCircleMore,
  MessageSquare,
  MoreHorizontal,
  Plus,
  Settings,
  Sparkles,
  Users,
} from "lucide-react";
import { Link, useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { cn } from "@/lib/utils";
import {
  type SidebarTabKey,
  derive_tab_from_path,
  useSidebarStore,
} from "@/store/sidebar";

/** Tab 定义 */
interface TabDef {
  key: SidebarTabKey;
  label: string;
  to: string;
  icon: typeof Home;
  /** 是否显示未读红点 */
  show_badge?: boolean;
}

/** Tab 定义：每个 Tab 对应一个路由入口 */
const TAB_ITEMS: TabDef[] = [
  {
    key: "home",
    label: "Home",
    to: AppRouteBuilders.home(),
    icon: Home,
  },
  {
    key: "dms",
    label: "DMs",
    to: AppRouteBuilders.dm_directory(),
    icon: MessageCircleMore,
  },
  {
    key: "activity",
    label: "Activity",
    to: AppRouteBuilders.activity(),
    icon: Bell,
    show_badge: false,
  },
  {
    key: "capabilities",
    label: "Capabilty",
    to: AppRouteBuilders.skills(),
    icon: Sparkles,
  },
  {
    key: "contacts",
    label: "Contacts",
    to: AppRouteBuilders.contacts(),
    icon: Users,
  },
];

/** More 菜单项定义 */
interface MoreMenuItem {
  key: string;
  label: string;
  icon: typeof Home;
  /** 内部路由路径（与 external_url 互斥） */
  to?: string;
  /** 外部链接（与 to 互斥） */
  external_url?: string;
}

/** More 菜单项列表 */
const MORE_MENU_ITEMS: MoreMenuItem[] = [
  {
    key: "files",
    label: "Files",
    icon: FolderOpen,
    to: AppRouteBuilders.files(),
  },
  {
    key: "settings",
    label: "Settings",
    icon: Settings,
    to: AppRouteBuilders.settings(),
  },
];

/** More 菜单分隔线后的外部链接 */
const MORE_MENU_EXTERNAL_ITEMS: MoreMenuItem[] = [
  {
    key: "documentation",
    label: "Documentation",
    icon: BookOpen,
    external_url: "https://docs.nexus.ai",
  },
  {
    key: "feedback",
    label: "Feedback",
    icon: MessageSquare,
    external_url: "https://feedback.nexus.ai",
  },
];

export function SidebarNarrowRail() {
  const location = useLocation();
  const navigate = useNavigate();
  const set_active_tab = useSidebarStore((s) => s.set_active_tab);
  const navigated_from_tab = useSidebarStore((s) => s.navigated_from_tab);
  const set_navigated_from_tab = useSidebarStore((s) => s.set_navigated_from_tab);

  // More 菜单状态
  const [is_more_open, set_is_more_open] = useState(false);
  const more_ref = useRef<HTMLDivElement>(null);

  // 根据当前路由推导激活的 Tab
  const derived_tab = derive_tab_from_path(location.pathname);
  const current_tab = location.pathname.startsWith("/rooms") && navigated_from_tab
    ? navigated_from_tab
    : derived_tab;

  // 点击外部关闭 More 菜单
  useEffect(() => {
    if (!is_more_open) return;
    const handle_click_outside = (e: MouseEvent) => {
      if (more_ref.current && !more_ref.current.contains(e.target as Node)) {
        set_is_more_open(false);
      }
    };
    document.addEventListener("mousedown", handle_click_outside);
    return () => document.removeEventListener("mousedown", handle_click_outside);
  }, [is_more_open]);

  // ESC 关闭 More 菜单
  useEffect(() => {
    if (!is_more_open) return;
    const handle_key_down = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        set_is_more_open(false);
      }
    };
    window.addEventListener("keydown", handle_key_down);
    return () => window.removeEventListener("keydown", handle_key_down);
  }, [is_more_open]);

  /** 处理 More 菜单项点击 */
  const handle_menu_item_click = useCallback(
    (item: MoreMenuItem) => {
      set_is_more_open(false);
      if (item.to) {
        navigate(item.to);
      } else if (item.external_url) {
        window.open(item.external_url, "_blank", "noopener,noreferrer");
      }
    },
    [navigate],
  );

  return (
    <aside className="flex h-full w-[88px] shrink-0 flex-col px-2 py-4">
      <div className="home-glass-panel radius-shell-xl flex h-full w-full flex-col items-center px-2 py-4">
        {/* Logo — 点击回到 Launcher */}
        <Link
          className="mb-6 flex h-12 w-12 items-center justify-center rounded-[18px] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(233,229,223,0.92))] text-lg font-black tracking-[-0.06em] text-slate-900 shadow-[0_14px_26px_rgba(102,112,145,0.14)] transition-transform duration-200 hover:scale-105"
          to={AppRouteBuilders.launcher()}
          title="回到 Launcher"
        >
          N
        </Link>

        {/* Tab 列表 */}
        <nav className="flex w-full flex-1 flex-col items-center gap-2">
          {TAB_ITEMS.map((tab) => {
            const Icon = tab.icon;
            const is_active = tab.key === current_tab;

            return (
              <Link
                key={tab.key}
                aria-current={is_active ? "page" : undefined}
              className={cn(
                  "group relative flex w-full flex-col items-center gap-1 rounded-[20px] px-2 py-3 text-[11px] font-semibold tracking-[0.01em] transition-all duration-300",
                  is_active
                    ? "workspace-card-strong text-slate-950 shadow-[0_16px_30px_rgba(102,112,145,0.14)]"
                    : "text-slate-600 hover:bg-white/30 hover:text-slate-900",
                )}
                to={tab.to}
                onClick={() => {
                  set_active_tab(tab.key);
                  set_navigated_from_tab(tab.key);
                }}
              >
                <div
                  className={cn(
                    "flex h-9 w-9 items-center justify-center rounded-[14px] transition-all duration-300",
                    is_active
                      ? "bg-white/70 text-slate-900"
                      : "bg-white/30 text-slate-600 group-hover:bg-white/48 group-hover:text-slate-900",
                  )}
                >
                  <Icon className="h-4.5 w-4.5" />
                </div>
                <span>{tab.label}</span>

                {/* 未读红点 */}
                {tab.show_badge ? (
                  <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-red-500" />
                ) : null}
              </Link>
            );
          })}

          {/* More 按钮 + 弹出菜单 */}
          <div ref={more_ref} className="relative w-full">
            <button
              className={cn(
                "group flex w-full flex-col items-center gap-1 rounded-[20px] px-2 py-3 text-[11px] font-semibold tracking-[0.01em] transition-all duration-300",
                is_more_open
                  ? "bg-white/30 text-slate-900"
                  : "text-slate-600 hover:bg-white/30 hover:text-slate-900",
              )}
              onClick={() => set_is_more_open((prev) => !prev)}
              title="更多"
            >
              <div
                className={cn(
                  "flex h-9 w-9 items-center justify-center rounded-[14px] transition-all duration-300",
                  is_more_open
                    ? "bg-white/48 text-slate-900"
                    : "bg-white/30 text-slate-600 group-hover:bg-white/48 group-hover:text-slate-900",
                )}
              >
                <MoreHorizontal className="h-4.5 w-4.5" />
              </div>
              <span>More</span>
            </button>

            {/* More 弹出菜单 */}
            {is_more_open ? (
              <div className="absolute bottom-0 left-full z-50 ml-2 w-48 animate-in fade-in slide-in-from-left-2 duration-150">
                <div className="home-glass-panel rounded-[16px] p-1.5 shadow-[0_16px_40px_rgba(102,112,145,0.20)]">
                  {/* 内部导航项 */}
                  {MORE_MENU_ITEMS.map((item) => {
                    const Icon = item.icon;
                    return (
                      <button
                        className="flex w-full items-center gap-2.5 rounded-[12px] px-3 py-2 text-sm text-slate-700 transition-colors hover:bg-white/40 hover:text-slate-900"
                        key={item.key}
                        onClick={() => handle_menu_item_click(item)}
                        type="button"
                      >
                        <Icon className="h-4 w-4 shrink-0" />
                        <span>{item.label}</span>
                      </button>
                    );
                  })}

                  {/* 分隔线 */}
                  <div className="my-1 h-px bg-slate-200/50" />

                  {/* 外部链接项 */}
                  {MORE_MENU_EXTERNAL_ITEMS.map((item) => {
                    const Icon = item.icon;
                    return (
                      <button
                        className="flex w-full items-center gap-2.5 rounded-[12px] px-3 py-2 text-sm text-slate-700 transition-colors hover:bg-white/40 hover:text-slate-900"
                        key={item.key}
                        onClick={() => handle_menu_item_click(item)}
                        type="button"
                      >
                        <Icon className="h-4 w-4 shrink-0" />
                        <span>{item.label}</span>
                      </button>
                    );
                  })}
                </div>
              </div>
            ) : null}
          </div>
        </nav>

        {/* 底部固定区域 */}
        <div className="mt-4 flex flex-col items-center gap-3">
          {/* [+] 新建按钮 */}
          <button
            className="flex h-10 w-10 items-center justify-center rounded-full bg-white/40 text-slate-700 shadow-[0_8px_18px_rgba(102,112,145,0.10)] transition-all duration-200 hover:bg-white/60 hover:shadow-[0_12px_24px_rgba(102,112,145,0.14)]"
            title="新建"
          >
            <Plus className="h-4.5 w-4.5" />
          </button>

          {/* 用户头像 */}
          <button
            className="flex h-11 w-11 items-center justify-center rounded-full bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(231,228,223,0.94))] text-sm font-bold text-slate-800 shadow-[0_12px_24px_rgba(102,112,145,0.12)] transition-transform duration-200 hover:scale-105"
            title="设置"
          >
            AG
          </button>
        </div>
      </div>
    </aside>
  );
}
