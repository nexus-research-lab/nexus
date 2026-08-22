/**
 * INPUT: 应用壳层提供的移动端页头动作挂载点与子页面内容。
 * OUTPUT: 向当前二级页面暴露可选的页头动作 Portal 目标。
 * POS: 移动端应用页头与业务页面之间的布局桥接；不持有具体动作或业务状态。
 */
"use client";

import {
  type ReactNode,
} from "react";

import { MobileAppPageHeaderActionsContext } from "./mobile-app-page-header-actions-context";

export function MobileAppPageHeaderActionsProvider({
  children,
  target,
}: {
  children: ReactNode;
  target: HTMLElement | null;
}) {
  return (
    <MobileAppPageHeaderActionsContext.Provider value={target}>
      {children}
    </MobileAppPageHeaderActionsContext.Provider>
  );
}
