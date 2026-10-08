# 受限 macOS 私有状态路径与 Full Access 语义

2026-09-28，macOS 27 arm64；Nexus 基线 b3e4e4ee8 加本批代码，Bridge 固定 c994b197，nxs 固定 2d1fd0d6，Claude Code 2.1.273。

## 当前产品行为

用户明确确认：Full Access 允许访问当前用户的本机文件，不提供沙箱隔离保证；不为该模式额外设计隔离身份。中英文选项提示同步说明该事实。后端残留安全检查及生命周期控制不等于隔离保证。

受限 macOS 会话从宿主 appfs.AppDir 生成整个 app 树的禁止写入规则，以及 data/config/cache/logs/rooms/processes/.migrations/.agents/sidecar.lock 的禁止读取规则；保留 platform-skills/host-skills 的只读投影。nxs 使用自身文件沙箱，Claude 命令使用原生 sandbox.filesystem，文件工具使用绝对 Read/Edit 规则。词法与物理路径同时保护；任务 ExtraEnv 不能替换宿主根。新私有路径须进入已保护目录或先扩展规则表。

## 验证

- options-final.log：clientopts 全包无缓存 race 通过，包括目录别名、相对宿主配置、已有私有目录符号链接目标及尾部空格、任务环境覆盖拒绝、Full Access 例外及不修改调用方切片。
- negotiation.log：固定真实 nxs 的桌面合同、host resource 初始化及关闭通过。该日志早于私有读范围与 Skill 兼容性调整，最终行为以 live-final.log 为准。
- live-final.log：使用主工作目录 .env 的 Provider 字段（仅 token/base URL/model），没有加载主数据库或其他配置。两个后端均经当前生产 builder 生成目录规则，测试不额外注入文件拒绝规则。验证工作区 Write/Bash、公开 Skill 投影 Read、私有原生 Write/Read 拒绝、私有 Bash 写入拒绝、明确拒绝网络目标，以及中断/关闭。
- negotiation-final.log：最终路径别名补充之后，固定 nxs 的两项真实握手/资源初始化测试再次通过。live-final.log 早于这次别名补充，证明常规布局；新增私有子目录别名与相对路径由 options-final.log 定向证明，不宣称已通过真实 Claude 特殊路径验收。
- architecture.log、vet.log、ui-lint.log：架构、目标 Go vet 和两份界面文案 ESLint 通过。

Go 均使用 GOWORK=off。普通包测试不能代替 native/live 必测名称；真实 Provider 调用由 scripts/desktop/check-live-sandbox.mjs 执行。日志 token 已由脚本及测试脱敏。

nxs SHA256: 6622c107941409c480a8fbb0a517edfde41f542cb450a365f0e37d047ebdd6aa

## 证据边界

本批证明上述工具执行路径，未证明全部 SDK IO、hook、IPC、外部服务、任意父目录移动或所有秘密/句柄边界。测试中断只检查自身受控 sleep fixture，不冒充脱离后代证明。没有启用 App 默认监督/自动恢复；没有真实 App UI、签名安装包、macOS 14.0、Intel、干净机器或 Windows 验收。
