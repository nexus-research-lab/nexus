# hooks/agent/runtime/state/

L5 | 父级: ../CLAUDE.md

封装运行状态机订阅、slot/权限状态、Room execution 首见锚点和权限过期计时。权限成功提交后先把精确 execution 转为 acknowledged 非交互 tombstone；Session 清理必须同时清空它。状态机实例不得越过本目录，消费者只使用具体业务命令。

[PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
