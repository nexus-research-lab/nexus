# Channel Authorization Human UI

- 本目录只消费 `channel_authorization` 原生 WebSocket 事件；二维码、展示 token 与验证码不得进入聊天消息、MCP 参数、持久 Store 或离线队列。
- `channel-authorization-model.ts` 独占未知事件校验，视图只消费生成协议中的判别类型。
- Presenter 状态只在认证应用壳层内存中存活，不写 localStorage；新展示替换旧展示，flow 不匹配的 ACK 不得关闭当前弹窗。
- 验证码提交与取消只发送 `flow_id + presentation_token` 以及当前验证码；owner、Agent、business session、round 与 runtime lease 必须由服务端已认证 sender 路由恢复。
- 二维码原文不以文本展示；验证码提交后立即清空输入，连接不可用时业务消息必须丢弃且明确提示用户重试。
- 关闭弹窗只关闭本地视图；“取消授权”必须调用服务端控制动作并等待无敏感值 ACK。
- 未发送与服务端负 ACK 必须分开显示：本地 `not_sent` 可证明未送达；负 ACK 不足以证明异步授权未写入，必须锁定验证码提交与取消，只允许关闭后回频道页读取最新状态。失效展示只允许关闭后重新发起连接。
- WebSocket `result.message` 是服务端运行结果附带文本，不得直接进入用户界面；Problem/Impact/Recovery 只由本地受控文案投影。
- 授权弹窗使用 plain chrome；只保留平台 prompt、渠道、失效时间、二维码/验证码和一句会话边界提示，不展示盾牌图标、径向装饰、实现宣言或“安全提交”等自述性措辞。
