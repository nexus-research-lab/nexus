package automation

import "strings"

type heartbeatTask struct {
	Name     string
	Interval string
	Prompt   string
}

func parseHeartbeatTasks(text string) []heartbeatTask {
	lines := strings.Split(text, "\n")
	tasks := make([]heartbeatTask, 0)
	current := map[string]string{}
	inTasks := false
	tasksIndent := 0
	pendingBlockKey := ""
	pendingBlockIndent := 0
	blockLines := make([]string, 0)
	blockContentIndent := 0

	flushCurrent := func() {
		if len(current) == 0 {
			return
		}
		task := buildHeartbeatTask(current)
		if task != nil {
			tasks = append(tasks, *task)
		}
		current = map[string]string{}
	}
	finishBlock := func() {
		if pendingBlockKey == "" {
			return
		}
		current[pendingBlockKey] = strings.TrimRight(strings.Join(blockLines, "\n"), " \t\r\n")
		pendingBlockKey = ""
		pendingBlockIndent = 0
		blockLines = make([]string, 0)
		blockContentIndent = 0
	}

	for i := 0; i < len(lines); {
		line := strings.TrimRight(lines[i], "\r")
		stripped := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if !inTasks {
			if stripped == "tasks:" {
				inTasks = true
				tasksIndent = indent
			}
			i++
			continue
		}

		if pendingBlockKey != "" {
			if stripped == "" {
				blockLines = append(blockLines, "")
				i++
				continue
			}
			if indent <= pendingBlockIndent {
				finishBlock()
				continue
			}
			if blockContentIndent == 0 {
				blockContentIndent = indent
			}
			if indent < blockContentIndent {
				finishBlock()
				continue
			}
			if blockContentIndent <= len(line) {
				blockLines = append(blockLines, strings.TrimRight(line[blockContentIndent:], " \t\r"))
			} else {
				blockLines = append(blockLines, "")
			}
			i++
			continue
		}

		// 中文注释：tasks 块回到更外层且不是列表项时，视为任务段结束。
		if stripped != "" && indent <= tasksIndent && !strings.HasPrefix(stripped, "-") {
			break
		}
		if stripped == "" {
			i++
			continue
		}

		if strings.HasPrefix(stripped, "-") {
			flushCurrent()
			item := strings.TrimSpace(strings.TrimPrefix(stripped, "-"))
			if item != "" {
				key, value := parseHeartbeatKeyValue(item)
				if key != "" {
					if value == "|" {
						pendingBlockKey = key
						pendingBlockIndent = indent
					} else {
						current[key] = value
					}
				}
			}
			i++
			continue
		}

		key, value := parseHeartbeatKeyValue(stripped)
		if key != "" {
			if value == "|" {
				pendingBlockKey = key
				pendingBlockIndent = indent
			} else {
				current[key] = value
			}
		}
		i++
	}

	if pendingBlockKey != "" {
		finishBlock()
	}
	flushCurrent()
	return tasks
}

func parseHeartbeatKeyValue(line string) (string, string) {
	index := strings.Index(line, ":")
	if index <= 0 {
		return "", ""
	}
	key := strings.TrimSpace(line[:index])
	value := strings.TrimSpace(line[index+1:])
	return key, cleanHeartbeatValue(value)
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

func buildHeartbeatTask(fields map[string]string) *heartbeatTask {
	name := strings.TrimSpace(fields["name"])
	interval := strings.TrimSpace(fields["interval"])
	prompt := strings.TrimSpace(fields["prompt"])
	if name == "" && interval == "" && prompt == "" {
		return nil
	}
	return &heartbeatTask{
		Name:     name,
		Interval: interval,
		Prompt:   prompt,
	}
}
