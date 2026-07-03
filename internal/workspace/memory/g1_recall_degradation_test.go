package memory

// g1_recall_degradation_test.go
// 论文 G1（记忆召回）课题的可重复退化实验 —— Lucy 第三棒交付物。
//
// 三组实验，全部基于真实 Engine.Search 端到端路径（非算法复制版）：
//   1. TestG1RecallCliff     —— 召回池断崖：ListEntries 硬上限 200 条带来的"隐性遗忘"
//   2. TestG1LexicalBlindSpot —— 词法盲区：纯 tokenOverlap 无法识别"语义相关但字面不同"
//   3. TestG1Latency         —— 延迟曲线：证伪"O(N) 全表扫会慢"的直觉，定位真实瓶颈
//
// 复现命令：
//   cd nexus && go test -run 'TestG1' -v -timeout 600s ./internal/workspace/memory/
//
// 已核实源码锚点（nexus/internal/workspace/memory/）：
//   - defaultListLimit = 200                  (engine.go:10)
//   - Search 只取 ListEntries(200)            (engine_query.go:61)
//   - ListEntries 文件降序(新→旧)、文件内倒序   (repository_engine_entry.go:23 + repository_file.go:89)
//   - scoreItem 主路径只算 tokenOverlap        (engine_scope_score.go:8-35，不调 levenshtein)
//   - tokenizeText 中文逐字、英文 [0-9a-zA-Z_]+ (similarity.go:99-112)
//   - ScoreThreshold=0.08 / MaxResults=5      (model_engine.go:101-103)
//   - 无 forget/decay/evict/TTL                (Tom 第二棒 grep 零命中)

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	g1ScopeKey = "room_shared:r1:c1"
	g1QueryCJK = "缓存失效诊断" // 召回查询（中文，逐字成 token）
)

func g1Scope() MemoryScope {
	return MemoryScope{Kind: ScopeKindRoomShared, RoomID: "r1", ConversationID: "c1"}
}

// g1Entry 构造一条测试记忆：同 scope / 同 status / 同 priority，控制变量。
func g1Entry(id string, createdAt time.Time, title string) *Entry {
	e := &Entry{
		ID:        id,
		CreatedAt: createdAt,
		Kind:      "LRN",
		Title:     title,
		Category:  "复现",
	}
	e.SetField("Scope", g1ScopeKey)
	e.SetField("状态", "active") // statusBoost=0.12
	e.SetField("详情", title)
	return e
}

func writeDiary(t *testing.T, dir, date string, entries []*Entry) {
	t.Helper()
	memDir := filepath.Join(dir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, e.Markdown())
	}
	body := strings.Join(parts, "\n\n") + "\n"
	if err := os.WriteFile(filepath.Join(memDir, date+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", date, err)
	}
}

// seedByDay 把多条 entry 按各自的 CreatedAt 日期分文件落盘。
func seedByDay(t *testing.T, dir string, entries []*Entry) {
	t.Helper()
	byDay := map[string][]*Entry{}
	for _, e := range entries {
		byDay[e.CreatedAt.Format("2006-01-02")] = append(byDay[e.CreatedAt.Format("2006-01-02")], e)
	}
	for day, group := range byDay {
		writeDiary(t, dir, day, group)
	}
}

// noiseTimestamps 生成从 start 起 count 个时间戳，每天至多 perDay 条（制造跨天文件）。
func noiseTimestamps(start time.Time, count, perDay int) []time.Time {
	out := make([]time.Time, 0, count)
	for day := 0; len(out) < count; day++ {
		base := start.AddDate(0, 0, day)
		for i := 0; i < perDay && len(out) < count; i++ {
			out = append(out, base.Add(time.Duration(i)*time.Minute))
		}
	}
	return out
}

// ---------------------------------------------------------------------------

// TestG1RecallCliff：召回池断崖。
// 目标条目（含 query 全部 token）固定在最老日期，其余 N-1 条噪音更新。
// 预期：N<=200 可召回；N>=201 目标被挤出最近 200 池 → 召回率 0%（断崖，非曲线）。
func TestG1RecallCliff(t *testing.T) {
	sizes := []int{50, 200, 201, 300, 500, 1000, 2000}
	target := time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local)
	noiseStart := time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local)

	type row struct {
		n, pool int
		hit     bool
	}
	var rows []row

	for _, n := range sizes {
		dir := t.TempDir()
		writeDiary(t, dir, "2026-01-01", []*Entry{
			g1Entry("LRN-20260101-TGT", target, "缓存失效诊断的标准流程与排错工具"),
		})
		noise := make([]*Entry, 0, n-1)
		ts := noiseTimestamps(noiseStart, n-1, 50)
		for i, c := range ts {
			noise = append(noise, g1Entry(
				fmt.Sprintf("LRN-%s-%04d", c.Format("20060102"), i+1),
				c, fmt.Sprintf("noise entry padding xyz %d", i+1)))
		}
		seedByDay(t, dir, noise)

		eng := NewEngine(dir, DefaultOptions())
		items, err := eng.Search(context.Background(), g1Scope(), RecallRequest{Query: g1QueryCJK})
		if err != nil {
			t.Errorf("N=%d search err: %v", n, err)
			continue
		}
		hit := false
		for _, it := range items {
			if strings.Contains(it.Title, "缓存失效诊断") {
				hit = true
				break
			}
		}
		rows = append(rows, row{n, len(items), hit})
	}

	t.Log("=== G1 召回池断崖（目标条目固定在最老日期）===")
	t.Log("N_total | returned | target_hit")
	for _, r := range rows {
		flag := "OK"
		if !r.hit {
			flag = "MISSED"
		}
		t.Logf("%7d | %5d    | %v (%s)", r.n, r.pool, r.hit, flag)
	}
	// 断言断崖点：N<=200 命中，N>=201 漏（验证 ListEntries(200) 硬截断）
	for _, r := range rows {
		if r.n <= 200 && !r.hit {
			t.Errorf("N=%d 应命中（<=200 在召回池内），但 missed", r.n)
		}
		if r.n >= 201 && r.hit {
			t.Errorf("N=%d 不应命中（>200 目标被挤出池），但 hit —— 断崖假设被推翻，请复核", r.n)
		}
	}
}

// ---------------------------------------------------------------------------

// TestG1LexicalBlindSpot：词法召回语义盲区。
// 同 scope / 同 status / 同时间，控制 boost 相同，只改 token 重叠。
// 预期：B(语义相关但字面不同) 与 C(完全不相关) 分数完全相等 → 词法召回无法识别语义。
func TestG1LexicalBlindSpot(t *testing.T) {
	scope := g1Scope()
	now := time.Now()
	mk := func(title string) MemoryItem {
		return MemoryItem{
			Title: title, Content: title, Status: "active",
			Scope: g1ScopeKey, CreatedAt: now, AccessCount: 1,
		}
	}
	query := "记忆召回机制"
	a := mk("记忆召回机制的实现细节")    // 字面高度重叠
	b := mk("长期上下文的持久化与检索能力") // 语义相关，字面零重叠
	c := mk("今日天气与体育赛事摘要")    // 完全不相关

	sA := scoreItem(query, scope, a)
	sB := scoreItem(query, scope, b)
	sC := scoreItem(query, scope, c)
	t.Logf("=== G1 词法盲区（query=%q）===", query)
	t.Logf("A 字面重叠   score=%.4f", sA)
	t.Logf("B 语义相关   score=%.4f  (tokenOverlap=0，分数纯靠 boost)", sB)
	t.Logf("C 完全无关   score=%.4f", sC)
	if sB != sC {
		t.Errorf("语义相关(B=%.4f)与无关(C=%.4f)分数应相等（词法盲区），差异说明存在非词法信号，请复核", sB, sC)
	}
	if sA <= sB {
		t.Errorf("字面重叠项(A=%.4f)应高于语义盲区项(B=%.4f)", sA, sB)
	}
}

// ---------------------------------------------------------------------------

// TestG1Latency：召回延迟曲线。
// 证伪"N 大→全表扫慢"：ListEntries 在最近文件凑满 200 即停，延迟应近恒定。
func TestG1Latency(t *testing.T) {
	sizes := []int{200, 500, 1000, 2000, 5000}
	noiseStart := time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local)

	t.Log("=== G1 召回延迟（3 次 warmup 后取 10 次中位数）===")
	t.Log("N_total | files | median | max")
	for _, n := range sizes {
		dir := t.TempDir()
		writeDiary(t, dir, "2026-01-01", []*Entry{
			g1Entry("LRN-20260101-TGT", time.Date(2026, 1, 1, 9, 0, 0, 0, time.Local), "缓存失效诊断"),
		})
		ts := noiseTimestamps(noiseStart, n-1, 50)
		noise := make([]*Entry, 0, n-1)
		for i, c := range ts {
			noise = append(noise, g1Entry(
				fmt.Sprintf("LRN-%s-%04d", c.Format("20060102"), i+1),
				c, fmt.Sprintf("noise entry padding xyz %d", i+1)))
		}
		seedByDay(t, dir, noise)

		files, _ := os.ReadDir(filepath.Join(dir, "memory"))
		eng := NewEngine(dir, DefaultOptions())
		var durs []time.Duration
		for i := 0; i < 13; i++ {
			start := time.Now()
			_, _ = eng.Search(context.Background(), g1Scope(), RecallRequest{Query: g1QueryCJK})
			if i >= 3 {
				durs = append(durs, time.Since(start))
			}
		}
		sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
		median := durs[len(durs)/2]
		t.Logf("%7d | %5d | %v | %v", n, len(files), median, durs[len(durs)-1])
	}
}
