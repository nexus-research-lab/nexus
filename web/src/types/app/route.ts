// INPUT: 当前 Router 消费的 Room、Skill、Connector 路径与 Room 控制器参数。
// OUTPUT: 已接入路由的窄参数类型。
// POS: 前端路由身份契约；不保留未使用页面参数。

export interface RoomRouteParams extends Record<string, string | undefined> {
  roomId?: string;
  conversationId?: string;
  sessionKey?: string;
}

export interface SkillsRouteParams extends Record<string, string | undefined> {
  skillName?: string;
}

export interface ConnectorsRouteParams extends Record<string, string | undefined> {
  connectorId?: string;
}

export interface RoomPageControllerOptions {
  roomId?: string | null;
  conversationId?: string | null;
  preferredConversationIds?: readonly string[];
  sessionKey?: string | null;
}
