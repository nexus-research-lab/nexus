package connectors

// CatalogEntry 表示一条连接器目录记录。
type CatalogEntry struct {
	ConnectorID     string
	Name            string
	Title           string
	Description     string
	Icon            string
	Category        string
	AuthType        string
	Status          string
	Provider        string
	RequiresExtra   []string
	AuthURL         string
	TokenURL        string
	APIBaseURL      string
	Scopes          []string
	MCPServerURL    string
	DocsURL         string
	Features        []string
	UserOAuthClient bool
}

// FeatureDetail 表示连接器内单个能力的说明。
type FeatureDetail struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Items       []string `json:"items,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

var categoryLabels = map[string]string{
	"productivity": "效率办公",
	"social":       "社交媒体",
	"ecommerce":    "电商零售",
	"development":  "研发协作",
	"business":     "商业运营",
	"marketing":    "营销增长",
}

var connectorCatalog = []CatalogEntry{
	{
		ConnectorID: "gmail",
		Name:        "gmail",
		Title:       "Gmail 邮箱",
		Description: "完整管理邮件、标签与基础设置",
		Icon:        "gmail",
		Category:    "productivity",
		AuthType:    "oauth2",
		Status:      "coming_soon",
		Provider:    "gmail",
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		APIBaseURL:  "https://gmail.googleapis.com",
		Scopes:      []string{"https://www.googleapis.com/auth/gmail.modify"},
		DocsURL:     "https://developers.google.com/gmail/api",
		Features:    []string{"读取邮件", "发送邮件", "管理标签", "搜索邮件"},
	},
	{
		ConnectorID: "feishu-docx",
		Name:        "feishu-docx",
		Title:       "飞书云文档",
		Description: "阅读、搜索、创建和更新飞书云文档，并查看云空间、知识库、Sheet 与 Bitable 内容",
		Icon:        "feishu-docx",
		Category:    "productivity",
		AuthType:    "oauth2",
		Status:      "available",
		Provider:    "feishu-docx",
		AuthURL:     "https://accounts.feishu.cn/open-apis/authen/v1/authorize",
		TokenURL:    "https://open.feishu.cn/open-apis/authen/v2/oauth/token",
		APIBaseURL:  "https://open.feishu.cn",
		Scopes: []string{
			"docx:document",
			"docx:document.block:convert",
			"drive:drive",
			"wiki:wiki",
			"sheets:spreadsheet",
			"bitable:app",
			"search:docs:read",
			"offline_access",
		},
		DocsURL:         "https://open.feishu.cn/document/develop-an-echo-bot/introduction",
		Features:        []string{"阅读文档", "全文搜索", "Sheet 内容", "Bitable 内容", "创建文档", "更新 Block", "云空间列表", "知识库浏览"},
		UserOAuthClient: true,
	},
	{
		ConnectorID: "x-twitter",
		Name:        "x-twitter",
		Title:       "X（Twitter）",
		Description: "读取推文、搜索对话并追踪趋势",
		Icon:        "x-twitter",
		Category:    "social",
		AuthType:    "oauth2",
		Status:      "coming_soon",
		Provider:    "x-twitter",
		AuthURL:     "https://twitter.com/i/oauth2/authorize",
		TokenURL:    "https://api.twitter.com/2/oauth2/token",
		APIBaseURL:  "https://api.twitter.com",
		Scopes:      []string{"tweet.read", "users.read", "offline.access"},
		DocsURL:     "https://developer.twitter.com/en/docs",
		Features:    []string{"读取推文", "发布推文", "搜索话题", "追踪趋势"},
	},
	{
		ConnectorID: "linkedin",
		Name:        "linkedin",
		Title:       "LinkedIn",
		Description: "访问个人档案、公司数据和职位浏览",
		Icon:        "linkedin",
		Category:    "social",
		AuthType:    "oauth2",
		Status:      "coming_soon",
		Provider:    "linkedin",
		AuthURL:     "https://www.linkedin.com/oauth/v2/authorization",
		TokenURL:    "https://www.linkedin.com/oauth/v2/accessToken",
		APIBaseURL:  "https://api.linkedin.com",
		Scopes:      []string{"openid", "profile", "email"},
		DocsURL:     "https://learn.microsoft.com/en-us/linkedin/",
		Features:    []string{"个人档案", "公司搜索", "职位浏览", "人脉管理"},
	},
	{
		ConnectorID:   "shopify",
		Name:          "shopify",
		Title:         "Shopify",
		Description:   "访问商品、订单与客户数据",
		Icon:          "shopify",
		Category:      "ecommerce",
		AuthType:      "oauth2",
		Status:        "coming_soon",
		Provider:      "shopify",
		RequiresExtra: []string{"shop"},
		AuthURL:       "https://{shop}.myshopify.com/admin/oauth/authorize",
		TokenURL:      "https://{shop}.myshopify.com/admin/oauth/access_token",
		APIBaseURL:    "https://{shop}.myshopify.com/admin/api/2024-07",
		Scopes:        []string{"read_products", "read_orders", "read_customers"},
		DocsURL:       "https://shopify.dev/docs/api",
		Features:      []string{"商品管理", "订单查看", "客户数据", "库存同步"},
	},
	{
		ConnectorID: "instagram",
		Name:        "instagram",
		Title:       "Instagram",
		Description: "访问账号、媒体和互动数据",
		Icon:        "instagram",
		Category:    "social",
		AuthType:    "oauth2",
		Status:      "coming_soon",
		Provider:    "instagram",
		AuthURL:     "https://www.instagram.com/oauth/authorize",
		TokenURL:    "https://api.instagram.com/oauth/access_token",
		APIBaseURL:  "https://graph.instagram.com",
		Scopes:      []string{"instagram_business_basic"},
		DocsURL:     "https://developers.facebook.com/docs/instagram-api",
		Features:    []string{"媒体发布", "互动数据", "粉丝分析", "评论管理"},
	},
	{
		ConnectorID: "github",
		Name:        "github",
		Title:       "GitHub",
		Description: "管理仓库、协作开发并跟踪问题",
		Icon:        "github",
		Category:    "development",
		AuthType:    "oauth2",
		Status:      "available",
		Provider:    "github",
		AuthURL:     "https://github.com/login/oauth/authorize",
		TokenURL:    "https://github.com/login/oauth/access_token",
		APIBaseURL:  "https://api.github.com",
		Scopes:      []string{"repo", "read:user", "user:email"},
		DocsURL:     "https://docs.github.com/en/rest",
		Features:    []string{"仓库管理", "Issue 跟踪", "PR 审查", "代码搜索"},
	},
	{ConnectorID: "google-calendar", Name: "google-calendar", Title: "Google 日历", Description: "读取日程并创建、修改事件", Icon: "google-calendar", Category: "productivity", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "google-drive", Name: "google-drive", Title: "Google 云盘", Description: "读取、检索并管理云盘文件", Icon: "google-drive", Category: "productivity", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "youtube", Name: "youtube", Title: "YouTube", Description: "读取、上传并管理频道与视频内容", Icon: "youtube", Category: "social", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "reddit", Name: "reddit", Title: "Reddit", Description: "搜索帖子、阅读讨论并监控社区", Icon: "reddit", Category: "social", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "tiktok", Name: "tiktok", Title: "TikTok", Description: "访问视频数据、热门内容和创作者洞察", Icon: "tiktok", Category: "social", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "odoo-erp", Name: "odoo-erp", Title: "Odoo ERP", Description: "访问 Odoo 模块（CRM、库存、财务等）", Icon: "odoo", Category: "business", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "square-pos", Name: "square-pos", Title: "Square POS", Description: "连接 Square POS，访问商户、目录与订单数据", Icon: "square", Category: "ecommerce", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "alibaba", Name: "alibaba", Title: "Alibaba.com", Description: "搜索供应商、商品与贸易数据", Icon: "alibaba", Category: "ecommerce", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "outlook", Name: "outlook", Title: "Outlook", Description: "高效管理邮件与日程", Icon: "outlook", Category: "productivity", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "airtable", Name: "airtable", Title: "Airtable", Description: "以协作方式组织与管理数据", Icon: "airtable", Category: "productivity", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "meta-ads", Name: "meta-ads", Title: "Meta Ads Manager", Description: "自动化广告洞察与优化，提升 ROI", Icon: "meta", Category: "marketing", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "ahrefs", Name: "ahrefs", Title: "Ahrefs", Description: "优化 SEO 策略并分析关键词", Icon: "ahrefs", Category: "marketing", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "similarweb", Name: "similarweb", Title: "Similarweb", Description: "分析网站流量与竞品情报", Icon: "similarweb", Category: "marketing", AuthType: "oauth2", Status: "coming_soon"},
	{ConnectorID: "dropbox", Name: "dropbox", Title: "Dropbox", Description: "管理文件、文件夹和共享权限", Icon: "dropbox", Category: "productivity", AuthType: "oauth2", Status: "coming_soon"},
}

var connectorFeatureDetails = map[string]map[string]FeatureDetail{
	"gmail": {
		"读取邮件": {Name: "读取邮件", Description: "读取用户邮箱中的邮件正文、发件人、收件人、主题和时间，用于在对话中检索上下文。", Items: []string{"按关键词或条件定位邮件", "读取邮件正文与基础元数据", "支持把邮件内容交给 Agent 总结或引用"}, Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		"发送邮件": {Name: "发送邮件", Description: "在用户确认后创建并发送邮件，适合回复客户、同步进度或发送结构化通知。", Items: []string{"生成草稿或直接发送", "支持收件人、抄送和主题", "发送前可由 Agent 生成正文"}, Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		"管理标签": {Name: "管理标签", Description: "读取和更新 Gmail 标签，用于整理邮件、标记待办或按业务流程归档。", Items: []string{"列出邮箱标签", "给邮件添加或移除标签", "配合搜索结果批量整理"}, Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		"搜索邮件": {Name: "搜索邮件", Description: "按 Gmail 查询语法检索邮件，让 Agent 能快速找到指定主题、联系人或时间范围内的邮件。", Items: []string{"支持关键词、发件人和时间范围", "返回匹配邮件摘要", "可继续读取选中邮件详情"}, Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
	},
	"feishu-docx": {
		"阅读文档":       {Name: "阅读文档", Description: "读取飞书 Docx 或 Wiki 文档，并转换为对 Agent 友好的 Markdown 内容。", Items: []string{"支持 docx 链接、wiki 链接或文档 ID", "保留标题、段落、列表、表格和块 ID 标记", "适合让 Agent 总结、改写或基于文档继续编辑"}, Scopes: []string{"docx:document", "wiki:wiki"}},
		"全文搜索":       {Name: "全文搜索", Description: "调用飞书搜索能力，在用户有权限访问的云文档、知识库和工作区内容中查找资料。", Items: []string{"按关键词搜索文档", "返回标题、类型、链接和摘要", "可继续读取搜索结果对应文档"}, Scopes: []string{"search:docs:read"}},
		"Sheet 内容":   {Name: "Sheet 内容", Description: "读取飞书多维表格以外的普通 Sheet 数据，便于 Agent 分析表格、生成摘要或定位单元格内容。", Items: []string{"解析 sheets 链接和 sheet ID", "读取指定工作表范围", "返回结构化行列数据"}, Scopes: []string{"sheets:spreadsheet"}},
		"Bitable 内容": {Name: "Bitable 内容", Description: "读取飞书 Bitable 表结构和记录，让 Agent 能查看业务数据库中的字段与记录内容。", Items: []string{"解析 base/table 链接", "列出表字段和记录", "支持查看记录值和基础元数据"}, Scopes: []string{"bitable:app"}},
		"创建文档":       {Name: "创建文档", Description: "在飞书云空间中创建新的 Docx 文档，用于沉淀会议纪要、方案草稿或执行结果。", Items: []string{"指定标题创建文档", "返回新文档链接和 document ID", "可继续追加或更新块内容"}, Scopes: []string{"docx:document", "drive:drive"}},
		"更新 Block":   {Name: "更新 Block", Description: "按飞书 Docx block ID 更新或追加文档块，适合对已有文档做精确编辑。", Items: []string{"支持基于 block ID 定位内容", "可追加后代块", "可更新文本块内容"}, Scopes: []string{"docx:document", "docx:document.block:convert"}},
		"云空间列表":      {Name: "云空间列表", Description: "列出用户可访问的飞书云空间文件，帮助 Agent 找到最近文档或指定目录内容。", Items: []string{"浏览云空间文件列表", "返回文件类型、名称和 token", "可配合文档读取继续处理"}, Scopes: []string{"drive:drive"}},
		"知识库浏览":      {Name: "知识库浏览", Description: "浏览飞书知识库空间和节点树，并把 Wiki 节点解析到真实云文档。", Items: []string{"列出可访问知识库", "分页浏览节点树", "解析 wiki 节点对应的 docx 文档"}, Scopes: []string{"wiki:wiki", "drive:drive"}},
	},
	"x-twitter": {
		"读取推文": {Name: "读取推文", Description: "读取账号或指定链接的推文内容与基础互动信息。", Items: []string{"获取推文正文和作者", "查看发布时间与互动数据", "用于舆情摘要或内容复盘"}, Scopes: []string{"tweet.read", "users.read"}},
		"发布推文": {Name: "发布推文", Description: "根据对话内容生成并发布推文，适合公告、活动或内容分发场景。", Items: []string{"生成发布文案", "支持发布前人工确认", "可结合趋势或历史内容优化表达"}, Scopes: []string{"tweet.write"}},
		"搜索话题": {Name: "搜索话题", Description: "按关键词搜索公开推文和话题，帮助 Agent 捕捉讨论上下文。", Items: []string{"按关键词检索推文", "返回相关作者和摘要", "用于话题研究和竞品观察"}, Scopes: []string{"tweet.read"}},
		"追踪趋势": {Name: "追踪趋势", Description: "跟踪热门话题和趋势变化，用于内容选题或市场观察。", Items: []string{"查看趋势关键词", "整理热点变化", "输出趋势摘要"}, Scopes: []string{"tweet.read", "users.read"}},
	},
	"linkedin": {
		"个人档案": {Name: "个人档案", Description: "读取 LinkedIn 个人基础档案，用于识别联系人背景和沟通上下文。", Items: []string{"读取姓名、职位和头像", "查看公开资料摘要", "辅助生成沟通材料"}, Scopes: []string{"openid", "profile", "email"}},
		"公司搜索": {Name: "公司搜索", Description: "搜索公司资料，帮助 Agent 获取组织背景、行业和公开信息。", Items: []string{"按名称查找公司", "返回公司基础资料", "用于客户或合作方研究"}, Scopes: []string{"profile"}},
		"职位浏览": {Name: "职位浏览", Description: "浏览职位相关信息，用于招聘分析、岗位研究或候选人沟通准备。", Items: []string{"检索职位条目", "查看职位描述摘要", "提取岗位要求"}, Scopes: []string{"profile"}},
		"人脉管理": {Name: "人脉管理", Description: "围绕联系人关系整理沟通对象和后续跟进线索。", Items: []string{"查看联系人基础资料", "整理沟通对象列表", "辅助生成跟进建议"}, Scopes: []string{"profile"}},
	},
	"shopify": {
		"商品管理": {Name: "商品管理", Description: "读取 Shopify 商品目录和库存相关字段，用于商品维护和经营分析。", Items: []string{"查看商品标题、价格和状态", "读取变体与库存信息", "辅助生成商品优化建议"}, Scopes: []string{"read_products"}},
		"订单查看": {Name: "订单查看", Description: "读取订单数据，帮助 Agent 汇总销售、履约和异常订单情况。", Items: []string{"查询订单列表", "查看订单明细和金额", "整理履约或退款线索"}, Scopes: []string{"read_orders"}},
		"客户数据": {Name: "客户数据", Description: "读取客户资料和订单关联信息，用于客服和复购分析。", Items: []string{"查看客户基础信息", "关联客户订单", "辅助客户分层和跟进"}, Scopes: []string{"read_customers"}},
		"库存同步": {Name: "库存同步", Description: "读取库存状态并为补货、下架或渠道同步提供依据。", Items: []string{"查看库存数量", "识别低库存商品", "生成库存处理建议"}, Scopes: []string{"read_products"}},
	},
	"instagram": {
		"媒体发布": {Name: "媒体发布", Description: "准备并发布 Instagram 媒体内容，用于品牌账号内容分发。", Items: []string{"生成图文说明", "发布前确认内容", "记录发布结果"}, Scopes: []string{"instagram_business_basic"}},
		"互动数据": {Name: "互动数据", Description: "读取媒体互动指标，帮助 Agent 分析内容表现。", Items: []string{"查看点赞、评论等指标", "对比不同内容表现", "输出复盘摘要"}, Scopes: []string{"instagram_business_basic"}},
		"粉丝分析": {Name: "粉丝分析", Description: "整理账号受众和粉丝变化信息，用于运营判断。", Items: []string{"查看粉丝基础统计", "分析增长趋势", "辅助制定内容方向"}, Scopes: []string{"instagram_business_basic"}},
		"评论管理": {Name: "评论管理", Description: "读取和整理评论，用于客服响应、舆情处理和内容反馈。", Items: []string{"查看媒体评论", "识别高优先级反馈", "辅助生成回复建议"}, Scopes: []string{"instagram_business_basic"}},
	},
	"github": {
		"仓库管理":     {Name: "仓库管理", Description: "读取和管理 GitHub 仓库信息，让 Agent 能围绕真实代码仓库工作。", Items: []string{"查看仓库、分支和文件信息", "读取 README、目录和源码片段", "辅助整理仓库状态和变更范围"}, Scopes: []string{"repo"}},
		"Issue 跟踪": {Name: "Issue 跟踪", Description: "读取和更新 GitHub Issue，用于问题整理、任务跟进和缺陷排查。", Items: []string{"检索 Issue 列表和详情", "查看标签、状态和评论", "辅助生成回复或处理建议"}, Scopes: []string{"repo"}},
		"PR 审查":    {Name: "PR 审查", Description: "读取 Pull Request 的变更、评论和检查状态，帮助 Agent 做代码审查和合并判断。", Items: []string{"查看 PR diff 和提交", "读取 review comments", "汇总风险、测试和待处理项"}, Scopes: []string{"repo"}},
		"代码搜索":     {Name: "代码搜索", Description: "在仓库中搜索代码、符号和文本，帮助 Agent 快速定位实现位置。", Items: []string{"按关键词搜索代码", "定位文件路径和片段", "结合仓库上下文继续分析"}, Scopes: []string{"repo"}},
	},
}

func connectorFeatureDetailsFor(entry CatalogEntry) []FeatureDetail {
	detailByName := connectorFeatureDetails[entry.ConnectorID]
	result := make([]FeatureDetail, 0, len(entry.Features))
	for _, name := range entry.Features {
		if detail, ok := detailByName[name]; ok {
			result = append(result, detail)
		}
	}
	return result
}
