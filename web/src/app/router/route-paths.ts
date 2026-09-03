/**
 * INPUT: 产品页面的稳定资源身份，以及 Room 内部/外部 Conversation 身份。
 * OUTPUT: 全应用 canonical 路由模板与编码后的导航构造器。
 * POS: App 路由与跨 Feature 导航的路径真相源。
 */
import { getExternalSessionKeyFromConversationId } from "@/lib/conversation/external-session";

export const APP_ROUTE_PATHS = {
  landing: "/",
  root: "/",
  login: "/login",
  setup: "/setup",
  launcher: "/launcher",
  home: "/app",
  room: "/rooms/:roomId",
  roomSession: "/rooms/:roomId/sessions/:sessionKey",
  roomConversation: "/rooms/:roomId/conversations/:conversationId",
  contacts: "/contacts",
  capability: "/capability",
  skills: "/capability/skills",
  skillDetail: "/capability/skills/:skillName",
  connectors: "/capability/connectors",
  connectorDetail: "/capability/connectors/:connectorId",
  connectorsOauthCallback: "/capability/connectors/oauth/callback",
  loops: "/capability/loops",
  loopDetail: "/capability/loops/:slug",
  workGraphDistillations: "/capability/workgraphs",
  workGraphDistillationDetail: "/capability/workgraphs/:distillationId",
  scheduledTasks: "/capability/scheduled-tasks",
  channels: "/capability/channels",
  pairings: "/capability/pairings",
  operations: "/operations",
  adminMembers: "/admin/members",
  settings: "/settings",
} as const;

export const AppRouteBuilders = {
  landing: () => APP_ROUTE_PATHS.landing,
  root: () => APP_ROUTE_PATHS.root,
  login: () => APP_ROUTE_PATHS.login,
  setup: () => APP_ROUTE_PATHS.setup,
  launcher: () => APP_ROUTE_PATHS.launcher,
  home: () => APP_ROUTE_PATHS.home,
  room: (roomId: string) => `/rooms/${encodeURIComponent(roomId)}`,
  roomSession: (roomId: string, sessionKey: string) =>
    `/rooms/${encodeURIComponent(roomId)}/sessions/${encodeURIComponent(sessionKey)}`,
  roomConversation: (roomId: string, conversationId: string) =>
    `/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}`,
  conversation: (roomId: string, conversationId: string) => {
    const externalSessionKey = getExternalSessionKeyFromConversationId(conversationId);
    return externalSessionKey
      ? `/rooms/${encodeURIComponent(roomId)}/sessions/${encodeURIComponent(externalSessionKey)}`
      : `/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}`;
  },
  contacts: () => APP_ROUTE_PATHS.contacts,
  contactAgent: (agentId: string) => `${APP_ROUTE_PATHS.contacts}?agent=${encodeURIComponent(agentId)}`,
  contactsCreate: () => `${APP_ROUTE_PATHS.contacts}?view=create`,
  contactsManage: () => `${APP_ROUTE_PATHS.contacts}?view=manage`,
  capability: () => APP_ROUTE_PATHS.capability,
  skills: () => APP_ROUTE_PATHS.skills,
  skillDetail: (skillName: string) => `/capability/skills/${encodeURIComponent(skillName)}`,
  connectors: () => APP_ROUTE_PATHS.connectors,
  connectorDetail: (connectorId: string) => `/capability/connectors/${encodeURIComponent(connectorId)}`,
  connectorsOauthCallback: () => APP_ROUTE_PATHS.connectorsOauthCallback,
  loops: () => APP_ROUTE_PATHS.loops,
  loopDetail: (slug: string) => `/capability/loops/${encodeURIComponent(slug)}`,
  workGraphDistillations: () => APP_ROUTE_PATHS.workGraphDistillations,
  workGraphDistillationDetail: (distillationId: string) => `/capability/workgraphs/${encodeURIComponent(distillationId)}`,
  scheduledTasks: () => APP_ROUTE_PATHS.scheduledTasks,
  channels: () => APP_ROUTE_PATHS.channels,
  pairings: () => APP_ROUTE_PATHS.pairings,
  operations: () => APP_ROUTE_PATHS.operations,
  adminMembers: () => APP_ROUTE_PATHS.adminMembers,
  settings: (section?: string) =>
    section
      ? `${APP_ROUTE_PATHS.settings}?section=${encodeURIComponent(section)}`
      : APP_ROUTE_PATHS.settings,
} as const;
