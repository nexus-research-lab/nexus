package automation

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// CronJobMatchesQuery 判断定时任务是否匹配用户口头描述。
func CronJobMatchesQuery(job protocol.CronJob, query string) bool {
	return CronJobMatchScore(job, query) > 0
}

// BestMatchingCronJobs 返回分数最高的一组候选。
// list/search 场景可以继续宽松召回；启停、删除、修改这类直接执行工具应使用最高分候选，
// 避免“飞书群暂停新闻”被宽泛别名同时匹配到所有飞书任务和所有暂停任务。
func BestMatchingCronJobs(jobs []protocol.CronJob, query string) []protocol.CronJob {
	if strings.TrimSpace(query) == "" {
		return jobs
	}
	bestScore := 0
	matches := make([]protocol.CronJob, 0, len(jobs))
	for _, job := range jobs {
		score := CronJobMatchScore(job, query)
		if score <= 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			matches = matches[:0]
		}
		if score == bestScore {
			matches = append(matches, job)
		}
	}
	return matches
}

// CronJobMatchScore 计算任务与自然语言 query 的匹配强度。
func CronJobMatchScore(job protocol.CronJob, query string) int {
	variants := QueryVariants(query)
	if len(variants) == 0 || (len(variants) == 1 && variants[0] == "") {
		return 1
	}
	haystack := strings.ToLower(strings.Join(CronJobSearchTerms(job), "\n"))
	score := 0
	base := strings.ToLower(strings.TrimSpace(query))
	if strings.Contains(haystack, base) {
		score += 100
	}
	for _, aliases := range matchedQueryAliasGroups(base) {
		if haystackMatchesAnyAlias(haystack, aliases) {
			score += 20
		}
	}
	for _, token := range queryTextTokens(base) {
		if strings.Contains(haystack, token) {
			score += 50
		}
	}
	return score
}

// CronJobSearchTerms 返回一个定时任务可被自然语言定位的稳定字段集合。
func CronJobSearchTerms(job protocol.CronJob) []string {
	terms := []string{
		job.JobID,
		job.Name,
		job.Instruction,
		job.AgentID,
		job.ExecutionKind,
		job.SessionTarget.Kind,
		job.SessionTarget.BoundSessionKey,
		job.SessionTarget.NamedSessionKey,
		job.SessionTarget.WakeMode,
		job.Delivery.Mode,
		job.Delivery.Channel,
		job.Delivery.To,
		job.Delivery.AccountID,
		job.Delivery.ThreadID,
		job.Source.Kind,
		job.Source.CreatorAgentID,
		job.Source.ContextType,
		job.Source.ContextID,
		job.Source.ContextLabel,
		job.Source.SessionKey,
		job.Source.SessionLabel,
		job.LastRunStatus,
		job.LastDeliveryStatus,
		job.RunningRunID,
	}
	terms = append(terms, scheduleSearchTerms(job.Schedule)...)
	terms = append(terms, deliveryChannelAliases(job.Delivery.Channel)...)
	terms = append(terms, boolAlias("enabled", "disabled", job.Enabled)...)
	terms = append(terms, boolAlias("running", "idle", job.Running)...)
	return terms
}

// QueryVariants 将用户说法展开成可用于任务和审计事件搜索的别名集合。
func QueryVariants(query string) []string {
	base := strings.ToLower(strings.TrimSpace(query))
	if base == "" {
		return []string{""}
	}
	variants := []string{base}
	for _, aliases := range queryAliasGroups() {
		if !queryMatchesAnyAlias(base, aliases) {
			continue
		}
		variants = append(variants, aliases...)
	}
	return uniqueNonEmptyLowerStrings(variants)
}

func matchedQueryAliasGroups(query string) [][]string {
	groups := make([][]string, 0)
	for _, aliases := range queryAliasGroups() {
		if queryMatchesAnyAlias(query, aliases) {
			groups = append(groups, aliases)
		}
	}
	return groups
}

func haystackMatchesAnyAlias(haystack string, aliases []string) bool {
	for _, alias := range aliases {
		normalized := strings.ToLower(strings.TrimSpace(alias))
		if normalized != "" && strings.Contains(haystack, normalized) {
			return true
		}
	}
	return false
}

func queryTextTokens(query string) []string {
	remainder := strings.ToLower(strings.TrimSpace(query))
	removeTerms := make([]string, 0)
	for _, aliases := range matchedQueryAliasGroups(remainder) {
		removeTerms = append(removeTerms, aliases...)
	}
	removeTerms = append(removeTerms, queryFillerTerms()...)
	sort.Slice(removeTerms, func(i, j int) bool {
		return utf8.RuneCountInString(removeTerms[i]) > utf8.RuneCountInString(removeTerms[j])
	})
	for _, term := range removeTerms {
		normalized := strings.ToLower(strings.TrimSpace(term))
		if normalized == "" {
			continue
		}
		remainder = strings.ReplaceAll(remainder, normalized, " ")
	}
	rawTokens := strings.FieldsFunc(remainder, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	tokens := make([]string, 0, len(rawTokens))
	for _, token := range rawTokens {
		token = strings.TrimSpace(token)
		if token == "" || utf8.RuneCountInString(token) < 2 {
			continue
		}
		tokens = append(tokens, token)
		tokens = append(tokens, cjkBigrams(token)...)
	}
	return uniqueNonEmptyLowerStrings(tokens)
}

func queryFillerTerms() []string {
	return []string{
		"定时任务",
		"任务",
		"那个",
		"这个",
		"这条",
		"帮我",
		"请",
		"一下",
		"一个",
		"恢复",
		"重新启用",
		"重新打开",
		"继续",
		"打开",
		"启用",
		"暂停",
		"停用",
		"停止",
		"删除",
		"移除",
		"修改",
		"改成",
		"改到",
		"改为",
		"检查",
		"查看",
		"发送情况",
		"发送",
		"投递",
		"发到",
		"发给",
		"的",
		"把",
	}
}

func cjkBigrams(value string) []string {
	runes := []rune(value)
	if len(runes) <= 2 || !containsCJK(runes) {
		return nil
	}
	items := make([]string, 0, len(runes)-1)
	for index := 0; index < len(runes)-1; index++ {
		items = append(items, string(runes[index:index+2]))
	}
	return items
}

func containsCJK(runes []rune) bool {
	for _, r := range runes {
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			return true
		}
	}
	return false
}

func scheduleSearchTerms(schedule protocol.Schedule) []string {
	terms := []string{
		schedule.Kind,
		schedule.Timezone,
	}
	if schedule.RunAt != nil {
		terms = append(terms, *schedule.RunAt)
	}
	if schedule.IntervalSeconds != nil {
		terms = append(terms, "interval", "every")
	}
	if schedule.CronExpression != nil {
		terms = append(terms, *schedule.CronExpression, "cron")
	}
	return terms
}

func deliveryChannelAliases(channel string) []string {
	switch protocol.NormalizeStoredChannelType(channel) {
	case protocol.SessionChannelFeishu:
		return []string{"feishu", "fs", "飞书", "飞书群"}
	case protocol.SessionChannelDingTalk:
		return []string{"dingtalk", "dt", "钉钉", "钉钉群"}
	case protocol.SessionChannelTelegram:
		return []string{"telegram", "tg"}
	case protocol.SessionChannelDiscord:
		return []string{"discord", "dg"}
	case protocol.SessionChannelWeChat:
		return []string{"wechat", "wx", "微信", "微信群"}
	case protocol.SessionChannelInternalSegment:
		return []string{"internal", "内部", "智能体", "收件箱", "定时任务收件箱"}
	case protocol.SessionChannelWebSocket:
		return []string{"websocket", "ws", "会话", "当前会话"}
	default:
		return nil
	}
}

func queryAliasGroups() [][]string {
	return [][]string{
		{"feishu", "fs", "飞书", "飞书群"},
		{"dingtalk", "dt", "钉钉", "钉钉群"},
		{"telegram", "tg"},
		{"discord", "dg"},
		{"wechat", "wx", "微信", "微信群"},
		{"internal", "内部", "智能体", "收件箱", "定时任务收件箱"},
		{"websocket", "ws", "会话", "当前会话"},
		{"running", "运行中", "正在跑"},
		{"idle", "空闲", "未运行"},
		{"disabled", "停用", "暂停"},
		{"enabled", "启用"},
	}
}

func queryMatchesAnyAlias(query string, aliases []string) bool {
	for _, alias := range aliases {
		normalized := strings.ToLower(strings.TrimSpace(alias))
		if normalized != "" && strings.Contains(query, normalized) {
			return true
		}
	}
	return false
}

func uniqueNonEmptyLowerStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}

func boolAlias(trueAlias string, falseAlias string, value bool) []string {
	if value {
		return []string{trueAlias}
	}
	return []string{falseAlias}
}
