/**
 * INPUT: 宿主确认已保存的命名 WorkGraph 目录变更。
 * OUTPUT: 能力计数、Composer picker 与首页目录的刷新信号。
 * POS: 后台 Skill + CLI 保存完成后的前端目录失效事件；不承载保存请求。
 */

export const WORKGRAPH_WORKFLOWS_CHANGED_EVENT =
  "nexus:workgraph-workflows-changed";
