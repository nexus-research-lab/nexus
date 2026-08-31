# auth/

L3 | 父级: web/src/app

## 职责

- `auth-provider.tsx` 编排登录状态、登录/登出命令与用户作用域切换后的运行时配置刷新；generation 推进后必须在任何后续 await 前同步撤下旧 owner 页面，避免仍挂载表单用新代次提交旧 draft/ID。
- `auth-status-bootstrap.ts` 以 auth owner generation 对 `auth/status` 完整受理流程单飞；旧代次请求不能被新代次复用，旧 promise 的 finally 也不能清除新请求。
- `auth-owner-scope.ts` 在认证状态发布前先摘除并断开旧 owner 的共享 WebSocket，再清空无法证明属于当前 owner 的 Home/Agent 目录、会话元数据/易失正文、Room 导航、侧栏未读、workspace 文件/实时内容和 Composer 草稿/历史；易失正文和 Agent 创建 journal 的浏览器 key 额外绑定稳定 owner scope，新 owner 不得认领。创建 journal 是精确恢复依据，登出时只从内存摘除而不删除该 owner 的 pending 记录；同 owner 再登录必须先对账。同 owner 通过独立 marker 继续复用既有状态，登出、跨 owner 与跨标签页身份变化必须先清空内存再重新读取，旧 owner 的迟到目录/Agent/workspace 请求、临时消息和草稿恢复不得回写。
- `shared/auth/auth-owner-generation.ts` 只维护进程内 owner 代次；scope reset 在清空前推进代次，已挂载的 WebSocket、Room snapshot 与目录对账回调必须先通过该 fence，不能在 React cleanup 前回填旧数据；清理完成后发布新代次，使未卸载页面重新 acquire 一条新 owner 握手。
- 无状态 Context 与消费 Hook 留在 `shared/auth/auth-context.ts`，供 Feature 直接读取。

认证副作用属于应用装配层；`shared/` 不得持有运行时配置请求或用户切换事务。
owner generation 只丢弃旧回调，不进入持久化、网络协议、路由、缓存 key 或业务身份，也不得触发额外读取。
