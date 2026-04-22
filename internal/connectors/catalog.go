package connectors

// CatalogEntry 表示一条连接器目录记录。
type CatalogEntry struct {
	ConnectorID   string
	Name          string
	Title         string
	Description   string
	Icon          string
	Category      string
	AuthType      string
	Status        string
	Provider      string
	RequiresExtra []string
	AuthURL       string
	TokenURL      string
	APIBaseURL    string
	Scopes        []string
	MCPServerURL  string
	DocsURL       string
	Features      []string
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
