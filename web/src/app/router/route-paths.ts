export const APP_ROUTE_PATHS = {
  login: "/login",
  launcher: "/",
  home: "/app",
  room: "/rooms/:room_id",
  room_conversation: "/rooms/:room_id/conversations/:conversation_id",
  contacts: "/contacts",
  skills: "/capability/skills",
  skill_detail: "/capability/skills/:skill_name",
  connectors: "/capability/connectors",
  connectors_oauth_callback: "/capability/connectors/oauth/callback",
  scheduled_tasks: "/capability/scheduled-tasks",
  channels: "/capability/channels",
  pairings: "/capability/pairings",
  settings: "/settings",
} as const;

export const AppRouteBuilders = {
  login: () => APP_ROUTE_PATHS.login,
  launcher: () => APP_ROUTE_PATHS.launcher,
  home: () => APP_ROUTE_PATHS.home,
  room: (room_id: string) => `/rooms/${encodeURIComponent(room_id)}`,
  room_conversation: (room_id: string, conversation_id: string) =>
    `/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
  contacts: () => APP_ROUTE_PATHS.contacts,
  skills: () => APP_ROUTE_PATHS.skills,
  skill_detail: (skill_name: string) => `/capability/skills/${encodeURIComponent(skill_name)}`,
  connectors: () => APP_ROUTE_PATHS.connectors,
  connectors_oauth_callback: () => APP_ROUTE_PATHS.connectors_oauth_callback,
  scheduled_tasks: () => APP_ROUTE_PATHS.scheduled_tasks,
  channels: () => APP_ROUTE_PATHS.channels,
  pairings: () => APP_ROUTE_PATHS.pairings,
  settings: () => APP_ROUTE_PATHS.settings,
} as const;

export type AppRoutePathKey = keyof typeof APP_ROUTE_PATHS;
