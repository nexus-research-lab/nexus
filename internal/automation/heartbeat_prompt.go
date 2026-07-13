package automation

import "strings"

// HeartbeatTask 表示 HEARTBEAT.md 里声明的一条周期任务提示。
type HeartbeatTask struct {
	Name     string
	Interval string
	Prompt   string
}

// ParseHeartbeatTasks 从 HEARTBEAT.md 的 tasks 段解析任务提示。
func ParseHeartbeatTasks(text string) []HeartbeatTask {
	parser := heartbeatTaskParser{
		lines:   strings.Split(text, "\n"),
		current: make(map[string]string),
	}
	return parser.parse()
}

type heartbeatTaskParser struct {
	lines       []string
	index       int
	tasks       []HeartbeatTask
	current     map[string]string
	inTasks     bool
	tasksIndent int
	block       *heartbeatBlock
}

type heartbeatBlock struct {
	key           string
	markerIndent  int
	contentIndent int
	lines         []string
}

type heartbeatLine struct {
	raw      string
	stripped string
	indent   int
}

func (p *heartbeatTaskParser) parse() []HeartbeatTask {
	for p.index < len(p.lines) {
		line := newHeartbeatLine(p.lines[p.index])
		if p.consumeLine(line) {
			p.index++
			continue
		}
		break
	}
	p.finishBlock()
	p.flushTask()
	return p.tasks
}

func (p *heartbeatTaskParser) consumeLine(line heartbeatLine) bool {
	if !p.inTasks {
		p.findTasksSection(line)
		return true
	}
	if p.block != nil {
		return p.consumeBlockLine(line)
	}
	if p.sectionEnded(line) {
		return false
	}
	p.consumeTaskLine(line)
	return true
}

func (p *heartbeatTaskParser) findTasksSection(line heartbeatLine) {
	if line.stripped != "tasks:" {
		return
	}
	p.inTasks = true
	p.tasksIndent = line.indent
}

func (p *heartbeatTaskParser) sectionEnded(line heartbeatLine) bool {
	return line.stripped != "" &&
		line.indent <= p.tasksIndent &&
		!strings.HasPrefix(line.stripped, "-")
}

func (p *heartbeatTaskParser) consumeTaskLine(line heartbeatLine) {
	if line.stripped == "" {
		return
	}
	field := line.stripped
	if strings.HasPrefix(field, "-") {
		p.flushTask()
		field = strings.TrimSpace(strings.TrimPrefix(field, "-"))
	}
	p.consumeField(field, line.indent)
}

func (p *heartbeatTaskParser) consumeField(field string, indent int) {
	if field == "" {
		return
	}
	key, value := parseHeartbeatKeyValue(field)
	if key == "" {
		return
	}
	if value == "|" {
		p.block = &heartbeatBlock{key: key, markerIndent: indent}
		return
	}
	p.current[key] = value
}

func (p *heartbeatTaskParser) consumeBlockLine(line heartbeatLine) bool {
	if line.stripped == "" {
		p.block.lines = append(p.block.lines, "")
		return true
	}
	if line.indent <= p.block.markerIndent {
		p.finishBlock()
		return p.consumeLine(line)
	}
	if p.block.contentIndent == 0 {
		p.block.contentIndent = line.indent
	}
	if line.indent < p.block.contentIndent {
		p.finishBlock()
		return p.consumeLine(line)
	}
	p.block.lines = append(p.block.lines, strings.TrimRight(line.raw[p.block.contentIndent:], " \t\r"))
	return true
}

func (p *heartbeatTaskParser) finishBlock() {
	if p.block == nil {
		return
	}
	p.current[p.block.key] = strings.TrimRight(strings.Join(p.block.lines, "\n"), " \t\r\n")
	p.block = nil
}

func (p *heartbeatTaskParser) flushTask() {
	if len(p.current) == 0 {
		return
	}
	if task := buildHeartbeatTask(p.current); task != nil {
		p.tasks = append(p.tasks, *task)
	}
	p.current = make(map[string]string)
}

func newHeartbeatLine(raw string) heartbeatLine {
	raw = strings.TrimRight(raw, "\r")
	return heartbeatLine{
		raw:      raw,
		stripped: strings.TrimSpace(raw),
		indent:   len(raw) - len(strings.TrimLeft(raw, " ")),
	}
}

func parseHeartbeatKeyValue(line string) (string, string) {
	key, value, found := strings.Cut(line, ":")
	if !found || strings.TrimSpace(key) == "" {
		return "", ""
	}
	return strings.TrimSpace(key), cleanHeartbeatValue(strings.TrimSpace(value))
}

func cleanHeartbeatValue(value string) string {
	if len(value) >= 2 {
		first := value[0]
		last := value[len(value)-1]
		if (first == '"' || first == '\'') && first == last {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func buildHeartbeatTask(fields map[string]string) *HeartbeatTask {
	name := strings.TrimSpace(fields["name"])
	interval := strings.TrimSpace(fields["interval"])
	prompt := strings.TrimSpace(fields["prompt"])
	if name == "" && interval == "" && prompt == "" {
		return nil
	}
	return &HeartbeatTask{
		Name:     name,
		Interval: interval,
		Prompt:   prompt,
	}
}
