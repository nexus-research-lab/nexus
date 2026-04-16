export interface RoomRouteParams extends Record<string, string | undefined> {
  room_id?: string;
  conversation_id?: string;
}

export interface ContactsRouteParams extends Record<string, string | undefined> {
  agent_id?: string;
}

export interface SkillsRouteParams extends Record<string, string | undefined> {
  skill_name?: string;
}

export interface RoomPageControllerOptions {
  room_id?: string | null;
  conversation_id?: string | null;
}
