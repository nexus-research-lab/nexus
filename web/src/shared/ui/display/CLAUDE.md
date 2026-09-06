# 展示原语

- 本目录拥有头像、徽标、元数据、骨架屏和状态块等只读展示组件。
- `UiSkeleton` 是加载占位颜色、胶囊外形、脉冲动效和 reduced-motion 行为的唯一所有者；`strong/default/subtle` 只表达同一占位组内的信息层级，业务调用方只传布局宽高和必要的语义 shape，不得重写颜色或 `animate-pulse`。
- `UiSkeletonCardList` 只播报一次本地化加载状态，各占位卡作为装饰隐藏，数量不增加重复播报。
- `UiQRCode` 只把调用方已经选定的 payload 投影为二维码和可选原文，不解释登录或授权协议；动态生成按当前 payload 隔离迟到结果，生成失败及内嵌/生成图片解码失败均进入本地化反馈，新 payload 才重新开始。业务方可提供完整失败说明，`showPayload=false` 时不能显示原文。外层表面、圆角、状态文案与可选 payload 使用共享 Panel、shape 与 Typography，只有二维码纸张保留固定扫描尺寸。
- 业务状态必须先由消费者投影，共享组件不解析领域协议。
- `UiBadge` 默认使用共享紧凑圆角；只有数字聚合、版本等明确胶囊语义才传 `shape="pill"`。业务层不得再用 `rounded-*` 覆盖徽标外形。
- `UiCounterBadge` 的实底与文字使用同一危险语义色及主题配对前景；`UiBadge` 的 active/success 共用外观配方，但保留消费者传入的业务语义。信息徽标的颜色只由当前 Badge recipe 派生，不保留第二套无消费者的主题色表。
- `UiResourceState` 的 error 态最多提供一个安全恢复动作；需要“采用最新/覆盖草稿”这类双向业务选择时必须显式使用 decision 态，不得伪装成失败恢复。
- `UiStateBlock` 的标题、说明和图标承载体只能选择 App Typography 与语义 shape role；空态使用对象标题层级，紧凑错误/决策态使用分区标题层级，不在业务调用方重写字号和圆角。
- `spinner-styles.ts` 是圆形加载指示器尺寸、颜色、旋转与 reduced-motion 行为的唯一 recipe；业务容器负责 `role/status`、`aria-busy` 和可见文案，不得把装饰性 Spinner 自己暴露成第二个播报节点。生产 `web/src` 的页面、Feature、App 壳与共享组件均由合同测试禁止直接使用 `animate-spin` 或边框 Spinner；启动猫与 Composer 字符动效是明确的品牌/交互例外。
- `UiAgentAvatar` 与 `UiRoomAvatar` 的 `md` 是列表、工作区 Header 与完整消息共同使用的 40px 主身份基线；所有尺寸统一采用随尺寸缩放的 rounded-square 外轮廓，头像 API 不提供圆形变体。紧凑消息、成员堆叠、正文内联和导航图标可保留更小的语境尺寸，展示型 Profile 可保留更大尺寸。
- `UiSeededAvatar` 的尺寸只映射到共享 `radius-control-*` 档位，瞬时执行状态只通过 `state="running"` 使用主题级 running 外环；不得在业务层用 `rounded-[Npx]`、品牌色 ring 或 shadow 重建头像状态。目录与详情传同一稳定资源标识，保证视觉身份连续。
- `UiSeededAvatar` 是全部数学曲线资源头像的唯一渲染入口；外轮廓与 `UiAgentAvatar` 一样按尺寸使用 rounded-square，不提供圆形变体。静态 SVG 曲线由稳定标识散列出的居中曲线族、旋转阶数、细节强度和整体朝向共同决定，消费者必须传入稳定 ID，不得直接读取生成器、内联同类 SVG、使用运行时随机数或随语言变化的标题作为种子。正文内联身份使用 24px，能力目录卡使用 40px，弹窗标题使用 32px，详情身份使用 48px；只能接收图片地址的消息头像通过同一生成器导出静态 Data URL。
- `UiRoomAvatar` 的双成员组合使用两枚自然比例的 rounded-square 错位轻叠；不得把成员裁成半幅，也不得退化成圆形。
- `avatar.tsx` 内部 `AvatarContent` 共用图片加载失败回退；只有地址变化才重试，成员拼图直接使用内容层，不套普通头像再强制重写尺寸。Agent/Room 根节点各提供唯一可访问名称，Room 内部成员图片作为装饰。拼图限制、微字号与形状合同归 `design.md`。
- `running` 表示仍在执行中的瞬时信息状态，必须使用主题级低饱和蓝灰 token；`success` 只表示完成，`primary` 只表达品牌或主动作，三者不得混用。
