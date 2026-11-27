/**
 * INPUT: 本机「侧栏天空」外观偏好（跟随时间 / 关闭 / 固定某一变体）。
 * OUTPUT: 持久化的偏好状态与跨窗口同步入口；天空变体的唯一类型定义。
 * POS: 侧栏天空偏好的唯一状态源；时间表与插画分别归 features/navigation/sidebar，本文件不读时钟也不渲染。
 */
import { create } from "zustand";
import { createJSONStorage, persist, type StateStorage } from "zustand/middleware";

import { readThemePreference, removeThemePreference, writeThemePreference } from "./theme-storage";

const STORAGE_KEY = "nexus-sidebar-sky";

export const SIDEBAR_SKY_VARIANTS = ["dawn", "day", "night", "meteor"] as const;
export type SidebarSkyVariant = (typeof SIDEBAR_SKY_VARIANTS)[number];

/** auto 按本地时间表切换；off 关闭天空并恢复普通品牌栏；其余值固定为该变体。 */
export type SidebarSkyPreference = "auto" | "off" | SidebarSkyVariant;
export const DEFAULT_SIDEBAR_SKY_PREFERENCE: SidebarSkyPreference = "auto";

const SIDEBAR_SKY_STORAGE: StateStorage = {
  getItem: readThemePreference,
  removeItem: removeThemePreference,
  setItem: writeThemePreference,
};

export function normalizeSidebarSkyPreference(value: unknown): SidebarSkyPreference {
  if (value === "auto" || value === "off") return value;
  // 已下线的变体（如旧版极光、星座）或损坏值统一回到跟随时间。
  return (SIDEBAR_SKY_VARIANTS as readonly unknown[]).includes(value) ? (value as SidebarSkyVariant) : DEFAULT_SIDEBAR_SKY_PREFERENCE;
}

export const useSidebarSkyPreference = create<{
  preference: SidebarSkyPreference;
  setPreference: (value: SidebarSkyPreference) => void;
}>()(persist((set) => ({
  preference: DEFAULT_SIDEBAR_SKY_PREFERENCE,
  setPreference: (value) => set({ preference: normalizeSidebarSkyPreference(value) }),
}), {
  name: STORAGE_KEY,
  storage: createJSONStorage(() => SIDEBAR_SKY_STORAGE),
  partialize: (state) => ({ preference: state.preference }),
  merge: (saved, current) => ({
    ...current,
    preference: normalizeSidebarSkyPreference((saved as { preference?: unknown } | null)?.preference),
  }),
}));

/** 独立设置窗口修改后，主窗口通过 storage 事件重读偏好。 */
export function syncSidebarSkyPreference(event: StorageEvent) {
  if (event.key === STORAGE_KEY || event.key === null) void useSidebarSkyPreference.persist.rehydrate();
}
