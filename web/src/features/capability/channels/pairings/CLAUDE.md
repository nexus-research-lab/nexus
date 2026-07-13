# Pairings

- `pairing-model.ts` 负责筛选、统计、分组、展示文本和创建载荷。
- `use-pairings-controller.ts` 负责列表请求、筛选状态和写命令。
- 更新载荷直接使用 `UpdatePairingPayload`，禁止引入 `agentId` 等协议别名。
- 同一时间只执行一个配对写命令，重复点击由 ref 同步拦截。
