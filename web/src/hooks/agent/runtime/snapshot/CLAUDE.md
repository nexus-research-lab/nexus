# hooks/agent/runtime/snapshot/

L5 | 父级: ../CLAUDE.md

负责活动会话的易失投影及 `sessionStorage` 读写。投影模型不得捕获 React 状态，存储失败不得改变后端会话语义。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
