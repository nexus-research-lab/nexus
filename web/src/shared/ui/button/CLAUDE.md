# 按钮原语

- 本目录拥有按钮和图标按钮的交互结构及样式投影。
- 业务命令、权限判断和加载事务由消费者负责。
- `UiIconButton` 用显式 `tooltip`、`title` 或字符串 `aria-label` 驱动共享 Tooltip；原生 `title` 不再下发给按钮，避免两套悬浮提示叠加。
- Ghost 与 text 按钮默认透明并使用次级文字，hover 使用轻中性底，checked / expanded / pressed state 使用更明确的中性活动底；状态切换不得增加边框或改变几何。品牌色只归明确的 primary 动作，危险色只归 destructive 动作。
