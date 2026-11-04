// INPUT: Changed repository paths on stdin and the current Go package import graph.
// OUTPUT: Sorted package paths for vet/test, including affected production and test consumers.
// POS: Scope selection for check-go-changed.sh; loads metadata without running tests.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type goPackage struct {
	Dir          string
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func main() {
	changed, err := io.ReadAll(os.Stdin)
	if err != nil {
		fail(err)
	}
	command := exec.Command("go", "list", "-json", "./...")
	command.Stderr = os.Stderr
	output, err := command.Output()
	if err != nil {
		fail(err)
	}
	var packages []goPackage
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for {
		var pkg goPackage
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(err)
		}
		packages = append(packages, pkg)
	}
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	for _, name := range selectPackages(root, strings.Split(string(changed), "\n"), packages) {
		fmt.Println(name)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

func selectPackages(root string, changed []string, packages []goPackage) []string {
	byDir := map[string]string{}
	reverse := map[string][]string{}
	for _, pkg := range packages {
		rel, _ := filepath.Rel(root, pkg.Dir)
		byDir[filepath.ToSlash(rel)] = pkg.ImportPath
		for _, dep := range append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...) {
			reverse[dep] = append(reverse[dep], pkg.ImportPath)
		}
	}
	selected := map[string]bool{}
	var queue []string
	add := func(dir string, consumers bool) {
		if name := byDir[dir]; name != "" {
			selected[name] = true
			if consumers {
				queue = append(queue, name)
			}
		}
	}
	for _, path := range changed {
		switch {
		case path == "go.mod" || path == "go.sum" || path == "go.work" || path == "go.work.sum":
			return []string{"./..."}
		case strings.HasPrefix(path, "db/migrations/"):
			for dir := range byDir {
				if dir == "internal/storage" || strings.HasPrefix(dir, "internal/storage/") {
					add(dir, false)
				}
			}
			for _, dir := range []string{"internal/migration", "internal/app/server", "cmd/nexus-server"} {
				add(dir, false)
			}
		case strings.HasPrefix(path, "skills/"):
			for _, dir := range []string{"internal/service/skills", "internal/service/workspace", "internal/handler/workspace"} {
				add(dir, true)
			}
		case path == "scripts/check-go-changed.sh":
			add("scripts/check-go-scope", false)
		default:
			if path == "" {
				continue
			}
			dir := filepath.ToSlash(filepath.Dir(path))
			for dir != "." && byDir[dir] == "" {
				dir = filepath.ToSlash(filepath.Dir(dir))
			}
			if byDir[dir] != "" {
				add(dir, !strings.HasSuffix(path, "_test.go") && !strings.Contains(path, "/testdata/"))
			} else if strings.HasSuffix(path, ".go") && (strings.HasPrefix(path, "internal/") || strings.HasPrefix(path, "cmd/")) {
				// A removed or platform-only package cannot be resolved in this host graph.
				return []string{"./..."}
			}
		}
	}
	visited := map[string]bool{}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		visited[name] = true
		for _, consumer := range reverse[name] {
			selected[consumer] = true
			queue = append(queue, consumer)
		}
	}
	result := make([]string, 0, len(selected))
	for name := range selected {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
