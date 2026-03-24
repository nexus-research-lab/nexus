export const APP_ROUTE_PATHS = {
  launcher: "/",
  nexus: "/nexus",
  nexusConversation: "/nexus/conversations/:conversationId",
  room: "/rooms/:roomId",
  roomConversation: "/rooms/:roomId/conversations/:conversationId",
  contacts: "/contacts",
  contactProfile: "/contacts/:agentId",
} as const;

export const AppRouteBuilders = {
  launcher: () => APP_ROUTE_PATHS.launcher,
  nexus: () => APP_ROUTE_PATHS.nexus,
  nexusConversation: (conversationId: string) =>
    `/nexus/conversations/${encodeURIComponent(conversationId)}`,
  room: (roomId: string) => `/rooms/${encodeURIComponent(roomId)}`,
  roomConversation: (roomId: string, conversationId: string) =>
    `/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}`,
  contacts: () => APP_ROUTE_PATHS.contacts,
  contactProfile: (agentId: string) => `/contacts/${encodeURIComponent(agentId)}`,
} as const;

export type AppRoutePathKey = keyof typeof APP_ROUTE_PATHS;
