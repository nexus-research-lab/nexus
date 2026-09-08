/**
 * INPUT: 宿主布局提供的可选页头动作挂载点。
 * OUTPUT: 供页面组合读取当前页头 Portal 目标的中立 Context 与 Hook。
 * POS: 跨层 React 插槽合同；不渲染 UI，不创建挂载点，也不决定视口或页面生命周期。
 */

import { createContext, useContext } from "react";

export const PageHeaderActionsContext =
  createContext<HTMLElement | null>(null);

export function usePageHeaderActionsTarget() {
  return useContext(PageHeaderActionsContext);
}
