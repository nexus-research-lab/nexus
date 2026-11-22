// INPUT: Connector 目录/详情视图与请求的展示语言。
// OUTPUT: 英文界面使用的 Connector 标题、说明与能力文案。
// POS: Connector 目录文案的英文投影；中文目录仍是真相源，不改变标识、授权范围或连接状态。
package connectors

import (
	"slices"
	"strings"
)

type connectorText struct {
	title       string
	description string
	// features 以中文目录中的能力名为键，与 connectorFeatureDetails 保持同一身份。
	features map[string]featureText
}

type featureText struct {
	name        string
	description string
	items       []string
}

var englishConnectorCatalog = map[string]connectorText{
	"richmail": {
		title:       "RichMail",
		description: "Use the MCP mail tools provided by the RichMail client on this computer",
	},
	"feishu-docx": {
		title:       "Feishu Docs",
		description: "Read, search, create and update Feishu documents, and browse Drive, wikis, Sheets and Bitable",
		features: map[string]featureText{
			"阅读文档":       {name: "Read documents", description: "Read a Feishu Docx or Wiki document and convert it to agent-friendly Markdown.", items: []string{"Accepts docx links, wiki links or document IDs", "Keeps headings, paragraphs, lists, tables and block ID markers", "Good for summarizing, rewriting or continuing to edit a document"}},
			"全文搜索":       {name: "Full-text search", description: "Search the documents, wikis and workspace content the user can access with Feishu search.", items: []string{"Search documents by keyword", "Returns titles, types, links and summaries", "Matching documents can be read next"}},
			"Sheet 内容":   {name: "Sheet content", description: "Read data from regular Feishu Sheets so the agent can analyze tables, summarize them or locate cells.", items: []string{"Resolves sheets links and sheet IDs", "Reads a range from a given worksheet", "Returns structured rows and columns"}},
			"Bitable 内容": {name: "Bitable content", description: "Read Feishu Bitable schemas and records so the agent can inspect fields and records in a business database.", items: []string{"Resolves base and table links", "Lists table fields and records", "Shows record values and basic metadata"}},
			"创建文档":       {name: "Create documents", description: "Create a new Docx document in Feishu Drive for meeting notes, drafts or run results.", items: []string{"Creates a document with the given title", "Returns the new document link and document ID", "Blocks can be appended or updated afterwards"}},
			"更新 Block":   {name: "Update blocks", description: "Update or append blocks by Feishu Docx block ID for precise edits to an existing document.", items: []string{"Locates content by block ID", "Appends child blocks", "Updates text block content"}},
			"云空间列表":      {name: "Drive listing", description: "List the Feishu Drive files the user can access to find recent documents or a folder's contents.", items: []string{"Browses the Drive file list", "Returns file types, names and tokens", "Works with document reading for follow-up steps"}},
			"知识库浏览":      {name: "Wiki browsing", description: "Browse Feishu wiki spaces and node trees, and resolve wiki nodes to the underlying documents.", items: []string{"Lists accessible wiki spaces", "Pages through the node tree", "Resolves a wiki node to its docx document"}},
		},
	},
	"amap": {
		title:       "Amap",
		description: "Look up places, addresses, weather and routes for travel, stores and local services",
		features: map[string]featureText{
			"地点搜索":  {name: "Place search", description: "Search Amap POIs by keyword and get names, addresses, coordinates, districts and categories.", items: []string{"Limit the search by city name, pinyin, citycode or adcode", "Controls paging, city restriction and detail level", "Good for finding stores, addresses and nearby candidates"}},
			"地理编码":  {name: "Geocoding", description: "Convert a structured address into Amap coordinates for search, routing and distance calculation.", items: []string{"Can be limited to a city", "Returns the matched address and location coordinates", "Turns a written address into a position you can compute with"}},
			"逆地理编码": {name: "Reverse geocoding", description: "Convert Amap coordinates into administrative divisions and a structured address.", items: []string{"Returns province, city, district, township, street and house number", "Good for explaining a coordinate or completing an address", "Works with the route and distance tools"}},
			"IP 定位": {name: "IP location", description: "Locate the province, city and city code for an IP address.", items: []string{"Returns province, city and adcode", "Good for coarse local recommendations", "Does not need the browser's location permission"}},
			"天气查询":  {name: "Weather", description: "Get current weather or a forecast for a city by name or standard adcode.", items: []string{"Covers cities and districts", "Returns current conditions or a forecast", "Good for trips, store operations and local questions"}},
			"路线规划":  {name: "Route planning", description: "Plan walking, driving, transit and cycling routes with distance, duration and steps.", items: []string{"Takes origin and destination coordinates", "Covers walking, driving, transit and cycling", "Good for planning trips and comparing commutes"}},
			"距离测量":  {name: "Distance measurement", description: "Calculate the distance between coordinates to filter places and estimate trips.", items: []string{"Takes origin and destination coordinates", "Useful for store distance, route estimates and sorting by proximity", "Returns the calculated distance"}},
		},
	},
	"didi": {
		title:       "DiDi",
		description: "Connect the DiDi MCP for ride price estimates, order tracking and route planning",
		features: map[string]featureText{
			"打车估价":   {name: "Fare estimate", description: "Check available ride types and estimated prices, and get the estimate ID needed to create an order.", items: []string{"Origin and destination coordinates must come from a live place search", "Returns ride types, their identifiers and estimated prices", "Estimate and confirm with the user before creating an order"}},
			"生成打车链接": {name: "Ride link", description: "Generate a link that opens the DiDi app or mini program, where the user places the order.", items: []string{"For cases that do not manage the full ordering flow", "Supports a set origin, destination and ride type", "The user continues in the DiDi client after opening the link"}},
			"创建订单":   {name: "Create order", description: "Create a real ride order through the DiDi MCP. The user must explicitly confirm before the call.", items: []string{"Needs the traceId from the latest estimate", "Creates a real order with real charges", "Never orders before the user confirms the ride type and price"}},
			"订单查询":   {name: "Order status", description: "Check order status, driver details and trip progress.", items: []string{"Finds the account's unfinished orders", "Shows driver, ride type, plate number and estimated arrival", "Good for tracking a ride after ordering"}},
			"取消订单":   {name: "Cancel order", description: "Cancel a ride order. Confirm with the user again first.", items: []string{"Needs the order ID", "May not be possible during or after the trip", "Returns whether the cancellation succeeded"}},
			"司机位置":   {name: "Driver location", description: "Get the driver's live coordinates, which can then be turned into a readable address.", items: []string{"Only works while an order is in progress", "Returns the driver's coordinates", "Works with reverse geocoding to show the location"}},
			"地点搜索":   {name: "Place search", description: "Search POIs by keyword and city to get the coordinates the route and ride tools need.", items: []string{"Coordinates must come from live search results", "Returns names, addresses, cities and coordinates", "Ask the user to confirm when there are several candidates"}},
			"路线规划":   {name: "Route planning", description: "Plan driving, transit, walking and cycling routes.", items: []string{"Takes origin and destination coordinates", "Covers several ways to travel", "Good for comparing routes and commute questions"}},
			"周边搜索":   {name: "Nearby search", description: "Search POIs around a coordinate.", items: []string{"Takes a keyword and a center coordinate", "The search radius can be set", "Good for nearby parking, hotels, coffee and other local needs"}},
		},
	},
	"dingtalk-ai-table": {
		title:       "DingTalk AI Tables",
		description: "Connect the DingTalk AI Tables remote MCP to work with bases, tables, fields, records and attachments",
		features: map[string]featureText{
			"Base 管理":   {name: "Bases", description: "List, search, create and update DingTalk AI Tables bases.", items: []string{"Search bases by name", "Create workspaces for projects, sales, tasks and more", "Lets the agent read and write structured work data"}},
			"Table 管理":  {name: "Tables", description: "Read and maintain the tables in a base.", items: []string{"Lists the tables in a base", "Creates or updates tables", "The user must explicitly confirm before deleting"}},
			"Field 管理":  {name: "Fields", description: "Read and maintain a table's field structure.", items: []string{"Shows field settings", "Creates or updates fields", "Lets the agent set up a lightweight data model"}},
			"Record 管理": {name: "Records", description: "Query, create, update and delete AI Tables records.", items: []string{"Pages through records", "Adds or updates records in bulk", "Confirm the target table and records before deleting"}},
			"附件上传":      {name: "Attachment upload", description: "Request an upload URL and write the file into an attachment field.", items: []string{"Requests an upload URL and fileToken first", "Writes the record after the file is uploaded", "Good for filing reports, images and PDFs in a table"}},
		},
	},
	"tencent-docs": {
		title:       "Tencent Docs",
		description: "Connect the official Tencent Docs MCP to create, find and edit smart documents, sheets, slides, flowcharts and space content",
		features: map[string]featureText{
			"创建智能文档": {name: "Create smart documents", description: "Create a Tencent Docs smart document from Markdown for reports, notes and drafts.", items: []string{"The recommended default for general documents", "Supports headings, paragraphs, lists, tables and code blocks", "Can be created in a chosen parent folder"}},
			"创建表格":   {name: "Create sheets", description: "Create a Tencent Docs spreadsheet from a Markdown table.", items: []string{"Good for data reports and statistics", "Produces a structured sheet", "Cells can be appended or updated later"}},
			"创建幻灯片":  {name: "Create slides", description: "Create Tencent Docs slides from hierarchical Markdown.", items: []string{"Good for project updates and training material", "Keep each slide to two to four paragraphs", "Generates an online presentation automatically"}},
			"创建流程图":  {name: "Create flowcharts", description: "Create an online flowchart from Mermaid.", items: []string{"Good for flows, architecture and sequence diagrams", "Mermaid content must follow the Tencent Docs MCP requirements", "Saved directly to the Tencent Docs space"}},
			"空间搜索":   {name: "Space search", description: "Search files in a Tencent Docs space and locate the content you need.", items: []string{"Search documents by keyword", "Returns space nodes and document details", "Matching documents can be read next"}},
			"读取内容":   {name: "Read content", description: "Read the body of a given Tencent Docs document.", items: []string{"Fetches document content by file_id", "Good for summaries, rewrites and Q&A", "Works with space search for follow-up steps"}},
			"更新表格":   {name: "Update sheets", description: "Update Tencent Docs spreadsheet cells in bulk.", items: []string{"For spreadsheet documents", "Supports appending data", "Confirm the target file and range before writing"}},
			"智能表格":   {name: "Smart sheets", description: "Work with the sheets, views, fields and records of Tencent Docs smart sheets.", items: []string{"Manages fields and records", "Good for task tracking, customer lists and lightweight databases", "Read the table structure before complex writes"}},
		},
	},
	"yuque": {
		title:       "Yuque",
		description: "Connect the official Yuque MCP Server to search knowledge bases, read documents and manage knowledge bases and notes",
		features: map[string]featureText{
			"用户信息":  {name: "User profile", description: "Read the current Yuque user's profile.", items: []string{"Gets the nickname, avatar and bio", "Confirms which account the token belongs to", "Avoids writing to the wrong account"}},
			"全文搜索":  {name: "Full-text search", description: "Search knowledge bases, documents and notes in Yuque.", items: []string{"Search by keyword", "Good for finding past proposals and saved documents quickly", "Matching documents can be read next"}},
			"知识库管理": {name: "Knowledge bases", description: "List, read, create and update Yuque knowledge bases.", items: []string{"Browses personal and team knowledge bases", "Reads knowledge base metadata", "Creates or updates knowledge base details"}},
			"文档管理":  {name: "Documents", description: "List, read, create and update Yuque documents.", items: []string{"Reads document content", "Creates new documents", "Updates existing documents once the user confirms"}},
			"资源管理":  {name: "Resources", description: "Manage Yuque resource objects.", items: []string{"Reads resource content", "Creates or updates resources", "Good for attachment-style knowledge assets"}},
			"目录管理":  {name: "Table of contents", description: "Read and update a Yuque knowledge base's table of contents.", items: []string{"Gets the TOC structure", "Maintains the order of entries", "Confirm the target knowledge base before updating"}},
			"笔记管理":  {name: "Notes", description: "List, read, create and update Yuque notes.", items: []string{"Good for quick personal notes and temporary capture", "Saves conversation results to a note", "Confirm the target note before updating"}},
		},
	},
	"github": {
		title:       "GitHub",
		description: "Manage repositories, collaborate on code and track issues",
		features: map[string]featureText{
			"仓库管理":     {name: "Repositories", description: "Read and manage GitHub repository information so the agent can work with real codebases.", items: []string{"Views repositories, branches and files", "Reads READMEs, directories and source snippets", "Helps summarize repository status and the scope of changes"}},
			"Issue 跟踪": {name: "Issue tracking", description: "Read and update GitHub issues to organize problems, follow up on tasks and investigate bugs.", items: []string{"Searches issue lists and details", "Shows labels, status and comments", "Helps draft replies or suggested fixes"}},
			"PR 审查":    {name: "PR review", description: "Read a pull request's changes, comments and check status to help the agent review code and judge whether to merge.", items: []string{"Views the PR diff and commits", "Reads review comments", "Summarizes risks, tests and open items"}},
			"代码搜索":     {name: "Code search", description: "Search code, symbols and text in a repository to find where something is implemented.", items: []string{"Search code by keyword", "Locates file paths and snippets", "Continues the analysis with repository context"}},
		},
	},
}

func isEnglishLocale(locale string) bool {
	return strings.HasPrefix(strings.ToLower(locale), "en")
}

// LocalizeInfo 把目录项的标题与说明投影为请求语言；
// 仅英文界面替换文案，自定义 MCP 与其余语言保留原文。
func LocalizeInfo(info Info, locale string) Info {
	text, ok := englishConnectorCatalog[info.ConnectorID]
	if !ok || info.Kind != ConnectorKindCatalog || !isEnglishLocale(locale) {
		return info
	}
	info.Title = text.title
	info.Description = text.description
	return info
}

// LocalizeDetail 在 LocalizeInfo 之上同步投影能力名称、说明与要点，授权范围保持不变。
func LocalizeDetail(detail Detail, locale string) Detail {
	text, ok := englishConnectorCatalog[detail.ConnectorID]
	if !ok || detail.Kind != ConnectorKindCatalog || !isEnglishLocale(locale) {
		return detail
	}
	detail.Info = LocalizeInfo(detail.Info, locale)
	features := make([]string, len(detail.Features))
	for index, name := range detail.Features {
		features[index] = name
		if feature, ok := text.features[name]; ok {
			features[index] = feature.name
		}
	}
	featureDetails := make([]FeatureDetail, len(detail.FeatureDetails))
	for index, item := range detail.FeatureDetails {
		if feature, ok := text.features[item.Name]; ok {
			item.Name = feature.name
			item.Description = feature.description
			item.Items = slices.Clone(feature.items)
		}
		featureDetails[index] = item
	}
	detail.Features = features
	detail.FeatureDetails = featureDetails
	return detail
}
