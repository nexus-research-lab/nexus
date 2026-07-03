export interface RoomRouteParams extends Record<string, string | undefined> {
  roomId?: string;
  conversationId?: string;
  sessionKey?: string;
}

export interface ContactsRouteParams extends Record<string, string | undefined> {
  agentId?: string;
}

export interface SkillsRouteParams extends Record<string, string | undefined> {
  skillName?: string;
}

export interface RoomPageControllerOptions {
  roomId?: string | null;
  conversationId?: string | null;
  sessionKey?: string | null;
}
