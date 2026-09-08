/**
 * INPUT: 宿主确认或明确用户命令已提交的命名 WorkGraph 目录变更。
 * OUTPUT: 能力计数、Composer picker 与首页目录的刷新信号。
 * POS: 多个 Feature 与会话 transport 共用的目录失效事件名称；不承载保存请求、刷新策略、意图识别或运行时命令。
 */

export const WORKGRAPH_WORKFLOWS_CHANGED_EVENT =
  "nexus:workgraph-workflows-changed";
