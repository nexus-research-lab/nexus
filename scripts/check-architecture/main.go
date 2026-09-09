// 架构门禁只检查生产导入，集成测试仍可通过 app 装配真实服务。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const internalPrefix = "github.com/nexus-research-lab/nexus/internal/"

type packageInfo struct {
	ImportPath string
	Imports    []string
}

func forbidden(from, to string) bool {
	within := func(path, root string) bool { return path == root || strings.HasPrefix(path, root+"/") }
	if from == "protocol" || from == "relay" {
		return true
	}
	if from == "runtime" {
		return to != "protocol"
	}
	if from == "app" && within(to, "app/server") {
		return true
	}
	if from == "service/orchestration" && within(to, "mcp") {
		return true
	}
	if within(from, "service") && (within(to, "app") || within(to, "handler")) {
		return true
	}
	if within(from, "storage") || within(from, "infra") || within(from, "message") {
		return within(to, "app") || within(to, "handler") || within(to, "service") || (within(from, "message") && within(to, "storage"))
	}
	return false
}

func check(input io.Reader) error {
	decoder := json.NewDecoder(input)
	var violations []string
	for {
		var item packageInfo
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		from := strings.TrimPrefix(item.ImportPath, internalPrefix)
		for _, dependency := range item.Imports {
			if !strings.HasPrefix(dependency, internalPrefix) {
				continue
			}
			to := strings.TrimPrefix(dependency, internalPrefix)
			if forbidden(from, to) {
				violations = append(violations, from+" -> "+to)
			}
		}
	}
	if len(violations) > 0 {
		return fmt.Errorf("禁止的生产依赖：\n%s", strings.Join(violations, "\n"))
	}
	return nil
}

func main() {
	cmd := exec.Command("go", "list", "-json", "./internal/...")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err == nil {
		err = check(strings.NewReader(string(output)))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("架构依赖检查通过")
}
