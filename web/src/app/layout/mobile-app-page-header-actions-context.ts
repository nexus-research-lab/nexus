/**
 * INPUT: 移动端应用页头动作挂载点上下文。
 * OUTPUT: 供业务页面读取当前页头 Portal 目标的轻量 Hook。
 * POS: 移动页头动作桥接的共享上下文；不渲染界面。
 */

import { createContext, useContext } from "react";

export const MobileAppPageHeaderActionsContext =
  createContext<HTMLElement | null>(null);

export function useMobileAppPageHeaderActionsTarget() {
  return useContext(MobileAppPageHeaderActionsContext);
}
