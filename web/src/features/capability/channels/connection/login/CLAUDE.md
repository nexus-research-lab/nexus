# Channel Login

- `use-channel-login-controller.ts` 独占扫码会话、串行轮询和验证码提交生命周期。
- `channel-login-model.ts` 将登录协议快照统一投影为 idle/session 状态，并集中标签、视觉语义、身份、输出与验证码提示。
- `channel-login-panel.tsx` 按 Header、二维码、验证码和会话输出拆分窄视图，不解释原始状态字段。
- `login-qr-code.tsx` 为共享二维码生成器提供不暴露载荷的登录专用加载态与完整失败说明；生成失败不改变扫码会话或已保存配置。
- 扫码标题、说明、会话身份、验证码和进度只能组合共享 Panel、Typography、Badge、Form 与 Feedback 原语；登录视图不拥有私有字号、字重或任意圆角。
- 登录写命令复用连接控制器提供的命令入口，不建立第二把互斥锁。
- 启动登录只允许发生在保存配置事务内部，避免出现无配置的孤立登录会话。
- 登录轮询是只读阶段：失败时保留二维码与最后状态，只允许重新读取。登录已成功但 Channel 刷新失败必须明确“登录已提交”，不得重新扫码。
- 验证码提交未知时按 exact login session 读取核对，未证明前不重复提交；登录面板不展示 Provider error/output、command、验证码提示原文、未知协议状态、login ID 或二维码原始载荷。
- 启动扫码返回 unknown/accepted/committed 时不得再调用启动 API；主操作只通过 GET 读取当前 owner + channel 唯一 active、未绑定对话授权的 Web 扫码会话。读取不到或服务端发现多个候选/绑定不匹配都不能证明原 POST 未受理，必须保持旧意图锁定；用户显式解除后才能单独发起新扫码。只有 not_applied 证据允许原操作重试。
- 运行中会话缺二维码、本地二维码生成失败或收到未知状态都必须展示固定的问题、数据影响和恢复说明；这些展示异常不得触发配置保存或登录写命令。
