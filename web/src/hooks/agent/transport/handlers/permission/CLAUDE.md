# 权限事件

- `permission-event-data.ts` 校验未知事件载荷，并统一读取事件作用域、交互模式、风险等级与授权建议。
- `permission-event-handlers.ts` 只把已解码请求增删到当前 Session 状态。

协议别名和默认交互模式只在解码边界解释；处理器不得读取未知字段或复制字段回退规则。
