// INPUT: 已真实接入的 Connector 元数据、能力说明与认证端点。
// OUTPUT: Connector 目录、详情和分类的服务端唯一真相。
// POS: Connector 服务静态目录；未实现产品不得以占位条目进入运行时。
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
	AutoOAuthClient bool
	DeviceAuth      bool
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
	"development":  "研发协作",
	"business":     "商业运营",
}

var connectorCatalog = []CatalogEntry{
	{
		ConnectorID:  "richmail",
		Name:         "richmail",
		Title:        "RichMail 邮箱",
		Description:  "连接本机 RichMail 客户端提供的 MCP 邮件工具",
		Icon:         "richmail",
		Category:     "productivity",
		AuthType:     "local_pairing",
		Status:       "available",
		Provider:     "richmail",
		APIBaseURL:   "http://127.0.0.1:3100",
		MCPServerURL: "http://127.0.0.1:3100/mcp",
		Features:     []string{},
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
		AutoOAuthClient: true,
		DeviceAuth:      true,
	},
	{
		ConnectorID:  "amap",
		Name:         "amap",
		Title:        "高德地图",
		Description:  "查询地点、地址、天气和路线，适合出行、门店和本地生活场景",
		Icon:         "amap",
		Category:     "business",
		AuthType:     "api_key",
		Status:       "available",
		Provider:     "amap",
		APIBaseURL:   "https://restapi.amap.com",
		MCPServerURL: "https://mcp.amap.com/mcp",
		DocsURL:      "https://lbs.amap.com/api/mcp-server/summary",
		Features:     []string{"地点搜索", "地理编码", "逆地理编码", "IP 定位", "天气查询", "路线规划", "距离测量"},
	},
	{
		ConnectorID:  "didi",
		Name:         "didi",
		Title:        "滴滴出行",
		Description:  "接入滴滴出行 MCP，支持打车估价、订单跟踪和路线规划",
		Icon:         "didi",
		Category:     "business",
		AuthType:     "api_key",
		Status:       "available",
		Provider:     "didi",
		APIBaseURL:   "https://mcp.didichuxing.com",
		MCPServerURL: "https://mcp.didichuxing.com/mcp-servers",
		DocsURL:      "https://mcp.didichuxing.com/api",
		Features:     []string{"打车估价", "生成打车链接", "创建订单", "订单查询", "取消订单", "司机位置", "地点搜索", "路线规划", "周边搜索"},
	},
	{
		ConnectorID:  "dingtalk-ai-table",
		Name:         "dingtalk-ai-table",
		Title:        "钉钉 AI 表格",
		Description:  "接入钉钉 AI 表格远程 MCP，支持 Base、Table、Field、Record 和附件操作",
		Icon:         "dingtalk",
		Category:     "productivity",
		AuthType:     "token",
		Status:       "available",
		Provider:     "dingtalk",
		APIBaseURL:   "https://mcp.dingtalk.com",
		MCPServerURL: "https://mcp.dingtalk.com/#/detail?mcpId=9555&detailType=marketMcpDetail",
		DocsURL:      "https://mcp.dingtalk.com/#/detail?mcpId=9555&detailType=marketMcpDetail",
		Features:     []string{"Base 管理", "Table 管理", "Field 管理", "Record 管理", "附件上传"},
	},
	{
		ConnectorID:  "tencent-docs",
		Name:         "tencent-docs",
		Title:        "腾讯文档",
		Description:  "接入腾讯文档官方 MCP，创建、查询和编辑智能文档、表格、幻灯片、流程图和空间内容",
		Icon:         "tencent-docs",
		Category:     "productivity",
		AuthType:     "token",
		Status:       "available",
		Provider:     "tencent-docs",
		APIBaseURL:   "https://docs.qq.com",
		MCPServerURL: "https://docs.qq.com/openapi/mcp",
		DocsURL:      "https://developer.cloud.tencent.com/mcp/server/11803",
		Features:     []string{"创建智能文档", "创建表格", "创建幻灯片", "创建流程图", "空间搜索", "读取内容", "更新表格", "智能表格"},
	},
	{
		ConnectorID: "yuque",
		Name:        "yuque",
		Title:       "语雀",
		Description: "接入语雀官方 MCP Server，搜索知识库、读取文档、管理知识库和笔记",
		Icon:        "yuque",
		Category:    "productivity",
		AuthType:    "token",
		Status:      "available",
		Provider:    "yuque",
		APIBaseURL:  "https://www.yuque.com/api/v2",
		DocsURL:     "https://github.com/yuque/yuque-mcp-server",
		Features:    []string{"用户信息", "全文搜索", "知识库管理", "文档管理", "资源管理", "目录管理", "笔记管理"},
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
		DeviceAuth:  true,
	},
}

var connectorFeatureDetails = map[string]map[string]FeatureDetail{
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
	"amap": {
		"地点搜索":  {Name: "地点搜索", Description: "按关键字检索高德 POI，返回名称、地址、经纬度、行政区和分类等信息。", Items: []string{"支持城市中文名、拼音、citycode 或 adcode 限定范围", "可控制分页、城市限制和返回详情级别", "适合查询门店、地址和周边候选地点"}},
		"地理编码":  {Name: "地理编码", Description: "把结构化地址转换为高德经纬度坐标，便于后续搜索、规划和距离计算。", Items: []string{"支持城市范围限定", "返回匹配地址和 location 坐标", "适合把文字地址转成可计算位置"}},
		"逆地理编码": {Name: "逆地理编码", Description: "把高德经纬度坐标转换为行政区划和结构化地址信息。", Items: []string{"返回省市区、乡镇街道和门牌相关信息", "适合坐标落点解释和地址补全", "可配合路线或距离工具使用"}},
		"IP 定位": {Name: "IP 定位", Description: "根据 IP 地址定位所在省市和城市编码。", Items: []string{"返回 province、city 和 adcode", "适合粗粒度本地化推荐", "不依赖浏览器定位权限"}},
		"天气查询":  {Name: "天气查询", Description: "按城市名称或标准 adcode 查询目标城市的实时天气或天气预报。", Items: []string{"支持城市或区县天气查询", "可返回实况天气或预报天气", "适合行程、门店运营和本地生活查询"}},
		"路线规划":  {Name: "路线规划", Description: "规划步行、驾车、公交和骑行路线，返回距离、耗时和步骤信息。", Items: []string{"支持起终点经纬度输入", "覆盖步行、驾车、公交、骑行等出行方式", "适合行程安排和通勤方案比较"}},
		"距离测量":  {Name: "距离测量", Description: "计算多个坐标点之间的距离，用于位置筛选和行程估算。", Items: []string{"支持起点和终点坐标", "可用于门店距离、路线预估和周边排序", "返回距离计算结果"}},
	},
	"didi": {
		"打车估价":   {Name: "打车估价", Description: "查询可用车型与价格预估，返回创建订单所需的预估流程 ID。", Items: []string{"起终点坐标必须来自实时地点搜索", "返回车型、车型标识和预估价格", "创建订单前必须先完成估价并向用户确认"}},
		"生成打车链接": {Name: "生成打车链接", Description: "生成跳转滴滴 App 或小程序的打车链接，由用户在客户端内完成下单。", Items: []string{"适合不托管完整下单流程的场景", "支持指定起终点和车型", "用户点击链接后在滴滴客户端继续操作"}},
		"创建订单":   {Name: "创建订单", Description: "通过滴滴 MCP 直接创建真实打车订单，调用前必须获得用户明确确认。", Items: []string{"依赖最新估价返回的 traceId", "会产生真实订单与费用", "禁止在用户未确认车型和价格时自动下单"}},
		"订单查询":   {Name: "订单查询", Description: "查询订单状态、司机信息和行程进度。", Items: []string{"支持查询当前账号未完成订单", "展示司机、车型、车牌和预计到达信息", "适合发单后持续跟踪"}},
		"取消订单":   {Name: "取消订单", Description: "取消指定打车订单，取消前应再次向用户确认。", Items: []string{"需要订单 ID", "行程中或完单后可能无法取消", "返回取消是否成功"}},
		"司机位置":   {Name: "司机位置", Description: "获取司机实时位置坐标，并可继续转换为可读地址。", Items: []string{"仅在有进行中订单时有效", "返回司机经纬度", "可配合逆地理编码展示位置"}},
		"地点搜索":   {Name: "地点搜索", Description: "按关键词和城市搜索 POI，获取后续路线和打车工具需要的坐标。", Items: []string{"坐标来源必须来自实时搜索结果", "返回名称、地址、城市和经纬度", "多候选结果应让用户确认"}},
		"路线规划":   {Name: "路线规划", Description: "规划驾车、公交、步行和骑行路线。", Items: []string{"支持起终点坐标输入", "覆盖多种出行方式", "适合路线比较和通勤咨询"}},
		"周边搜索":   {Name: "周边搜索", Description: "搜索指定坐标周边的 POI。", Items: []string{"支持关键词和中心点坐标", "可控制搜索半径", "适合附近停车场、酒店、咖啡等本地生活查询"}},
	},
	"dingtalk-ai-table": {
		"Base 管理":   {Name: "Base 管理", Description: "列出、搜索、创建和更新钉钉 AI 表格 Base。", Items: []string{"支持按名称搜索 Base", "可创建项目、销售、任务等业务表格空间", "适合把结构化办公数据交给 Agent 读写"}},
		"Table 管理":  {Name: "Table 管理", Description: "读取和维护 Base 下的表目录。", Items: []string{"列出 Base 内表格", "创建或更新数据表", "删除前应由用户明确确认"}},
		"Field 管理":  {Name: "Field 管理", Description: "读取和维护表字段结构。", Items: []string{"查看字段配置", "创建或更新字段", "适合让 Agent 搭建轻量业务数据模型"}},
		"Record 管理": {Name: "Record 管理", Description: "查询、创建、更新和删除 AI 表格记录。", Items: []string{"支持分页查询记录", "可批量新增或更新记录", "删除记录前必须确认目标表和记录"}},
		"附件上传":      {Name: "附件上传", Description: "申请附件上传地址并写入附件字段。", Items: []string{"先申请上传地址和 fileToken", "上传文件后再写入记录", "适合把报告、图片、PDF 等归档到业务表"}},
	},
	"tencent-docs": {
		"创建智能文档": {Name: "创建智能文档", Description: "通过 Markdown 创建腾讯文档智能文档，适合报告、纪要和方案草稿。", Items: []string{"默认推荐用于通用文档创建", "支持标题、段落、列表、表格和代码块", "可指定父目录创建"}},
		"创建表格":   {Name: "创建表格", Description: "通过 Markdown 表格创建腾讯文档 Excel。", Items: []string{"适合数据报表和统计表", "可生成结构化表格", "后续可继续追加或更新单元格"}},
		"创建幻灯片":  {Name: "创建幻灯片", Description: "通过层级 Markdown 创建腾讯文档幻灯片。", Items: []string{"适合项目汇报和培训材料", "每页建议控制 2 到 4 个段落", "自动生成在线演示文稿"}},
		"创建流程图":  {Name: "创建流程图", Description: "通过 Mermaid 创建在线流程图。", Items: []string{"适合流程、架构和时序展示", "Mermaid 内容需遵循腾讯文档 MCP 要求", "可直接沉淀到腾讯文档空间"}},
		"空间搜索":   {Name: "空间搜索", Description: "搜索腾讯文档空间文件并定位目标内容。", Items: []string{"按关键词搜索文档", "返回空间节点和文档信息", "可继续读取匹配文档内容"}},
		"读取内容":   {Name: "读取内容", Description: "读取指定腾讯文档的正文内容。", Items: []string{"基于 file_id 获取文档内容", "适合摘要、改写和问答", "可结合空间搜索继续处理"}},
		"更新表格":   {Name: "更新表格", Description: "批量更新腾讯文档表格单元格。", Items: []string{"适用于 Excel 文档", "支持追加数据", "写入前应确认目标文件和范围"}},
		"智能表格":   {Name: "智能表格", Description: "操作腾讯文档智能表格的工作表、视图、字段和记录。", Items: []string{"支持字段和记录管理", "适合任务跟踪、客户表和轻量业务库", "复杂写入前先读取表结构"}},
	},
	"yuque": {
		"用户信息":  {Name: "用户信息", Description: "读取当前语雀用户信息。", Items: []string{"获取昵称、头像和个人简介", "用于确认当前 Token 对应账号", "避免写入错误账号"}},
		"全文搜索":  {Name: "全文搜索", Description: "在语雀内容中搜索知识库、文档和笔记。", Items: []string{"按关键词检索资料", "适合快速找历史方案和沉淀文档", "可继续读取命中文档"}},
		"知识库管理": {Name: "知识库管理", Description: "列出、读取、创建和更新语雀知识库。", Items: []string{"浏览个人或团队知识库", "读取知识库元数据", "可创建或更新知识库信息"}},
		"文档管理":  {Name: "文档管理", Description: "列出、读取、创建和更新语雀文档。", Items: []string{"读取文档正文", "创建新文档", "按用户确认更新已有文档"}},
		"资源管理":  {Name: "资源管理", Description: "管理语雀资源对象。", Items: []string{"读取资源内容", "创建或更新资源", "适合处理附件式知识资产"}},
		"目录管理":  {Name: "目录管理", Description: "读取和更新语雀知识库目录。", Items: []string{"获取 TOC 结构", "维护目录顺序", "更新前应确认目标知识库"}},
		"笔记管理":  {Name: "笔记管理", Description: "列出、读取、创建和更新语雀笔记。", Items: []string{"适合个人速记和临时沉淀", "可把对话结果写入笔记", "更新前应确认目标笔记"}},
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
