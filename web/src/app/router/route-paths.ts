export const APP_ROUTE_PATHS = {
  login: "/login",
  launcher: "/",
  home: "/app",
  dm_directory: "/dms",
  room: "/rooms/:room_id",
  room_conversation: "/rooms/:room_id/conversations/:conversation_id",
  contacts: "/contacts",
  contact_profile: "/contacts/:agent_id",
  skills: "/capability/skills",
  skill_detail: "/capability/skills/:skill_name",
  connectors: "/capability/connectors",
  scheduled_tasks: "/capability/scheduled-tasks",
  channels: "/capability/channels",
  pairings: "/capability/pairings",
  files: "/files",
  settings: "/settings",
} as const;

export const AppRouteBuilders = {
  login: () => APP_ROUTE_PATHS.login,
  launcher: () => APP_ROUTE_PATHS.launcher,
  home: () => APP_ROUTE_PATHS.home,
  dm_directory: () => APP_ROUTE_PATHS.dm_directory,
  room: (room_id: string) => `/rooms/${encodeURIComponent(room_id)}`,
  room_conversation: (room_id: string, conversation_id: string) =>
    `/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
  contacts: () => APP_ROUTE_PATHS.contacts,
  contact_profile: (agent_id: string) => `/contacts/${encodeURIComponent(agent_id)}`,
  skills: () => APP_ROUTE_PATHS.skills,
  skill_detail: (skill_name: string) => `/capability/skills/${encodeURIComponent(skill_name)}`,
  connectors: () => APP_ROUTE_PATHS.connectors,
  scheduled_tasks: () => APP_ROUTE_PATHS.scheduled_tasks,
  channels: () => APP_ROUTE_PATHS.channels,
  pairings: () => APP_ROUTE_PATHS.pairings,
  files: () => APP_ROUTE_PATHS.files,
  settings: () => APP_ROUTE_PATHS.settings,
} as const;

export type AppRoutePathKey = keyof typeof APP_ROUTE_PATHS;
