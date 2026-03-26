export const APP_ROUTE_PATHS = {
  launcher: "/",
  dm_directory: "/dms",
  room_directory: "/rooms",
  room: "/rooms/:room_id",
  room_conversation: "/rooms/:room_id/conversations/:conversation_id",
  contacts: "/contacts",
  contact_profile: "/contacts/:agent_id",
} as const;

function createLauncherSearchParams(params: Record<string, string | null | undefined>) {
  const search_params = new URLSearchParams();

  Object.entries(params).forEach(([key, value]) => {
    if (!value) {
      return;
    }
    search_params.set(key, value);
  });

  const search_string = search_params.toString();
  return search_string ? `/?${search_string}` : APP_ROUTE_PATHS.launcher;
}

export const AppRouteBuilders = {
  launcher: () => APP_ROUTE_PATHS.launcher,
  launcher_app: (app_prompt?: string) =>
    createLauncherSearchParams({
      surface: "app",
      app_prompt: app_prompt?.trim() || undefined,
    }),
  dm_directory: () => APP_ROUTE_PATHS.dm_directory,
  room_directory: () => APP_ROUTE_PATHS.room_directory,
  room: (room_id: string) => `/rooms/${encodeURIComponent(room_id)}`,
  room_conversation: (room_id: string, conversation_id: string) =>
    `/rooms/${encodeURIComponent(room_id)}/conversations/${encodeURIComponent(conversation_id)}`,
  contacts: () => APP_ROUTE_PATHS.contacts,
  contact_profile: (agent_id: string) => `/contacts/${encodeURIComponent(agent_id)}`,
} as const;

export type AppRoutePathKey = keyof typeof APP_ROUTE_PATHS;
