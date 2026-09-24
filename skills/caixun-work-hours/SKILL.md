---
name: caixun-work-hours
title: 彩讯工时填报
version: 1.0.2
description: 彩讯（RichPMS）工时填报助手：把当前会话、代码记录或用户口述整理成日报，按 8 小时分配，经用户确认后写入工时系统。用户提到“填日报、报工、补报、本周工时、日报被驳回”时使用。
scope: any
tags: [彩讯, 工时, 日报, RichPMS, 报工]
category_key: data-automation
category_name: 数据与自动化
recommendation: 适合需要在彩讯 RichPMS 中整理、核对并提交本人日报工时的场景。
---

# 彩讯工时填报

这是一个随 Nexus 发布、可直接从技能选择器启用的彩讯工时模板，来源于 SkillHub 的
`@user_ab3d2b8a/richpm-daily-report`（version 1.0.2）。首次使用只需接入一次
`richpm` MCP；没有接入时，按“首次接入”步骤配置，不会凭空提交工时。

你通过 MCP 服务器 `richpm` 的工具操作 RichPMS 工时系统（工具：get_report_calendar、get_fillable_projects_tasks、get_day_detail、check_can_submit、save_daily_report_draft、submit_daily_report）。
在 Nexus 中优先从左侧“连接器”或智能体“工具”页接入并完成授权；若当前版本没有该入口，
再提示用户按所用外部客户端接入 MCP 服务器 `<后端地址>/api/ai/mcp`：
- Claude Code：`claude mcp add richpm <后端地址>/api/ai/mcp --transport http`
- WorkBuddy：设置 → MCP 插件 → 添加 HTTP 服务器，地址同上（鉴权方式以客户端支持为准）
- Codex：`~/.codex/config.toml` 的 `[mcp_servers.richpm]` 配置 url
接入后按管理员指引完成授权（PMS 账号），工具名前缀以客户端实际注入为准（如 mcp__richpm__get_report_calendar）。

## 首次接入

用户说“配置 richpm / 接入日报”，或首次使用且工具不可用时：

1. Nexus 内先引导用户到“连接器”或智能体“工具”页添加 `richpm`，填写管理员提供的服务地址并完成授权。
   Skill 不自行修改 Nexus 配置、凭据或其他客户端的配置文件。
2. 若用户明确选择外部客户端，再由用户主动运行 `scripts/setup_mcp.py` 完成兼容配置；该脚本会修改外部客户端配置，
   运行前应说明影响并确认目标客户端。
3. 授权（均弹浏览器输 PMS 账密）：WorkBuddy 跑 `scripts/ai-workbuddy-auth.py`、ZCode 跑
   `scripts/richpm_zcode_auth.py`；Claude Code 按客户端的 MCP/OAuth 登录流程完成授权；Codex（CLI）
   运行 `codex mcp login richpm`（自动动态注册后弹浏览器；桌面版对 http 内网授权服务器有安全限制，
   需 HTTPS/SSH 隧道，详见对接指导）。
4. 完成后提示重启外部客户端/开新会话，说“查我的日报”验证。

## 外部客户端兜底接入（Codex stdio bridge）

仅在用户明确选择 Codex 外部客户端且原生登录异常时，使用技能包内置的 stdio 桥接（Mac/Windows 通用）。
这条路径会由用户显式修改 Codex 配置，不是 Nexus 内置接入路径：

1. **一次授权**（浏览器输 PMS 账密，凭据缓存在 `~/.codex/richpm/oauth.json` 自动刷新）：
   - Mac/Linux：`python3 <技能包目录>/scripts/richpm_codex_mcp_client.py --login`
   - Windows：`py <技能包目录>\scripts\richpm_codex_mcp_client.py --login`
2. **config.toml 改 command 型 MCP**（替换原 url 型 richpm 节点）：
   ```toml
   [mcp_servers.richpm]
   command = "python3"          # Windows 用 "py"
   args = ["<技能包目录>/scripts/richpm_codex_mcp_client.py", "--stdio"]
   ```
3. 重启 Codex 生效；令牌过期自动刷新（401 自动重试一次），失效时重跑第 1 步。

## 令牌失效自愈（WorkBuddy 专属，2026-09-20）

调用 MCP 工具返回 401 / "令牌无效"时，**在会话内按两级处理，用户无需离开 WorkBuddy**：

1. **免密续期（优先，先试这个）**：运行技能包内 `scripts/ai-workbuddy-refresh.sh`
   （GitBash：`bash <技能包目录>/scripts/ai-workbuddy-refresh.sh`，环境变量 `RICHPM_AI_BASE` 可覆盖后端地址，
   默认从 mcp.json 的 richpm.url 自动推导）。
   成功输出"令牌已续期并回写"→ 提示用户**开新会话**重试即可。
2. **浏览器重新授权（refresh 失效时，退出码 2）**：运行 `python <技能包目录>/scripts/ai-workbuddy-auth.py`
   ——会**自动弹出系统浏览器**到 RichPMS 授权页，请告诉用户"在弹出的页面输入 PMS 账号密码并确认"；
   页面显示"RichPMS 授权成功"后脚本自动完成令牌回写。双击 `scripts/ai-workbuddy-auth.py` 同效（装有 Python 的机器）。
3. 环境缺失（无 python / 无 GitBash）时如实告知用户，勿反复重试。

授权成功后必须提示：**在 WorkBuddy 开新会话**（MCP 连接器才会重载 mcp.json 里的新令牌）。

## 强制流程（不可跳步）

1. **定位未填日**：调 `get_report_calendar`（yearMonth=当月）。跳过休息日(isWeekend/isHoliday，除非 isMakeupWorkday 调休)、已填(status=filled)、待审(status 有 SUBMITTED)、未雇佣日；只处理未填/驳回日。向用户报告你打算处理哪些天。
2. **明确可报范围**：调 `get_fillable_projects_tasks`。返回列表即当前全部可填报的项目与任务（已过滤不可报项），projectCode/taskId 必须取自该列表，禁止编造；若列表为空，如实告知用户无可报项目。
3. **汇总工作内容**：从上下文（git log、会话记录、用户口述）提炼每天做了什么。信息不足时**询问用户**，不要虚构。
   内容必须是可读的中文/英文文本；**禁止出现连续问号（???）等乱码**，系统会拒绝乱码内容入库。
   **含中文的脚本/参数编码规则**（Windows 实证教训）：经 PowerShell 管道（`$code | py -`）或
   C-locale shell 传原始中文会被控制台编码替换成问号——**中文一律用 Unicode 转义（`\uXXXX`）写入
   或 JSON 用 `ensure_ascii=True` 传输**，或将内容写入 UTF-8 临时文件后引用，勿让原始中文经过控制台管道。
4. **8 小时分配**：每个工作日的各行 hours 合计必须 = 8（0.5h 粒度）。多项目按实际投入拆分；纯请假日走 PMS 页面办理，AI 不代填请假。
5. **展示方案并等待确认（硬性）**：以表格展示每日方案（日期/项目/任务/工时/内容摘要），明确询问用户是否确认。用户未明确同意前，禁止调用任何写工具。多日补报时逐日或整体确认均可，但确认必须显式。
6. **落草稿**：逐日调 `save_daily_report_draft`（workDate + rows[]）。
7. **预检**：调 `check_can_submit`，不可提交时把 reason 告知用户并尝试修正（如 8h 不齐、预警阻断）。
8. **提交**：调 `submit_daily_report`。回报结果：已进入三级审批（项目经理→部门总监→VP）。

## 驳回重报

用户说"日报被驳回"时：先调 `get_day_detail`（workDate）读取 rejectComment 与审批链，向用户展示驳回原因，修改 workContent 后重新走第 5-8 步。

## 错误自纠错

校验失败会返回结构化错误（详见 references/validation-rules.md）：code 1005=工时合计不等于8、1007=必填缺失、1009=预警阻断、1010=通道关闭。按提示修正参数后可重试一次，仍失败则如实告知用户，禁止反复盲试。

## 边界

- 禁止为他人填报（系统强制 token 本人身份，尝试会被拒绝）
- 禁止未确认提交；禁止虚构工作内容
- 只处理本人当日/历史未填日，未来日期受 30 天窗口限制（code 见 references）
