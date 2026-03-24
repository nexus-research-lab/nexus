export const APP_ROUTE_PATHS = {
  launcher: "/",
  nexus: "/nexus",
  nexus_conversation: "/nexus/conversations/:conversation_id",
  room: "/rooms/:room_id",
  room_conversation: "/rooms/:room_id/conversations/:conversation_id",
  contacts: "/contacts",
  contact_profile: "/contacts/:agent_id",
} as const;

export const AppRouteBuilders = {
  launcher: () => APP_ROUTE_PATHS.launcher,
  nexus: () => APP_ROUTE_PATHS.nexus,
  nexus_conversation: (conversation_id: string) =>
    `/nexus/conversations/${encodeURIComponent(conversation_id)}`,
  room: (room_id: string) => `/rooms/${encodeURIComponent(room_id)}`,
  room_conversation: (room_id: string, conversation_id: string) =>
    `/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
  contacts: () => APP_ROUTE_PATHS.contacts,
  contact_profile: (agent_id: string) => `/contacts/${encodeURIComponent(agent_id)}`,
} as const;

export type AppRoutePathKey = keyof typeof APP_ROUTE_PATHS;
