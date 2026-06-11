export const APP_ROUTE_PATHS = {
  landing: "/",
  login: "/login",
  launcher: "/launcher",
  home: "/app",
  room: "/rooms/:room_id",
  room_session: "/rooms/:room_id/sessions/:session_key",
  room_conversation: "/rooms/:room_id/conversations/:conversation_id",
  contacts: "/contacts",
  skills: "/capability/skills",
  skill_detail: "/capability/skills/:skill_name",
  connectors: "/capability/connectors",
  connector_detail: "/capability/connectors/:connector_id",
  connectors_oauth_callback: "/capability/connectors/oauth/callback",
  scheduled_tasks: "/capability/scheduled-tasks",
  channels: "/capability/channels",
  pairings: "/capability/pairings",
  memory: "/memory",
  settings: "/settings",
} as const;

export const AppRouteBuilders = {
  landing: () => APP_ROUTE_PATHS.landing,
  login: () => APP_ROUTE_PATHS.login,
  launcher: () => APP_ROUTE_PATHS.launcher,
  home: () => APP_ROUTE_PATHS.home,
  room: (room_id: string) => `/rooms/${encodeURIComponent(room_id)}`,
  room_session: (room_id: string, session_key: string) =>
    `/rooms/${encodeURIComponent(room_id)}/sessions/${encodeURIComponent(session_key)}`,
  room_conversation: (room_id: string, conversation_id: string) =>
    `/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
  contacts: () => APP_ROUTE_PATHS.contacts,
  contact_agent: (agent_id: string) => `${APP_ROUTE_PATHS.contacts}?agent=${encodeURIComponent(agent_id)}`,
  skills: () => APP_ROUTE_PATHS.skills,
  skill_detail: (skill_name: string) => `/capability/skills/${encodeURIComponent(skill_name)}`,
  connectors: () => APP_ROUTE_PATHS.connectors,
  connector_detail: (connector_id: string) => `/capability/connectors/${encodeURIComponent(connector_id)}`,
  connectors_oauth_callback: () => APP_ROUTE_PATHS.connectors_oauth_callback,
  scheduled_tasks: () => APP_ROUTE_PATHS.scheduled_tasks,
  channels: () => APP_ROUTE_PATHS.channels,
  pairings: () => APP_ROUTE_PATHS.pairings,
  memory: () => APP_ROUTE_PATHS.memory,
  settings: () => APP_ROUTE_PATHS.settings,
} as const;

export type AppRoutePathKey = keyof typeof APP_ROUTE_PATHS;
