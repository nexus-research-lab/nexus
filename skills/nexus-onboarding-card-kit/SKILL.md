---
name: nexus-onboarding-card-kit
description: Nexus 角色新手引导的原生交互卡片模板。仅在角色引导 Skill 收集 Agent、Room 信息或请求确认时使用。
---

# Nexus 新手引导卡片模板

这是角色引导的强制 UI 契约。加载任一 `nexus-onboarding-*` 角色 Skill 后，在第一次向用户收集信息之前加载本 Skill。

## 强制规则

1. 引导中的字段填写、选项选择和创建确认必须调用原生 `AskUserQuestion`，不得用普通文本、Markdown 列表或文本问句代替。
2. 普通文本只能陈述当前进度，不能包含问号、填写要求或“请告诉我”等变相提问。
3. 每轮只调用一个卡片模板。`AskUserQuestion` 必须是该轮最后一个动作；调用后等待用户在卡片中提交。
4. 用户已在自然语言中提供某个字段时直接记录，不再重复展示对应卡片。
5. 如果卡片超时或交互通道暂不可用，不要降级为文本问题。保留当前步骤，等用户下一次继续时重新展示同一模板。
6. 卡片标题必须使用下面定义的 `header` 原文。Nexus 前端通过这些标题启用预置的新手引导视觉，但不根据标题执行任何业务动作。
7. 每个问题保留 2–3 个真实可用选项。用户也可以直接在卡片的“自定义回答”输入区填写，不要添加“其他”或“自己填写”这种无实际值选项。

## 模板 A：Agent 名称

只收集 Agent 名称。候选项替换成与角色匹配的三个真实名称。

```json
{
  "questions": [
    {
      "header": "新手引导 · Agent 名称",
      "question": "你的 Agent 叫什么名字？",
      "multiSelect": false,
      "options": [
        {"label": "<角色候选名1>", "description": "<名称气质说明>"},
        {"label": "<角色候选名2>", "description": "<名称气质说明>"},
        {"label": "<角色候选名3>", "description": "<名称气质说明>"}
      ]
    }
  ]
}
```

## 模板 B：Agent 核心职责

选项标签必须能直接映射为职责描述；自定义回答可以是一句自然语言。

```json
{
  "questions": [
    {
      "header": "新手引导 · Agent 职责",
      "question": "这个 Agent 主要帮你完成哪类工作？",
      "multiSelect": false,
      "options": [
        {"label": "<职责方向1>", "description": "<目标用户、主要工作和产出>"},
        {"label": "<职责方向2>", "description": "<目标用户、主要工作和产出>"},
        {"label": "<职责方向3>", "description": "<目标用户、主要工作和产出>"}
      ]
    }
  ]
}
```

## 模板 C：工作风格

```json
{
  "questions": [
    {
      "header": "新手引导 · 工作风格",
      "question": "你希望它以什么方式与你协作？",
      "multiSelect": false,
      "options": [
        {"label": "<风格1>", "description": "<行为说明>"},
        {"label": "<风格2>", "description": "<行为说明>"},
        {"label": "<风格3>", "description": "<行为说明>"}
      ]
    }
  ]
}
```

## 模板 D：Agent 创建确认

问题正文必须汇总名称、职责、风格和当前模型。

```json
{
  "questions": [
    {
      "header": "新手引导 · 创建 Agent",
      "question": "<Agent 配置摘要>。确认创建吗？",
      "multiSelect": false,
      "options": [
        {"label": "确认创建", "description": "使用以上配置创建真实 Agent"},
        {"label": "修改名称", "description": "返回 Agent 名称卡片"},
        {"label": "修改职责或风格", "description": "返回对应配置卡片"}
      ]
    }
  ]
}
```

只有收到“确认创建”后才能调用真实创建工具。

## 模板 E：Room 任务主题

候选项必须是该角色可直接执行的真实场景，用户也可在自定义回答中描述自己的任务。

```json
{
  "questions": [
    {
      "header": "新手引导 · Room 任务",
      "question": "这次想让团队协作完成什么任务？",
      "multiSelect": false,
      "options": [
        {"label": "<场景1>", "description": "<输入与预期产出>"},
        {"label": "<场景2>", "description": "<输入与预期产出>"},
        {"label": "<场景3>", "description": "<输入与预期产出>"}
      ]
    }
  ]
}
```

## 模板 F：协作成员确认

```json
{
  "questions": [
    {
      "header": "新手引导 · 协作成员",
      "question": "请选择这次 Room 的协作配置。",
      "multiSelect": false,
      "options": [
        {"label": "三角色协作（推荐）", "description": "你的 Agent + 两位专业顾问"},
        {"label": "精简双角色", "description": "你的 Agent + 一位关键顾问"},
        {"label": "仅使用我的 Agent", "description": "先建立单角色 Room，之后再添加成员"}
      ]
    }
  ]
}
```

## 模板 G：Room 创建确认

问题正文必须汇总 Room 名称、任务主题、成员及分工。

```json
{
  "questions": [
    {
      "header": "新手引导 · 创建 Room",
      "question": "<Room 配置摘要>。确认创建吗？",
      "multiSelect": false,
      "options": [
        {"label": "确认创建", "description": "创建真实 Room 并生成可点击卡片"},
        {"label": "修改任务", "description": "返回 Room 任务卡片"},
        {"label": "修改成员", "description": "返回协作成员卡片"}
      ]
    }
  ]
}
```

只有收到“确认创建”后才能创建或补齐协作 Agent，并调用真实 Room 创建工具。

## 模板 H：Room 创建完成卡片

这不是 `AskUserQuestion` 卡片，也不由模型生成 UI。它是 Nexus 消息层预先定义的 `nexus_resource_artifact` Room 卡片，固定展示：

- “ROOM 已创建”状态。
- Room 头像、名称、任务描述。
- 参与协作的 Agent 数量与名称。
- “进入 Room”按钮；点击后必须进入该 Room 的主会话。若创建结果携带首次主持消息，进入后必须自动发布该消息并启动真实 Agent 协作。

执行约束：

1. 用户在模板 G 选择“确认创建”后，必须以 `nexusctl --json room create` 完成真实创建。
2. 成功命令返回的 `domain=room`、`action=create`、`item.room.id` 和 `item.conversation.id` 是模板 H 的唯一数据源；不得自己拼 JSON、Markdown 链接或文本假卡片。
3. `room create` 成功后，不要再调用 `room get`、`room list` 等命令来“补卡片”，也不要仅用文字声称“点击卡片”。系统会直接把本次成功结果投影成模板 H。
4. 若成功结果中缺少 Room ID 或主会话 ID，视为创建链路未完整收口；如实说明错误并保留当前步骤，不得宣布 Room 任务完成。
5. 只有用户点击模板 H 进入 Room 后，新手引导才真正结束。
6. 引导 Room 必须由 `room create` 的 `--initial-message`、`--initial-target-agent-id` 与主持人参数提供首次进入动作；卡片不得在前端猜测开场内容。首次开场可使用预置脚本，后续 Agent 输出必须来自 Room 的真实模型服务。
