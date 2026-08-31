# hooks/agent/runtime/snapshot/

L5 | 父级: ../CLAUDE.md

负责活动会话的易失投影及 `sessionStorage` 读写。投影模型不得捕获 React 状态，存储失败不得改变后端会话语义。快照 key 同时绑定稳定 auth owner scope 与 Session key，并只允许创建它的 owner generation 读写；owner 切换先撤下旧 namespace、同步定向清除全部本模块键，再启用新 namespace 并发布 generation。即使浏览器拒绝清理，旧 key 对新 owner 也保持不可见，迟到的旧 Hook 不得恢复、覆盖或删除新 owner 的同名 Session 快照。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
