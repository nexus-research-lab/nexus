// INPUT: 已认证账号/会话作用域与冻结的消息命令。
// OUTPUT: 发送前持久保存、恢复和按精确回执删除的发件箱。
// POS: 浏览器仅保存未确认意图；消息事实仍以 Relay 为准，不自动发送。

export interface TeamMessageIntent {
  id: string;
  text: string;
  agentIds: string[];
  membershipVersion: number;
}

export class TeamMessageOutbox {
  constructor(private readonly scope: string, private readonly storage: Storage = localStorage) {}

  save(intent: TeamMessageIntent): void {
    // 每条命令独立 key，多窗口不能覆盖另一窗口的未确认写入。
    const key = this.prefix() + intent.id;
    const encoded = JSON.stringify(intent);
    this.storage.setItem(key, encoded);
    if (this.storage.getItem(key) !== encoded) throw new Error("无法保存未确认消息");
  }

  read(): TeamMessageIntent[] {
    const intents: TeamMessageIntent[] = [];
    for (let index = 0; index < this.storage.length; index++) {
      const key = this.storage.key(index);
      if (!key?.startsWith(this.prefix())) continue;
      const intent: unknown = JSON.parse(this.storage.getItem(key) ?? "null");
      if (!isIntent(intent) || key !== this.prefix() + intent.id) throw new Error("未确认消息损坏，不能自动丢弃");
      intents.push(intent);
    }
    return intents;
  }

  confirm(id: string): void { this.storage.removeItem(this.prefix() + id); }

  private prefix(): string { return `nexus:team:outbox:v1:${this.scope}:`; }
}

function isIntent(value: unknown): value is TeamMessageIntent {
  if (!value || typeof value !== "object") return false;
  const intent = value as Partial<TeamMessageIntent>;
  return typeof intent.id === "string" && intent.id.length > 0 &&
    typeof intent.text === "string" && intent.text.trim().length > 0 &&
    Array.isArray(intent.agentIds) && intent.agentIds.every((id) => typeof id === "string") &&
    typeof intent.membershipVersion === "number" && Number.isSafeInteger(intent.membershipVersion) && intent.membershipVersion > 0;
}
